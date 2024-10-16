package main

import (
	"crypto/sha1"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"sync"

	"github.com/codecrafters-io/bittorrent-starter-go/internal/utils"
)

type TorrentInfo struct {
	url         string
	length      int
	infoHash    string
	pieceLength int
	pieces      []string
	peers       []string
	peerId      string
}

type DownloadResult struct {
	index    int
	contents []byte
}

const (
	N_PEERS           = 3
	PEER_SIZE         = 6
	PEER_ADDRESS_SIZE = 4
	PEER_PORT_SIZE    = 2
	PEER_ID_LENGTH    = 20
	HANDSHAKE_SIZE    = 68

	BITFIELD_ID      = 5
	INTERESTED_ID    = 2
	UNCHOKE_ID       = 1
	REQUEST_ID       = 6
	PIECE_ID         = 7
	PIECE_BLOCK_SIZE = 16 * 1024
)

// Ensures gofmt doesn't remove the "os" encoding/json import (feel free to remove this!)
var (
	_       = json.Marshal
	letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
)

func generatePeerId() string {
	b := make([]rune, 0, PEER_ID_LENGTH)
	for range PEER_ID_LENGTH {
		b = append(b, letters[rand.IntN(len(letters))])
	}

	return string(b)
}

func parseTorrentFile(filePath string) (map[string]interface{}, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	res, _, err := utils.DecodeBencode((string(data)))
	if err != nil {
		return nil, err
	}

	return res.(map[string]interface{}), nil
}

func getTorrentInfo(torrentFilePath string) (*TorrentInfo, error) {
	fileContents, err := parseTorrentFile(torrentFilePath)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	info, ok := fileContents["info"].(map[string]interface{})
	if !ok {
		panic(errors.New("Error extracting info"))
	}

	encodedInfo, err := utils.EncodeBencode(reflect.ValueOf(info))
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	hasher := sha1.New()
	hasher.Write([]byte(encodedInfo))

	pieces, ok := info["pieces"].(string)
	if !ok {
		panic(errors.New("Error extrating pieces"))
	}

	torrentInfo := TorrentInfo{
		url:         fileContents["announce"].(string),
		length:      info["length"].(int),
		infoHash:    string(hasher.Sum(nil)),
		pieceLength: info["piece length"].(int),
		pieces:      make([]string, 0, len(pieces)/20),
		peerId:      generatePeerId(),
	}

	for i := 0; i < len(pieces); i += 20 {
		torrentInfo.pieces = append(torrentInfo.pieces, pieces[i:i+20])
	}

	return &torrentInfo, nil
}

func printTorrentInfo(torrentFilePath string) {
	torrentInfo, err := getTorrentInfo(torrentFilePath)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Tracker URL: %s\n", torrentInfo.url)
	fmt.Printf("Length: %d\n", torrentInfo.length)
	fmt.Printf("Info Hash: %x\n", torrentInfo.infoHash)
	fmt.Printf("Piece Length: %d\n", torrentInfo.pieceLength)
	fmt.Println("Piece Hashes:")
	for _, piece := range torrentInfo.pieces {
		fmt.Printf("%x\n", piece)
	}
}

func getTorrentPeers(torrentInfo *TorrentInfo) (int, []string) {
	req, err := http.NewRequest(http.MethodGet, torrentInfo.url, nil)
	if err != nil {
		panic(err)
	}

	q := req.URL.Query()
	q.Add("info_hash", torrentInfo.infoHash)
	q.Add("peer_id", torrentInfo.peerId)
	q.Add("port", "6881")
	q.Add("uploaded", "0")
	q.Add("downloaded", "0")
	q.Add("left", strconv.Itoa(torrentInfo.length))
	q.Add("compact", "1")

	req.URL.RawQuery = q.Encode()

	res, err := http.Get(req.URL.String())
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		panic(errors.New(fmt.Sprintf("error - invalid code received fetching peers %d\n", res.StatusCode)))
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		panic(err)
	}

	responseDict, _, err := utils.DecodeBencode(string(body))
	decoded, ok := responseDict.(map[string]any)
	if !ok {
		panic(errors.New("error parsing response"))
	}

	interval := decoded["interval"].(int)

	peers := make([]string, 0, N_PEERS)
	peerString := decoded["peers"].(string)
	for i := 0; i < len(peerString); i += PEER_SIZE {
		port := binary.BigEndian.Uint16([]byte(peerString[i+PEER_ADDRESS_SIZE : i+PEER_ADDRESS_SIZE+PEER_PORT_SIZE]))
		peer := fmt.Sprintf("%d.%d.%d.%d:%d", peerString[i], peerString[i+1], peerString[i+2], peerString[i+3], port)
		peers = append(peers, peer)
	}

	return interval, peers
}

func printPeers(torrentInfo *TorrentInfo) {
	_, peers := getTorrentPeers(torrentInfo)

	for _, peer := range peers {
		fmt.Println(peer)
	}
}

func (torrentInfo *TorrentInfo) openPeerConnection(peerAddr string) net.Conn {
	conn, err := net.Dial("tcp", peerAddr)
	if err != nil {
		panic(err)
	}

	return conn
}

func (torrentInfo *TorrentInfo) handshake(conn net.Conn) []byte {
	handshakePayload := []byte{byte(19)}
	handshakePayload = append(handshakePayload, []byte("BitTorrent protocol")...)
	handshakePayload = append(handshakePayload, make([]byte, 8)...) // 8 zeros
	handshakePayload = append(handshakePayload, []byte(torrentInfo.infoHash)...)
	handshakePayload = append(handshakePayload, []byte(torrentInfo.peerId)...)

	_, err := conn.Write(handshakePayload)
	if err != nil {
		conn.Close()
		panic(err)
	}

	buf := make([]byte, HANDSHAKE_SIZE)
	n := 0
	for n < HANDSHAKE_SIZE {
		read, err := conn.Read(buf)
		if err != nil {
			conn.Close()
			panic(err)
		}
		n += read
	}

	return buf[48:]
}

func receiveMessage(conn net.Conn, messageId int) []byte {
	// prefix contains the lenght of the payload
	var payloadSize uint32
	if err := binary.Read(conn, binary.BigEndian, &payloadSize); err != nil {
		panic(err)
	}

	var id byte
	if err := binary.Read(conn, binary.BigEndian, &id); err != nil {
		panic(err)
	}
	if id != uint8(messageId) {
		fmt.Println(id)
		panic("unexpected message id")
	}

	if payloadSize > 1 {
		payloadBuf := make([]byte, payloadSize-1) // -1 bc of id is already read
		if _, err := io.ReadAtLeast(conn, payloadBuf, len(payloadBuf)); err != nil {
			panic(err)
		}
		return payloadBuf
	}

	return nil
}

func sendMessage(conn net.Conn, messageId int, payload []byte) {
	message := make([]byte, 4+1+len(payload))
	binary.BigEndian.PutUint32(message[:4], uint32(len(payload)+1))
	message[4] = byte(messageId)
	copy(message[5:], payload)

	if _, err := conn.Write(message); err != nil {
		panic(err)
	}
}

func (torrentInfo *TorrentInfo) exchangeFirstMessages(conn net.Conn) {
	// first, bitfield message (4 bytes prefix + 1 byte messageId)
	receiveMessage(conn, BITFIELD_ID)

	// now send the interested message (empty payload and size 1 (the messageId))
	sendMessage(conn, INTERESTED_ID, nil)

	// receive the unchoke message back (empty payload)
	receiveMessage(conn, UNCHOKE_ID)
}

func (torrentInfo *TorrentInfo) downloadPiece(conn net.Conn, pieceIndex int) []byte {
	// last piece may not be full length
	pieceLength := torrentInfo.pieceLength
	if pieceIndex == len(torrentInfo.pieces)-1 {
		pieceLength = torrentInfo.length - ((len(torrentInfo.pieces) - 1) * torrentInfo.pieceLength)
	}

	// last block may not be full length
	nBlocks := pieceLength / PIECE_BLOCK_SIZE
	lastBlockSize := pieceLength % PIECE_BLOCK_SIZE
	if lastBlockSize != 0 {
		nBlocks++
	}

	contents := make([]byte, pieceLength)
	for i := range nBlocks {
		blockLength := PIECE_BLOCK_SIZE
		if i == nBlocks-1 && lastBlockSize > 0 {
			blockLength = lastBlockSize
		}

		request := make([]byte, 3*4)
		binary.BigEndian.PutUint32(request[:4], uint32(pieceIndex))          // index
		binary.BigEndian.PutUint32(request[4:8], uint32(i*PIECE_BLOCK_SIZE)) // begin
		binary.BigEndian.PutUint32(request[8:], uint32(blockLength))         // block size
		sendMessage(conn, REQUEST_ID, request)

		resp := receiveMessage(conn, PIECE_ID)
		copy(contents[i*PIECE_BLOCK_SIZE:], resp[8:]) // the first 8 bytes of the response contain index and begin values
	}

	hasher := sha1.New()
	hasher.Write(contents)
	if string(hasher.Sum(nil)) != torrentInfo.pieces[pieceIndex] {
		panic("downloaded hash does not match")
	}
	return contents
}

func (torrentInfo *TorrentInfo) runDownloadJob(peer string, pieces <-chan int, results chan<- *DownloadResult, wg *sync.WaitGroup) {
	conn := torrentInfo.openPeerConnection(peer)
	defer conn.Close()
	torrentInfo.handshake(conn)
	torrentInfo.exchangeFirstMessages(conn)

	for piece := range pieces {
		results <- &DownloadResult{piece, torrentInfo.downloadPiece(conn, piece)}
	}

	wg.Done()
}

func (torrentInfo *TorrentInfo) downloadTorrent() []byte {
	_, peers := getTorrentPeers(torrentInfo)

	pieces := make(chan int, len(torrentInfo.pieces))
	results := make(chan *DownloadResult, len(torrentInfo.pieces))
	var wg sync.WaitGroup

	for _, peer := range peers {
		go torrentInfo.runDownloadJob(peer, pieces, results, &wg)
	}

	wg.Add(len(peers))
	for piece := range torrentInfo.pieces {
		pieces <- piece
	}
	close(pieces)
	wg.Wait()

	contents := make([]byte, torrentInfo.length)
	for range torrentInfo.pieces {
		result := <-results
		copy(contents[result.index*torrentInfo.pieceLength:], result.contents)
	}

	return contents
}

func (torrentInfo *TorrentInfo) downloadTorrentSerial() []byte {
	_, peers := getTorrentPeers(torrentInfo)
	peer := peers[rand.IntN(len(peers))]
	conn := torrentInfo.openPeerConnection(peer)
	defer conn.Close()

	torrentInfo.handshake(conn)
	torrentInfo.exchangeFirstMessages(conn)

	contents := make([]byte, torrentInfo.length)
	for i := range torrentInfo.pieces {
		copy(contents[i*torrentInfo.pieceLength:], torrentInfo.downloadPiece(conn, i))
	}

	return contents
}

func writeToFile(content []byte, filePath string) {
	file, err := os.Create(filePath)
	if err != nil {
		panic(err)
	}

	defer file.Close()

	_, err = file.Write(content)
	if err != nil {
		panic(err)
	}
}

func main() {
	command := os.Args[1]

	if command == "decode" {
		bencodedValue := os.Args[2]

		decoded, _, err := utils.DecodeBencode(bencodedValue)
		if err != nil {
			fmt.Println(err)
			return
		}

		jsonOutput, _ := json.Marshal(decoded)
		fmt.Println(string(jsonOutput))
	} else if command == "info" {
		torrentFilePath := os.Args[2]
		printTorrentInfo(torrentFilePath)
	} else if command == "peers" {
		torrentInfo, err := getTorrentInfo(os.Args[2])
		if err != nil {
			panic(err)
		}
		printPeers(torrentInfo)
	} else if command == "handshake" {
		torrentInfo, err := getTorrentInfo(os.Args[2])
		if err != nil {
			panic(err)
		}

		conn := torrentInfo.openPeerConnection(os.Args[3])
		defer conn.Close()

		peerId := torrentInfo.handshake(conn)
		fmt.Printf("Peer ID: %x\n", peerId)
	} else if command == "download_piece" {
		torrentInfo, err := getTorrentInfo(os.Args[4])
		if err != nil {
			panic(err)
		}

		_, peers := getTorrentPeers(torrentInfo)
		peer := peers[rand.IntN(len(peers))]

		conn := torrentInfo.openPeerConnection(peer)
		defer conn.Close()

		torrentInfo.handshake(conn)

		pieceIndex, err := strconv.Atoi(os.Args[5])
		if err != nil {
			panic(err)
		}

		torrentInfo.exchangeFirstMessages(conn)
		pieceContents := torrentInfo.downloadPiece(conn, pieceIndex)
		writeToFile(pieceContents, os.Args[3])
	} else if command == "download" {
		torrentInfo, err := getTorrentInfo(os.Args[4])
		if err != nil {
			panic(err)
		}

		contents := torrentInfo.downloadTorrent()
		writeToFile(contents, os.Args[3])
	} else if command == "magnet_parse" {
		magnet, err := utils.ParseMagnetLink(os.Args[2])
		if err != nil {
			panic(err)
		}
		fmt.Printf("Tracker URL: %s\nInfo Hash: %s\n", magnet.TrackerUrl, magnet.InfoHash)
	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}

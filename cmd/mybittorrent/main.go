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

const (
	N_PEERS           = 3
	PEER_SIZE         = 6
	PEER_ADDRESS_SIZE = 4
	PEER_PORT_SIZE    = 2
	PEER_ID_LENGTH    = 20
	HANDSHAKE_SIZE    = 68
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

func handshake(peerAddr string, torrentInfo *TorrentInfo) []byte {
	conn, err := net.Dial("tcp", peerAddr)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	handshakePayload := []byte{byte(19)}
	handshakePayload = append(handshakePayload, []byte("BitTorrent protocol")...)
	handshakePayload = append(handshakePayload, make([]byte, 8)...) // 8 zeros
	handshakePayload = append(handshakePayload, []byte(torrentInfo.infoHash)...)
	handshakePayload = append(handshakePayload, []byte(torrentInfo.peerId)...)

	_, err = conn.Write(handshakePayload)
	if err != nil {
		panic(err)
	}

	buff := make([]byte, HANDSHAKE_SIZE)
	n := 0
	for n < HANDSHAKE_SIZE {
		read, err := conn.Read(buff)
		if err != nil {
			panic(err)
		}
		n += read
	}

	return buff[48:] // return peerId that starts on byte 48
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
		fmt.Printf("Peer ID: %x\n", handshake(os.Args[3], torrentInfo))
	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}

package main

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"os"
	"reflect"

	"github.com/codecrafters-io/bittorrent-starter-go/internal/utils"
	// bencode "github.com/jackpal/bencode-go" // Available if you need it!
)

// Ensures gofmt doesn't remove the "os" encoding/json import (feel free to remove this!)
var _ = json.Marshal

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

		fileContents, err := parseTorrentFile(torrentFilePath)
		if err != nil {
			fmt.Println(err)
			return
		}

		fmt.Printf("Tracker URL: %s\n", fileContents["announce"])

		info, ok := fileContents["info"].(map[string]interface{})

		if ok {
			fmt.Printf("Length: %d\n", info["length"])

			encodedInfo, err := utils.EncodeBencode(reflect.ValueOf(info))
			if err != nil {
				fmt.Println(err)
				return
			}

			hasher := sha1.New()
			hasher.Write([]byte(encodedInfo))
			fmt.Printf("Info Hash: %x\n", hasher.Sum(nil))
		} else {
			fmt.Println("Error extracting length")
		}
	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"unicode"
	// bencode "github.com/jackpal/bencode-go" // Available if you need it!
)

// Ensures gofmt doesn't remove the "os" encoding/json import (feel free to remove this!)
var _ = json.Marshal

// Example:
// - 5:hello -> hello
// - 10:hello12345 -> hello12345
func decodeBencode(bencodedString string) (interface{}, error) {
	if len(bencodedString) == 0 {
		return nil, errors.New("found empty input")
	}

	if unicode.IsDigit(rune(bencodedString[0])) {
		return decodeString(bencodedString)
	}

	if bencodedString[0] == 'i' {
		return decodeInt(bencodedString)
	}

	return nil, errors.ErrUnsupported
}

func decodeString(bencodedString string) (interface{}, error) {
	i := 0

	for i = 0; i < len(bencodedString); i++ {
		if !unicode.IsDigit(rune(bencodedString[i])) {
			break
		}
	}

	length, err := strconv.Atoi(bencodedString[:i])
	if err != nil {
		return nil, err
	}

	if i == len(bencodedString) || bencodedString[i] != ':' {
		return nil, errors.New("was expecting a ':', found something else")
	}

	if i+1+length > len(bencodedString) {
		return nil, errors.New("found end of input, was expecting more characters")
	}

	return bencodedString[i+1 : i+1+length], nil
}

func decodeInt(bencodedString string) (interface{}, error) {
	i := 1

	for ; i < len(bencodedString); i++ {
		if i == 1 && bencodedString[i] == '-' {
			continue
		}

		if !unicode.IsDigit(rune(bencodedString[i])) {
			break
		}
	}

	if i == len(bencodedString) || bencodedString[i] != 'e' {
		return nil, errors.New("was expecting 'e' at end of integer, found something else")
	}

	decoded, err := strconv.Atoi(bencodedString[1:i])
	if err != nil {
		return nil, err
	}

	return decoded, nil
}

func main() {
	command := os.Args[1]

	if command == "decode" {
		bencodedValue := os.Args[2]

		decoded, err := decodeBencode(bencodedValue)
		if err != nil {
			fmt.Println(err)
			return
		}

		jsonOutput, _ := json.Marshal(decoded)
		fmt.Println(string(jsonOutput))
	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}

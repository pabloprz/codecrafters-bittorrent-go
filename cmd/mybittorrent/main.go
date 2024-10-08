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
	i := 0

	for i = 0; i < len(bencodedString); i++ {
		if !unicode.IsDigit(rune(bencodedString[i])) {
			break
		}
	}

	if i == 0 {
		return nil, errors.New("was expecting an number, found something else")
	}

	length, err := strconv.Atoi(bencodedString[:i])
	if err != nil {
		return nil, err
	}

	if bencodedString[i] != ':' {
		return nil, errors.New("was expecting a ':', found something else")
	}

	if i+1+length > len(bencodedString) {
		return nil, errors.New("found end of input, was expecting more characters")
	}

	return bencodedString[i+1 : i+1+length], nil
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

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
		res, _, err := decodeString(bencodedString)
		return res, err
	}

	if bencodedString[0] == 'i' {
		res, _, err := decodeInt(bencodedString)
		return res, err
	}

	if bencodedString[0] == 'l' {
		res, _, err := decodeList(bencodedString)
		return res, err
	}

	return nil, errors.ErrUnsupported
}

func decodeList(bencoded string) (interface{}, int, error) {
	list := []interface{}{}
	i := 1
	for i < len(bencoded) {
		if unicode.IsDigit(rune(bencoded[i])) {
			str, end, err := decodeString(bencoded[i:])
			if err != nil {
				return nil, -1, err
			}
			i += end
			list = append(list, str)
			continue
		}

		if bencoded[i] == 'i' {
			num, end, err := decodeInt(bencoded[i:])
			if err != nil {
				return nil, -1, err
			}
			i += end
			list = append(list, num)
			continue
		}

		if bencoded[i] == 'l' {
			l, end, err := decodeList(bencoded[i:])
			if err != nil {
				return nil, -1, err
			}
			i += end
			list = append(list, l)
			continue
		}

		if bencoded[i] == 'e' {
			return list, i + 1, nil
		}

		return nil, -1, errors.ErrUnsupported
	}

	return list, i + 1, nil
}

func decodeString(bencodedString string) (interface{}, int, error) {
	i := 0

	for i = 0; i < len(bencodedString); i++ {
		if !unicode.IsDigit(rune(bencodedString[i])) {
			break
		}
	}

	length, err := strconv.Atoi(bencodedString[:i])
	if err != nil {
		return nil, -1, err
	}

	if i >= len(bencodedString) || bencodedString[i] != ':' {
		return nil, -1, errors.New("was expecting a ':', found something else")
	}

	upto := i + 1 + length

	if upto > len(bencodedString) {
		return nil, -1, errors.New("found end of input, was expecting more characters")
	}

	return bencodedString[i+1 : upto], upto, nil
}

func decodeInt(bencodedString string) (interface{}, int, error) {
	i := 1

	for ; i < len(bencodedString); i++ {
		if i == 1 && bencodedString[i] == '-' {
			continue
		}

		if !unicode.IsDigit(rune(bencodedString[i])) {
			break
		}
	}

	if i >= len(bencodedString) || bencodedString[i] != 'e' {
		return nil, -1, errors.New("was expecting 'e' at end of integer, found something else")
	}

	decoded, err := strconv.Atoi(bencodedString[1:i])
	if err != nil {
		return nil, -1, err
	}

	return decoded, i + 1, nil
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

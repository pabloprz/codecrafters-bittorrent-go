package utils

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

type stringValue struct {
	key   string
	value reflect.Value
}

func EncodeBencode(val reflect.Value) (string, error) {
	if !val.IsValid() {
		return "", errors.New("cannot write a null value")
	}

	switch kind := val.Kind(); kind {
	case reflect.String:
		return encodeString(val.String())
	// What about uints?
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return encodeInt(val.Int())
	case reflect.Array, reflect.Slice:
		return encodeList(val)
	case reflect.Map:
		return encodeDict(val)
	case reflect.Interface:
		return EncodeBencode(val.Elem())
	default:
		return "", errors.New("unsuported type")
	}
}

func encodeDict(input reflect.Value) (string, error) {
	if input.Type().Key().Kind() != reflect.String {
		return "", errors.New("unsuported map type")
	}

	var builder strings.Builder
	builder.WriteRune('d')

	// Sort the dict keys
	sortedKeys := make([]stringValue, 0, len(input.MapKeys()))
	for _, k := range input.MapKeys() {
		sortedKeys = append(sortedKeys, stringValue{key: k.String(), value: input.MapIndex(k)})
	}
	slices.SortFunc(sortedKeys, func(a, b stringValue) int {
		return strings.Compare(a.key, b.key)
	})

	for _, k := range sortedKeys {
		key, err := EncodeBencode(reflect.ValueOf(k.key))
		if err != nil {
			return "", err
		}
		builder.WriteString(key)

		val, err := EncodeBencode(k.value)
		if err != nil {
			return "", err
		}
		builder.WriteString(val)
	}

	builder.WriteRune('e')
	return builder.String(), nil
}

func encodeList(input reflect.Value) (string, error) {
	var builder strings.Builder
	builder.WriteRune('l')

	for i := 0; i < input.Len(); i++ {
		elem, err := EncodeBencode(input.Index(i))
		if err != nil {
			return "", err
		}
		builder.WriteString(elem)
	}

	builder.WriteRune('e')
	return builder.String(), nil
}

func encodeString(input string) (string, error) {
	return fmt.Sprintf("%d:%s", len(input), input), nil
}

func encodeInt(input int64) (string, error) {
	return fmt.Sprintf("i%de", input), nil
}

// Example:
// - 5:hello -> hello
// - 10:hello12345 -> hello12345
func DecodeBencode(bencodedString string) (interface{}, int, error) {
	if len(bencodedString) == 0 {
		return nil, -1, errors.New("found empty input")
	}

	if unicode.IsDigit(rune(bencodedString[0])) {
		return decodeString(bencodedString)
	}

	if bencodedString[0] == 'i' {
		return decodeInt(bencodedString)
	}

	if bencodedString[0] == 'l' {
		return decodeList(bencodedString)
	}

	if bencodedString[0] == 'd' {
		return decodeDict(bencodedString)
	}

	return nil, -1, errors.ErrUnsupported
}

func decodeDict(bencoded string) (map[string]interface{}, int, error) {
	dict := make(map[string]interface{})

	i := 1
	for i < len(bencoded) {
		if bencoded[i] == 'e' {
			return dict, i + 1, nil
		}

		if !unicode.IsDigit(rune(bencoded[i])) {
			return nil, -1, errors.New("dictionary keys must be strings")
		}

		key, end, err := decodeString(bencoded[i:])
		if err != nil {
			return nil, -1, err
		}
		i += end

		val, end, err := DecodeBencode(bencoded[i:])
		if err != nil {
			return nil, -1, err
		}
		i += end

		dict[key] = val
	}

	return dict, i + 1, nil
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

func decodeString(bencodedString string) (string, int, error) {
	i := 0

	for i = 0; i < len(bencodedString); i++ {
		if !unicode.IsDigit(rune(bencodedString[i])) {
			break
		}
	}

	length, err := strconv.Atoi(bencodedString[:i])
	if err != nil {
		return "", -1, err
	}

	if i >= len(bencodedString) || bencodedString[i] != ':' {
		return "", -1, errors.New("was expecting a ':', found something else")
	}

	upto := i + 1 + length

	if upto > len(bencodedString) {
		return "", -1, errors.New("found end of input, was expecting more characters")
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

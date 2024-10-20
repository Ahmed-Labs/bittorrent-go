package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"unicode"
)

func isBencodedString(s string) bool {
	return unicode.IsDigit(rune(s[0]))
}

func isBencodedInteger(s string) bool {
	return rune(s[0]) == 'i' && rune(s[len(s)-1]) == 'e'
}

func isBencodedList(s string) bool {
	return rune(s[0]) == 'l' && rune(s[len(s)-1]) == 'e'
}

func decodeBencodedString(bencodedString string, idx *int) (string, error) {
	var firstColonIndex int

	for i := 0; i < len(bencodedString); i++ {
		if bencodedString[i] == ':' {
			firstColonIndex = i
			break
		}
	}
	lengthStr := bencodedString[:firstColonIndex]

	length, err := strconv.Atoi(lengthStr)
	if err != nil {
		return "", err
	}
	*idx = firstColonIndex + length + 1
	return bencodedString[firstColonIndex+1 : firstColonIndex+1+length], nil
}

func decodeBencodedInt(bencodedInt string) (int, error) {
	encoded := bencodedInt[1 : len(bencodedInt)-1]

	// Check for leading 0's
	if len(encoded) > 1 && (encoded[0] == '0' ||
		encoded[0] == '-' && encoded[1] == '0') {
		return -1, fmt.Errorf("Invalid encoded bencode integer: trailing 0's")
	}

	decoded, err := strconv.Atoi(encoded)
	if err != nil {
		return -1, err
	}
	return decoded, nil
}

func decodeBencodedList(bencodedList string) ([]interface{}, error) {
	var res []interface{}

	i := 1
	for i < len(bencodedList) {
		if unicode.IsDigit(rune(bencodedList[i])) {
			decoded, err := decodeBencodedString(bencodedList[i:], &i)
			if err != nil {
				return res, err
			}
			res = append(res, decoded)
		} else {
			j := i
			for j < len(bencodedList) && bencodedList[j] != 'e' {
				j++
			}
			decoded, err := decodeBencodedInt(bencodedList[i : j+1])
			if err != nil {
				return res, err
			}
			res = append(res, decoded)
			i = j + 1
		}
		i++
	}
	return res, nil
}

func decodeBencode(bencoded string) (interface{}, error) {
	if isBencodedList(bencoded) {
		return decodeBencodedList(bencoded)
	} else if isBencodedInteger(bencoded) {
		return decodeBencodedInt(bencoded)
	} else if isBencodedString(bencoded) {
		return decodeBencodedString(bencoded, nil)
	} else {
		return "", fmt.Errorf("Unexpected bencoded string received. Unable to decode.")
	}
}

func main() {
	command := os.Args[1]

	if command == "decode" {
		// Uncomment this block to pass the first stage
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

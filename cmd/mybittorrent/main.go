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

func decodeBencodedString(bencodedString string) (string, error) {
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

	return bencodedString[firstColonIndex+1 : firstColonIndex+1+length], nil
}

func bencodeStringEnd(b string, idx int) int {
	var firstColonIndex int

	for i := idx; i < len(b); i++ {
		if b[i] == ':' {
			firstColonIndex = i
			break
		}
	}

	lengthStr := b[idx:firstColonIndex]
	length, err := strconv.Atoi(lengthStr)
	if err != nil {
		return -1
	}

	return idx + length + 1
}

func bencodeIntEnd(b string, idx int) int {
	i := idx

	for i < len(b) && b[i] != 'e' {
		i++
	}
	return i
}

func bencodeListEnd(b string, idx int) int {
	stack := []rune{'l'}
	i := idx+1

	for i < len(b) && len(stack) > 0 {
		if b[i] == 'l' {
			i = bencodeListEnd(b, i)
		} else if b[i] == 'i' {
			i = bencodeIntEnd(b, i)
		} else if unicode.IsDigit(rune(b[i])) {
			i = bencodeStringEnd(b, i)
		} else if b[i] == 'e' {
			stack = stack[:len(stack)-1]
		}
		i++
	}
	return i-1
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
	i := 0
	bencodedList = bencodedList[1:len(bencodedList)-1]
	res := []interface{}{}

	for i < len(bencodedList) {
		if bencodedList[i] == 'l' {
			endIdx := bencodeListEnd(bencodedList, i)
			d, err := decodeBencodedList(bencodedList[i:endIdx+1])
			if err != nil {
				return nil, err
			}
			res = append(res, d)
			i = endIdx + 1
		} else if unicode.IsDigit(rune(bencodedList[i])){
			endIdx := bencodeStringEnd(bencodedList, i)			
			d, err := decodeBencodedString(bencodedList[i:endIdx+1])
			if err != nil {
				return nil, err
			}
			res = append(res, d)
			i = endIdx + 1
			
		} else if rune(bencodedList[i]) == 'i' {
			endIdx := bencodeIntEnd(bencodedList, i)
			d, err := decodeBencodedInt(bencodedList[i:endIdx+1])
			if err != nil {
				return nil, err
			}
			res = append(res, d)
			i = endIdx + 1
		} else {
			i++
		}
	}
	return res, nil
}

func decodeBencode(bencoded string) (interface{}, error) {
	if isBencodedList(bencoded) {
		return decodeBencodedList(bencoded)
	} else if isBencodedInteger(bencoded) {
		return decodeBencodedInt(bencoded)
	} else if isBencodedString(bencoded) {
		return decodeBencodedString(bencoded)
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

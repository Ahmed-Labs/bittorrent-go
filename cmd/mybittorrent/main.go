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

func isBencodedDictionary(s string) bool {
	return rune(s[0]) == 'd'
}

func bencodedStringEnd(b string, idx int) int {
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
	return firstColonIndex + length
}

func bencodedIntEnd(b string, idx int) int {
	i := idx

	for i < len(b) && b[i] != 'e' {
		i++
	}
	return i
}

func bencodedListEnd(b string, idx int) int {
	stack := []rune{'l'}
	i := idx+1

	for i < len(b) && len(stack) > 0 {
		if b[i] == 'l' {
			i = bencodedListEnd(b, i)
		} else if b[i] == 'i' {
			i = bencodedIntEnd(b, i)
		} else if unicode.IsDigit(rune(b[i])) {
			i = bencodedStringEnd(b, i)
		} else if b[i] == 'e' {
			stack = stack[:len(stack)-1]
		}
		i++
	}
	return i-1
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
			endIdx := bencodedListEnd(bencodedList, i)
			d, err := decodeBencodedList(bencodedList[i:endIdx+1])
			if err != nil {
				return nil, err
			}
			res = append(res, d)
			i = endIdx + 1
		} else if unicode.IsDigit(rune(bencodedList[i])){
			endIdx := bencodedStringEnd(bencodedList, i)			
			d, err := decodeBencodedString(bencodedList[i:endIdx+1])
			if err != nil {
				return nil, err
			}
			res = append(res, d)
			i = endIdx + 1
			
		} else if rune(bencodedList[i]) == 'i' {
			endIdx := bencodedIntEnd(bencodedList, i)
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

func decodeBencodedDictionary(bencoded string) (map[string]interface{}, error) {
	res := map[string]interface{}{}
	i := 1
	var cursor int8 = 0
	var key string
	
	for i < len(bencoded) {
		if unicode.IsDigit(rune(bencoded[i])){
			endIdx := bencodedStringEnd(bencoded, i)
			d, err := decodeBencodedString(bencoded[i:endIdx+1])
			if err != nil {
				return nil, err
			}
			i = endIdx + 1
			if cursor == 0 {
				key = d
			} else {
				res[key] = d
			}

		} else if bencoded[i] == 'l' {
			endIdx := bencodedListEnd(bencoded, i)
			d, err := decodeBencodedList(bencoded[i:endIdx+1])
			if err != nil {
				return nil, err
			}
			i = endIdx + 1
			res[key] = d

		} else if rune(bencoded[i]) == 'i' {
			endIdx := bencodedIntEnd(bencoded, i)
			d, err := decodeBencodedInt(bencoded[i:endIdx+1])
			if err != nil {
				return nil, err
			}
			i = endIdx + 1
			res[key] = d
			
		} else if rune(bencoded[i]) == 'd' {
			endIdx := bencodedListEnd(bencoded, i)
			d, err := decodeBencodedDictionary(bencoded[i:endIdx+1])
			if err != nil {
				return nil, err
			}
			i = endIdx + 1
			res[key] = d

		} else {
			i++
		}
		cursor ^= 1
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
	} else if isBencodedDictionary(bencoded) {
		return decodeBencodedDictionary(bencoded)
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
			panic(err)
		}

		jsonOutput, _ := json.Marshal(decoded)
		fmt.Println(string(jsonOutput))
	} else if command == "info" {
		data, err := os.ReadFile(os.Args[2])
		if err != nil {
			panic(err)
		}
		decodedData, err := decodeBencodedDictionary(string(data))
		if err != nil {
			panic(err)
		}
		fmt.Println("Tracker URL:", decodedData["announce"])
 		fmt.Println("Length:", (decodedData["info"]).(map[string]interface{})["length"])
	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}

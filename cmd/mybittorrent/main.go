package main

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/jackpal/bencode-go"
	"github.com/sqids/sqids-go"
)

func checkError(err error) {
	if err != nil {
		panic(err)
	}
}

func generateID() string {
	s, _ := sqids.New(sqids.Options{
		MinLength: 20,
	})
	id, _ := s.Encode([]uint64{1, 2, 3})
	return id[:20]
}

func readTorrent(filename string) map[string]interface{} {
	// Read raw file data
	data, err := os.ReadFile(filename)
	checkError(err)

	// Deocde bencoded torrent metadata
	res, err := decodeBencodedDictionary(string(data))
	checkError(err)

	return res
}

func getTrackerURL(filename string) string {
	return readTorrent(filename)["announce"].(string)
}

func getTorrentMetadataInfo(filename string) map[string]interface{} {
	return readTorrent(filename)["info"].(map[string]interface{})
}

func getInfoHash(filename string) []byte {
	decodedData := readTorrent(filename)

	// Bencode info dict
	var sb strings.Builder
	err := bencode.Marshal(&sb, decodedData["info"])
	checkError(err)

	// Generate SHA-1 hash of info dict
	var encodedInfo string = sb.String()
	hasher := sha1.New()
	hasher.Write([]byte(encodedInfo))

	return hasher.Sum(nil)
}

func getTorrentMetadata(filename string) {
	trackerURL := getTrackerURL(filename)
	info := getTorrentMetadataInfo(filename)
	length := info["length"]

	fmt.Println("Tracker URL:", trackerURL)
	fmt.Println("Length:", length)

	hash := getInfoHash(filename)
	fmt.Printf("Info Hash: %x\n", hash)

	pieceLength := info["piece length"].(int)
	pieces := []byte((info["pieces"]).(string))

	fmt.Println("Piece Length:", pieceLength)
	fmt.Println("Pieces Hashes:")

	for currByte := 0; currByte < len(pieces); currByte += 20 {
		fmt.Printf("%x\n", pieces[currByte:currByte+20])
	}
}

func getTorrentPeers(filename string) []string {
	// Build query parameters used with tracker url
	params := url.Values{}
	length := strconv.Itoa(getTorrentMetadataInfo(filename)["length"].(int))

	params.Add("info_hash", string(getInfoHash(filename)))
	params.Add("peer_id", generateID())
	params.Add("port", "6881")
	params.Add("uploaded", "0")
	params.Add("downloaded", "0")
	params.Add("left", length)
	params.Add("compact", "1")

	// Send get request to tracker url with query parameters
	trackerURL := getTrackerURL(filename) + "?" + params.Encode()
	res, err := http.Get(trackerURL)
	checkError(err)
	defer res.Body.Close()

	// Read the response body
	body, err := io.ReadAll(res.Body)
	checkError(err)

	// Decode bencoded response body
	bencodedData := string(body)
	decodedData, err := decodeBencodedDictionary(bencodedData)
	checkError(err)

	// Each peer is 6 bytes, 4 bytes for ip, 2 bytes for port
	peers := []byte(decodedData["peers"].(string))
	separatedPeers := []string{}

	for i := 0; i < len(peers); i += 6 {
		parts := []string{}
		for j := 0; j < 4; j++ {
			parts = append(parts, strconv.Itoa(int(peers[i+j])))
		}
		port := strconv.Itoa(int(peers[i+4])<<8 + int(peers[i+5]))
		peer := strings.Join(parts, ".") + ":" + port

		separatedPeers = append(separatedPeers, peer)
		fmt.Println(peer)
	}
	return separatedPeers
}

func sendPeerHandshake(filename, peer string) []byte {
	// Open tcp connection with peer (ip:port)
	conn, err := net.Dial("tcp", peer)
	checkError(err)

	// Build peer handshake payload
	handshakePayload := []byte{}
	reservedBytes := [8]byte{0}
	peerID := generateID()

	handshakePayload = append(handshakePayload, 19)
	handshakePayload = append(handshakePayload, []byte("BitTorrent protocol")...)
	handshakePayload = append(handshakePayload, (reservedBytes[:])...)
	handshakePayload = append(handshakePayload, getInfoHash(filename)...)
	handshakePayload = append(handshakePayload, []byte(peerID)...)

	// Write handhsake payload to connection
	_, err = conn.Write(handshakePayload)
	checkError(err)

	// Read handshake response from peer
	buf := make([]byte, 68)
	_, err = conn.Read(buf)
	checkError(err)

	responsePeerID := buf[len(buf)-20:]
	fmt.Printf("Peer ID: %x\n", responsePeerID)

	return responsePeerID
}

func main() {
	command := os.Args[1]

	if command == "decode" {
		// Uncomment this block to pass the first stage
		bencodedValue := os.Args[2]

		decoded, err := decodeBencode(bencodedValue)
		checkError(err)

		jsonOutput, _ := json.Marshal(decoded)
		fmt.Println(string(jsonOutput))
	} else if command == "info" {
		getTorrentMetadata(os.Args[2])
	} else if command == "peers" {
		getTorrentPeers(os.Args[2])
	} else if command == "handshake" {
		sendPeerHandshake(os.Args[2], os.Args[3])
	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}

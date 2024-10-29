package main

import (
	"crypto/sha1"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/jackpal/bencode-go"
	"github.com/sqids/sqids-go"
)

type TorrentMetadata struct {
	trackerURL  string
	length      int
	infoHash    []byte
	pieceLength int
	pieceHashes [][]byte
}

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

func getTorrentMetadata(filename string) TorrentMetadata {
	trackerURL := getTrackerURL(filename)
	info := getTorrentMetadataInfo(filename)
	length := info["length"].(int)

	fmt.Println("Tracker URL:", trackerURL)
	fmt.Println("Length:", length)

	hash := getInfoHash(filename)
	fmt.Printf("Info Hash: %x\n", hash)

	pieceLength := info["piece length"].(int)
	pieces := []byte((info["pieces"]).(string))
	pieceHashes := [][]byte{}

	fmt.Println("Piece Length:", pieceLength)
	fmt.Println("Pieces Hashes:")

	for currByte := 0; currByte < len(pieces); currByte += 20 {
		fmt.Printf("%x\n", pieces[currByte:currByte+20])
		pieceHashes = append(pieceHashes, pieces[currByte:currByte+20])
	}

	return TorrentMetadata{
		trackerURL:  trackerURL,
		length:      length,
		infoHash:    hash,
		pieceLength: pieceLength,
		pieceHashes: pieceHashes,
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

func sendPeerHandshake(conn net.Conn, filename string) []byte {
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
	_, err := conn.Write(handshakePayload)
	checkError(err)

	// Read handshake response from peer
	buf := make([]byte, 68)
	_, err = conn.Read(buf)
	checkError(err)

	responsePeerID := buf[len(buf)-20:]
	fmt.Printf("Peer ID: %x\n", responsePeerID)

	return responsePeerID
}

func getPeerMessage(conn net.Conn, messageType byte) []byte {
	messageLengthBytes := make([]byte, 4)
	_, err := io.ReadFull(conn, messageLengthBytes)
	checkError(err)

	messageLength := binary.BigEndian.Uint32(messageLengthBytes)
	message := make([]byte, messageLength)

	_, err = io.ReadFull(conn, message)
	checkError(err)

	if message[0] == messageType {
		fmt.Println("Received good message")
	} else {
		fmt.Println("Received bad message")
	}
	return message[1:]
}

func sendPeerMessage(conn net.Conn, messageType byte, messageContent []byte) {
	messageLength := make([]byte, 4)
	binary.BigEndian.PutUint32(messageLength, uint32(len(messageContent)+1))

	message := append(messageLength, messageType)
	message = append(message, messageContent...)
	fmt.Println("Sending message: ", message)
	_, err := conn.Write(message)
	checkError(err)
}

func downloadTorrent(filename, destination string) bool {
	peers := getTorrentPeers(filename)
	fmt.Println("Available peers:", peers)

	conn, err := net.Dial("tcp", peers[0])
	checkError(err)
	defer conn.Close()

	sendPeerHandshake(conn, filename)

	getPeerMessage(conn, BITFIELD)
	fmt.Println("Received bitfield message")

	sendPeerMessage(conn, INTERESTED, []byte{})
	fmt.Println("Sent interested message")

	getPeerMessage(conn, UNCHOKE)
	fmt.Println("Received unchoke message")

	t := getTorrentMetadata(filename)
	fileData := []byte{}

	for currPiece := 0; currPiece < len(t.pieceHashes); currPiece++ {
		currBlock := 0
		piece := []byte{}
		currPieceLength := min(t.pieceLength, t.length-(currPiece)*t.pieceLength)

		for offset := 0; offset < currPieceLength; offset = currBlock * (1 << 14) {
			index := uint32(currPiece)
			begin := uint32(offset)
			length := uint32(min(1<<14, currPieceLength-offset))

			messageContent := []byte{}
			messageContent = binary.BigEndian.AppendUint32(messageContent, index)
			messageContent = binary.BigEndian.AppendUint32(messageContent, begin)
			messageContent = binary.BigEndian.AppendUint32(messageContent, length)
			sendPeerMessage(conn, REQUEST, messageContent)

			// block = 32-bit index, 32-bit begin, block data is the rest
			block := getPeerMessage(conn, PIECE)
			piece = append(piece, block[8:]...)
			currBlock++
		}
		// Generate SHA-1 hash of piece
		hasher := sha1.New()
		hasher.Write(piece)
		hash := hasher.Sum(nil)

		if slices.Compare(hash, t.pieceHashes[currPiece]) != 0 {
			return false
		}
		fileData = append(fileData, piece...)
	}

	file, err := os.Create(destination)
	checkError(err)
	defer file.Close()

	n, err := file.Write(fileData)
	checkError(err)

	return n == t.length
}

func downloadTorrentPiece(filename, destination string, pieceIdx int) bool {
	peers := getTorrentPeers(filename)
	fmt.Println("Available peers:", peers)

	conn, err := net.Dial("tcp", peers[0])
	checkError(err)
	defer conn.Close()

	sendPeerHandshake(conn, filename)

	getPeerMessage(conn, BITFIELD)
	fmt.Println("Received bitfield message")

	sendPeerMessage(conn, INTERESTED, []byte{})
	fmt.Println("Sent interested message")

	getPeerMessage(conn, UNCHOKE)
	fmt.Println("Received unchoke message")

	t := getTorrentMetadata(filename)
	fileData := []byte{}

	currPiece := pieceIdx

	currBlock := 0
	piece := []byte{}
	currPieceLength := min(t.pieceLength, t.length-(currPiece)*t.pieceLength)

	for offset := 0; offset < currPieceLength; offset = currBlock * (1 << 14) {
		index := uint32(currPiece)
		begin := uint32(offset)
		length := uint32(min(1<<14, currPieceLength-offset))

		messageContent := []byte{}
		messageContent = binary.BigEndian.AppendUint32(messageContent, index)
		messageContent = binary.BigEndian.AppendUint32(messageContent, begin)
		messageContent = binary.BigEndian.AppendUint32(messageContent, length)
		sendPeerMessage(conn, REQUEST, messageContent)

		// block = 32-bit index, 32-bit begin, block data is the rest
		block := getPeerMessage(conn, PIECE)
		piece = append(piece, block[8:]...)
		currBlock++
	}
	// Generate SHA-1 hash of piece
	hasher := sha1.New()
	hasher.Write(piece)
	hash := hasher.Sum(nil)

	if slices.Compare(hash, t.pieceHashes[currPiece]) != 0 {
		return false
	}
	fileData = append(fileData, piece...)


	file, err := os.Create(destination)
	checkError(err)
	defer file.Close()

	_, err = file.Write(fileData)
	checkError(err)

	return true
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
		// Open tcp connection with peer (ip:port)
		conn, err := net.Dial("tcp", os.Args[3])
		checkError(err)
		sendPeerHandshake(conn, os.Args[2])
	} else if command == "download_piece" {
		destination := os.Args[3]
		torrent := os.Args[4]
		pieceIdx, err := strconv.Atoi(os.Args[5])
		checkError(err)
		
		success := downloadTorrentPiece(torrent, destination, pieceIdx)
		fmt.Println("Downloaded: ", success)
	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}

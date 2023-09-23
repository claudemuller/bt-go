package bittorrent

import (
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"

	"github.com/claudemuller/bt-go/internal/pkg/bencode"
)

const (
	pieceHashLen = 20
	peerIDLen    = 20
	port         = 6881
	compact      = 1
	peerSize     = 6
	ipSize       = 4
)

type torrent struct {
	filename string
	info     info
	peers    []peer
}

type info struct {
	trackerURL  string
	length      int
	infoHash    [20]byte
	pieceLength int
	pieces      []string
}

func NewTorrent(fname string) (torrent, error) {
	data, err := os.ReadFile(fname)
	if err != nil {
		return torrent{}, fmt.Errorf("error opening torrent file: %v", err)
	}

	res, err := bencode.Decode(string(data))
	if err != nil {
		return torrent{}, fmt.Errorf("error decoding data: %v", err)
	}

	bytes, _ := json.Marshal(res)
	fmt.Printf("%s\n", string(bytes))

	tracker, ok := res.(map[string]interface{})
	if !ok {
		return torrent{}, errors.New("error asserting decoded res data")
	}

	tInfo, ok := tracker["info"].(map[string]interface{})
	if !ok {
		return torrent{}, errors.New("error asserting decoded tracker data")
	}

	infoEnc, err := bencode.Encode(tInfo)
	if err != nil {
		return torrent{}, fmt.Errorf("error bencoding info map: %v", err)
	}

	hash := sha1.New()
	hash.Write([]byte(infoEnc))
	bs := hash.Sum(nil)

	pieceHashesStr, ok := tInfo["pieces"].(string)
	if !ok {
		return torrent{}, errors.New("error asserting decoded pieces hashes")
	}
	pieceHashesBytes := []byte(pieceHashesStr)
	pieceHashes := make([]string, 0, len(pieceHashesStr)/pieceHashLen)

	for i := 0; i < len(pieceHashesStr); i += pieceHashLen {
		pieceHash := fmt.Sprintf("%x", pieceHashesBytes[i:i+pieceHashLen])
		pieceHashes = append(pieceHashes, pieceHash)
	}

	torrInfo := info{
		trackerURL:  tracker["announce"].(string),
		length:      tInfo["length"].(int),
		pieceLength: tInfo["piece length"].(int),
		pieces:      pieceHashes,
	}

	peers, err := getPeers(torrInfo)
	if err != nil {
		return torrent{}, fmt.Errorf("error fetching peers: %v\n", err)
	}

	torr := torrent{
		filename: fname,
		peers:    peers,
		info:     torrInfo,
	}

	copy(torr.info.infoHash[:], bs)

	return torr, nil
}

func getPeers(torrInfo info) ([]peer, error) {
	req, err := http.NewRequest(http.MethodGet, torrInfo.trackerURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	q := req.URL.Query()
	q.Add("info_hash", string(torrInfo.infoHash[:]))
	q.Add("peer_id", genPeerID(peerIDLen))
	q.Add("port", strconv.Itoa(port))
	q.Add("uploaded", "0")
	q.Add("downloaded", "0")
	q.Add("left", strconv.Itoa(torrInfo.length))
	q.Add("compact", strconv.Itoa(compact))
	req.URL.RawQuery = q.Encode()

	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error GET'ing %s: %v", req.URL.String(), err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error decoding body: %v", err)
	}

	decRes, err := bencode.Decode(string(body))
	if err != nil {
		return nil, fmt.Errorf("error be-decoding bencoded response: %v", err)
	}

	trackerResp, ok := decRes.(map[string]interface{})
	if !ok {
		return nil, errors.New("error asserting tracker response")
	}

	psBytes, ok := trackerResp["peers"].(string)
	if !ok {
		return nil, errors.New("error asserting peers in response i.e. there appear to be no peers")
	}

	peersBytes := []byte(psBytes)
	peerCount := len(peersBytes) / peerSize
	peers := make([]peer, 0, peerCount)

	for i := 0; i < peerCount; i++ {
		offset := i * peerSize

		ipBytes := peersBytes[offset : offset+ipSize]
		ip := net.IP(ipBytes).String()

		portInt := (int(peersBytes[offset+4]) << 8) | int(peersBytes[offset+5])
		port := strconv.Itoa(portInt)

		p := peer{
			ip:   ip,
			port: port,
		}

		peers = append(peers, p)
	}

	return peers, nil
}

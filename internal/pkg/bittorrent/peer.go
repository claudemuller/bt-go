package bittorrent

import (
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"strconv"
)

const (
	handshakeLen     = 68
	kb               = 1024
	blockSize    int = 16 * kb
)

type handshake struct {
	peerID     [20]byte
	protoStr   [19]byte
	infoHash   [20]byte
	extensions [8]byte
}

func (hs *handshake) serialise() []byte {
	buf := make([]byte, handshakeLen)

	buf[0] = byte(len(hs.protoStr))
	curr := 1
	curr += copy(buf[curr:], hs.protoStr[:])
	curr += copy(buf[curr:], make([]byte, 8))
	curr += copy(buf[curr:], hs.infoHash[:])
	curr += copy(buf[curr:], hs.peerID[:])

	return buf
}

type peer struct {
	ip        string
	port      string
	handshake handshake
	conn      *net.TCPConn
}

func (p *peer) String() string {
	return fmt.Sprintf("%s:%s", p.ip, p.port)
}

// connect connects to a peer and completes the handshake.
// The conn TCP connection needs to be closed in the caller.
func (p *peer) connect(infoHash [20]byte) error {
	ip, port, err := net.SplitHostPort(p.String())
	portInt, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("error converting port str to int: %v", err)
	}

	hostIP, err := net.ResolveIPAddr("ip", ip)
	if err != nil {
		return fmt.Errorf("error resolving IP address: %v", err)
	}

	hostAddr := net.TCPAddr{
		IP:   hostIP.IP,
		Port: portInt,
	}

	conn, err := net.DialTCP("tcp", nil, &hostAddr)
	if err != nil {
		return fmt.Errorf("error connecting to host: %v", err)
	}

	/*------------------------------------------------------------------------------
	 * Build and send handshake request
	 */

	peerID := "00112233445566778899"

	var pID [20]byte
	var pStr [19]byte

	copy(pID[:], []byte(peerID))
	copy(pStr[:], []byte("BitTorrent protocol"))

	reqHs := handshake{
		protoStr: pStr,
		infoHash: infoHash,
		peerID:   pID,
	}

	if _, err = conn.Write(reqHs.serialise()); err != nil {
		return fmt.Errorf("error writing to connection: %v", err)
	}

	/*------------------------------------------------------------------------------
	 * Read handshake response
	 */

	res := make([]byte, handshakeLen)
	if _, err := io.ReadFull(conn, res); err != nil {
		return fmt.Errorf("error reading handshake reply: %v", err)
	}

	if uint8(handshakeLen) != res[0] {
		return errors.New("invalid handshake lenght")
	}

	var resProtoStr [19]byte

	copy(resProtoStr[:], res[1:20])
	if resProtoStr != pStr {
		return errors.New("invalid protocol string")
	}

	var resInfoHash [20]byte

	copy(resInfoHash[:], res[28:48])
	if resInfoHash != infoHash {
		return errors.New("invalid return info hash")
	}

	var resPeerID [20]byte

	copy(resPeerID[:], res[48:])
	if resPeerID != pID {
		return errors.New("invalid return info hash")
	}

	resHs := handshake{
		protoStr: pStr,
		infoHash: infoHash,
		peerID:   pID,
	}

	p.conn = conn
	p.handshake = resHs

	return nil
}

func (p *peer) downloadPiece(torr torrent, pieceIdx int) ([]byte, error) {
	msg, err := readMsg(p.conn)
	if err != nil {
		return nil, fmt.Errorf("error reading message: %v", err)
	}

	if msg.ID != msgBitfield {
		return nil, fmt.Errorf("not bitfield msg: %d", msg.ID)
	}

	/*------------------------------------------------------------------------------
	 * Send `unchoke` message for piece
	 */

	unchokeMsg := message{ID: msgUnchoke}
	if _, err := p.conn.Write(unchokeMsg.serialise()); err != nil {
		return nil, fmt.Errorf("error sending '%s' message: %v", msgStrs[msgUnchoke], err)
	}

	/*------------------------------------------------------------------------------
	 * Send `interested` message for piece
	 */

	interestMsg := message{ID: msgInterested}
	fmt.Printf("sending '%s' msg\n", msgStrs[msgInterested])
	if _, err := p.conn.Write(interestMsg.serialise()); err != nil {
		return nil, fmt.Errorf("error sending '%s' message: %v", msgStrs[msgInterested], err)
	}

	/*------------------------------------------------------------------------------
	 * Wait for `unchoke` message
	 */

	msg, err = readMsg(p.conn)
	if err != nil {
		return nil, fmt.Errorf("error reading message: %v", err)
	}
	fmt.Printf("received msg '%s' [%d]\n", msgStrs[msg.ID], msg.ID)

	// fileBuf := make([]byte, torrentInfo.PieceLength)
	fmt.Printf("%#v\n", torr.info)

	/*------------------------------------------------------------------------------
	 * Download each block in piece
	 */

	if pieceIdx > len(torr.info.pieces) {
		return nil, fmt.Errorf("this torrent only has %d pieces but %d-th requested", len(torr.info.pieces), pieceIdx+1)
	}

	pieceLength := torr.info.pieceLength
	if pieceIdx == len(torr.info.pieces)-1 {
		pieceLength = torr.info.length - pieceIdx*torr.info.pieceLength
	}

	lastBlockSize := pieceLength % blockSize
	numBlocks := (pieceLength - lastBlockSize) / blockSize

	fmt.Printf("there are %d blocks in piece %d\n", numBlocks, pieceIdx)

	if lastBlockSize > 0 {
		fmt.Printf("piece %d has an unaligned block of size %d\n", pieceIdx, lastBlockSize)
		numBlocks++
	} else {
		fmt.Printf("piece %d has size of %d and is aligned with blocksize of %d\n", pieceIdx, torr.info.pieceLength, blockSize)
	}

	pieceBuf := make([]byte, pieceLength)

	for i := 0; i < numBlocks; i++ {
		length := blockSize
		if lastBlockSize > 0 && i == numBlocks-1 {
			fmt.Printf("reached last block, changing size to %d\n", lastBlockSize)
			length = lastBlockSize
		}

		/*------------------------------------------------------------------------------
		 * Send `request` message for block of piece
		 */

		payload := make([]byte, 12)
		binary.BigEndian.PutUint32(payload[0:4], uint32(pieceIdx))
		binary.BigEndian.PutUint32(payload[4:8], uint32(i*blockSize))
		binary.BigEndian.PutUint32(payload[8:12], uint32(length))

		requestMsg := message{
			ID:      msgRequest,
			Payload: payload,
		}
		fmt.Printf("sending '%s' [%d] msg for block %d\n", msgStrs[msgRequest], msgRequest, i)
		if _, err := p.conn.Write(requestMsg.serialise()); err != nil {
			return nil, fmt.Errorf("error sending '%s' message: %v", msgStrs[msgRequest], err)
		}

		msg, err = readMsg(p.conn)
		if err != nil {
			return nil, fmt.Errorf("error reading message: %v", err)
		}
		fmt.Printf("received msg '%s' [%d] for block %d\n", msgStrs[msg.ID], msg.ID, i)

		blockLen, err := parseBlock(pieceIdx, pieceBuf, msg)
		if err != nil {
			return nil, fmt.Errorf("error parsing block: %v", err)
		}
		fmt.Printf("block len: %d\n", blockLen)
	}

	fmt.Printf("piece len: %d\n", len(pieceBuf))

	/*------------------------------------------------------------------------------
	 * Validate the piece against its hash
	 */

	hash := sha1.New()
	hash.Write(pieceBuf)
	bs := hash.Sum(nil)

	if string(bs) == torr.info.pieces[pieceIdx] {
		return nil, errors.New("error validating piece")
	}

	payload := make([]byte, 4)
	binary.BigEndian.PutUint32(payload, uint32(pieceIdx))
	haveMsg := message{
		ID:      msgHave,
		Payload: payload,
	}

	/*------------------------------------------------------------------------------
	 * Send `have` message for piece
	 */

	fmt.Printf("sending '%s' msg for piece %d\n", msgStrs[msgHave], pieceIdx)
	if _, err := p.conn.Write(haveMsg.serialise()); err != nil {
		return nil, fmt.Errorf("error sending '%s' message: %v", msgStrs[msgHave], err)
	}

	return pieceBuf, nil
}

func genPeerID(n int) string {
	const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}

	return string(b)
}

func parseBlock(idx int, buf []byte, msg *message) (int, error) {
	if msg.ID != msgPiece {
		return 0, fmt.Errorf("expected piece '%s' [%d], got '%s' [%d]", msgStrs[msgPiece], msgPiece, msgStrs[msg.ID], msg.ID)
	}

	if len(msg.Payload) < 8 {
		return 0, fmt.Errorf("payload too short: %d < 8", len(msg.Payload))
	}
	fmt.Printf("payload len: %d\n", len(msg.Payload))

	parsedIdx := int(binary.BigEndian.Uint32(msg.Payload[0:4]))
	if parsedIdx != idx {
		return 0, fmt.Errorf("expected index %d, got %d", idx, parsedIdx)
	}
	fmt.Printf("parsed idx: %d\n", parsedIdx)

	offset := int(binary.BigEndian.Uint32(msg.Payload[4:8]))
	fmt.Printf("begin offset: %d\n", offset)
	if offset >= len(buf) {
		return 0, fmt.Errorf("begin offset too high: %d >= %d", offset, len(buf))
	}

	data := msg.Payload[8:]
	if offset+len(data) > len(buf) {
		return 0, fmt.Errorf("data too long [%d] for offset %d with length %d", len(data), offset, len(buf))
	}

	copy(buf[offset:], data)

	return len(data), nil
}

func readMsg(r io.Reader) (*message, error) {
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(r, lenBuf); err != nil {
		return nil, fmt.Errorf("error reading length: %v", err)
	}
	fmt.Printf("msg len: %d\n", lenBuf)

	length := binary.BigEndian.Uint32(lenBuf)
	// Keep-alive message
	if length == 0 {
		return nil, nil
	}

	msgBuf := make([]byte, length)
	if _, err := io.ReadFull(r, msgBuf); err != nil {
		return nil, fmt.Errorf("error reading message: %v", err)
	}

	m := message{
		ID:      uint8(msgBuf[0]),
		Payload: msgBuf[1:],
	}

	return &m, nil
}

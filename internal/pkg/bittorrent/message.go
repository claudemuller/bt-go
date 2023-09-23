package bittorrent

import "encoding/binary"

const (
	msgChoke         uint8 = 0
	msgUnchoke       uint8 = 1
	msgInterested    uint8 = 2
	msgNotInterested uint8 = 3
	msgHave          uint8 = 4
	msgBitfield      uint8 = 5
	msgRequest       uint8 = 6
	msgPiece         uint8 = 7
	msgCancel        uint8 = 8
)

var msgStrs = []string{
	"Choke",
	"Unchoke",
	"Interested",
	"NotInterested",
	"Have",
	"Bitfield",
	"Request",
	"Piece",
	"Cancel",
}

type message struct {
	ID      uint8
	Payload []byte
}

func (m *message) serialise() []byte {
	if m == nil {
		return make([]byte, 4)
	}

	length := uint32(len(m.Payload) + 1)
	buf := make([]byte, 4+length)
	binary.BigEndian.PutUint32(buf[0:4], length)
	buf[4] = byte(m.ID)

	copy(buf[5:], m.Payload)

	return buf
}

type Bitfield []byte

func (bf Bitfield) Has(idx int) bool {
	byteIdx := idx / 8
	offset := idx % 8
	return bf[byteIdx]>>(7-offset)&1 != 0
}

func (bf Bitfield) Set(idx int) {
	byteIdx := idx / 8
	offset := idx % 8
	bf[byteIdx] |= 1 << (7 - offset)
}

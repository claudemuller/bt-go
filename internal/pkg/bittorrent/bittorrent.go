package bittorrent

import (
	"fmt"
	"os"
)

func Download(fname, output string) error {
	torrent, err := NewTorrent(fname)
	if err != nil {
		return fmt.Errorf("error creating torrent: %v\n", err)
	}

	fileBuf := make([]byte, torrent.info.length)

	for i := 0; i < len(torrent.info.pieces); i++ {
		// TODO: Handle multiple clients
		err := torrent.peers[0].connect(torrent.info.infoHash)
		if err != nil {
			return fmt.Errorf("error shaking hands with peer: %v\n", err)
		}

		piece, err := torrent.peers[0].downloadPiece(torrent, i)
		if err != nil {
			return fmt.Errorf("error downloading piece %s: %v\n", os.Args[5], err)
		}
		torrent.peers[0].conn.Close()

		copy(fileBuf[i*torrent.info.pieceLength:], piece)
	}

	if err = os.WriteFile(output, fileBuf, os.ModePerm); err != nil {
		return fmt.Errorf("error writing to file: %v", err)
	}

	return nil
}

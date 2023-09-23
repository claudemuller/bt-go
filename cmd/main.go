package main

import (
	"flag"
	"log"
	"os"

	"github.com/claudemuller/bt-go/internal/pkg/bittorrent"
)

func main() {
	var filename, output string

	flag.StringVar(&filename, "filename", "", "the torrent filename")
	flag.StringVar(&output, "output", "", "the output filename")
	flag.Parse()

	if filename == "" || output == "" {
		flag.Usage()
		os.Exit(1)
	}

	if err := bittorrent.Download(filename, output); err != nil {
		log.Printf("error downloading %s: %v\n", filename, err)
		os.Exit(1)
	}

	log.Printf("Downloaded %s to %s.", filename, output)
}

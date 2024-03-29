package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/na--/winston/torrent/metadata"
)

func main() {
	var showHelp bool
	flag.BoolVar(&showHelp, "h", false, "Show help message")
	flag.Parse()

	if showHelp || flag.NArg() == 0 {
		//
		fmt.Printf("Usage: %v infohash1 [infohash2 ...]\n\n", os.Args[0])
		fmt.Println("Example infohash: 4d753474429d817b80ff9e0c441ca660ec5d2450")
		fmt.Println()
		fmt.Println("Options:")
		flag.PrintDefaults()
		os.Exit(1)
	}

	filesToDownload, finished := metadata.StartNewDownloadManager()

	for _, infoHash := range flag.Args() {
		filesToDownload <- infoHash
	}

	<-finished
}

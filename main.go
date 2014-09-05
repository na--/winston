package main

import (
	"flag"
	"os"

	"github.com/na--/winston/torrent/metadata"

	log "github.com/golang/glog"
)

func main() {
	var showHelp bool
	flag.BoolVar(&showHelp, "h", false, "Show help message")
	flag.Parse()

	if showHelp {
		// To see logs, use the -logtostderr flag and change the verbosity with
		// -v 0 (less verbose) up to -v 5 (more verbose).
		log.Errorf("Usage: %v\n\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
	}

	infoHashList := []string{
		//"4d753474429d817b80ff9e0c441ca660ec5d2450",
		//"7a1073bc39e6b0b01e3730227b8ffea6aeac5d59",
		"757b25d9681d493167b8d3759dbfddc983e80646"}

	filesToDownload, finished := metadata.StartNewDownloadManager()

	for _, infoHash := range infoHashList {
		filesToDownload <- infoHash
	}

	<-finished
}

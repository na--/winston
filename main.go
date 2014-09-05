package main

import (
	"flag"
	"os"
	"sync"
	"time"

	log "github.com/golang/glog"

	"github.com/nictuku/dht"
)

type downloadEventType int

const (
	eventSucessfulDownload downloadEventType = iota
	eventTimeout
)

type downloadEvent struct {
	infoHash  dht.InfoHash
	eventType downloadEventType
}

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

	infoHashList := [1]string{
		//"4d753474429d817b80ff9e0c441ca660ec5d2450",
		//"7a1073bc39e6b0b01e3730227b8ffea6aeac5d59",
		"757b25d9681d493167b8d3759dbfddc983e80646"}

	// Starts a DHT node with the default options. It picks a random UDP port. To change this, see dht.NewConfig.
	d, err := dht.New(nil)
	if err != nil {
		log.Errorf("WINSTON: New DHT error: %v\n", err)
		os.Exit(1)
	}

	go d.Run()
	time.Sleep(3 * time.Second) //TODO: remove after adding ticks again

	var remainingDownloadsWg sync.WaitGroup
	filesToDownload := make(chan dht.InfoHash)
	go downloadManager(d, filesToDownload, &remainingDownloadsWg)

	for _, infoHashString := range infoHashList {
		infoHash, err := dht.DecodeInfoHash(infoHashString)
		if err != nil {
			log.Errorf("WINSTON: DecodeInfoHash error: %v\n", err)
			os.Exit(1)
		}

		remainingDownloadsWg.Add(1)
		filesToDownload <- infoHash

	}

	remainingDownloadsWg.Wait()
}

func makePeerBuffer(in <-chan []string) chan string {
	out := make(chan string)

	go func() {
		defer close(out)
		var bufferedPeers []string

		// Get a group of peers and then try to pass them ony by one to "out" channgel.
		// If more are received meanwhile, add them to the slice :)
		for chunkOfPeers := range in {
			bufferedPeers = append(bufferedPeers, chunkOfPeers...)
		loop:
			for {
				select {
				case anotherChunkOfPeers, ok := <-in: // More peers are received before they can be processed by the receiver
					if !ok {
						// Channel was closed (successful download or timeout)
						return
					}
					// Buffer the newly received peers
					bufferedPeers = append(bufferedPeers, anotherChunkOfPeers...) //TODO: consider a maximum size for the buffer?

				case out <- bufferedPeers[0]: // Receiver consumed the first buffered peer

					bufferedPeers = bufferedPeers[1:] // TODO: check for possible memory leak?

					// If no more peers are in the buffer, go back to the beginning to fill up the tank
					if len(bufferedPeers) == 0 {
						break loop
					}
				}
			}
		}
	}()

	return out
}

func downloadManager(d *dht.DHT, filesToDownload chan dht.InfoHash, remainingDownloadsWg *sync.WaitGroup) {
	currentDownloads := make(map[dht.InfoHash]chan []string)
	downloadEvents := make(chan downloadEvent)

	for {
		select {
		case newFile := <-filesToDownload:
			if _, ok := currentDownloads[newFile]; ok {
				log.V(1).Infof("WINSTON: File %x is already downloading, skipping...\n", newFile)
				continue
			}
			log.V(1).Infof("WINSTON: Accepted %x for download...\n", newFile)

			// Create a channel for all the found peers
			currentDownloads[newFile] = make(chan []string)

			bufferedPeerChannel := makePeerBuffer(currentDownloads[newFile])

			// Ask that nice DHT fellow to find those peers :)
			d.PeersRequest(string(newFile), false)

			// Create a new gorouite that manages the download for the specific file
			go downloadFile(newFile, bufferedPeerChannel, downloadEvents)

		case newEvent := <-downloadEvents:
			if newEvent.eventType == eventSucessfulDownload {
				log.V(1).Infof("WINSTON: Download of %x completed :)\n", newEvent.infoHash)
			} else if newEvent.eventType == eventTimeout {
				log.V(1).Infof("WINSTON: Download of %x failed: time out :(\n", newEvent.infoHash)
			}
			close(currentDownloads[newEvent.infoHash])
			delete(currentDownloads, newEvent.infoHash)
			remainingDownloadsWg.Done()

		case newPeers, chanOk := <-d.PeersRequestResults:
			if !chanOk {
				// Something went wrong, mayday, mayday!
				log.Errorf("WINSTON: BORK!\n")
				os.Exit(1)
			}

			for ih, peers := range newPeers {
				// Check if download is still active
				if currentPeersChan, ok := currentDownloads[ih]; ok {
					log.V(1).Infof("WINSTON: Received %d new peers for file %x\n", len(peers), ih)
					currentPeersChan <- peers
				} else {
					log.V(1).Infof("WINSTON: Received %d peers for non-current file %x (probably completed or timed out)\n", len(peers), ih)
				}
			}
		}
	}
}

func downloadFile(infoHash dht.InfoHash, peerChannel chan string, eventsChannel chan downloadEvent) {
	//TODO:implement
	//TODO: get peers from buffered channel, connect to them, download torrent file
	//TODO: add some sure way to detect goroutine finished (defer send to channel?)
	count := 0
	tick := time.Tick(10 * time.Second)
	timeout := time.After(60 * time.Second)

	for {
		select {
		case newPeer, chanOk := <-peerChannel:
			if !chanOk {
				log.V(2).Infof("WINSTON: Peer channel for %x was closed, probably by torrent timeout. Killing download goroutine...\n", infoHash)
				return
			}
			count++
			log.V(2).Infof("WINSTON: Peer #%d received for torrent %x: %s\n", count, infoHash, dht.DecodePeerAddress(newPeer))
			peer.DownloadMetadataFromPeer(dht.DecodePeerAddress(newPeer), string(infoHash))

			time.Sleep(30 * time.Second) //TODO: remove
			eventsChannel <- downloadEvent{infoHash, eventTimeout}
			return

		case <-tick:
			log.V(2).Infof("WINSTON: Tick-tack %...\n", infoHash)

		case <-timeout:
			log.V(2).Infof("WINSTON: Torrent %x timed out...\n", infoHash)
			eventsChannel <- downloadEvent{infoHash, eventTimeout}
			return
		}
	}
}

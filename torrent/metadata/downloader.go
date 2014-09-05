// Package metadata is used for managing paralel downloads of torrent metadata from
// BitTorrent peers (found via the DHT network)

package metadata

import (
	"os"
	"time"

	"github.com/na--/winston/torrent/peer"

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

// StartNewDownloadManager starts a new goroutine and returns 2 channels:
// The first is used to pass torrent infohashes to the downloader;
// The second is used by the downloader to signal when all the requested torrents
// have been downloaded or have timed out.
func StartNewDownloadManager() (filesToDownload chan string, finished chan bool) {
	// Starts a DHT node with the default options, picks a random UDP port.
	d, err := dht.New(nil)
	if err != nil {
		log.Errorf("WINSTON: New DHT error: %v\n", err)
		os.Exit(1)
	}

	go d.Run()
	time.Sleep(5 * time.Second) //TODO: this is necessary; remove after investigating the DHT lib

	filesToDownload = make(chan string)
	finished = make(chan bool)

	go downloadManager(d, filesToDownload, finished)

	return filesToDownload, finished
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

func downloadManager(d *dht.DHT, filesToDownload chan string, finished chan bool) {
	currentDownloads := make(map[dht.InfoHash]chan []string)
	downloadEvents := make(chan downloadEvent)

	for {
		select {
		case newInfoHashString := <-filesToDownload:
			newFile, err := dht.DecodeInfoHash(newInfoHashString)
			if err != nil {
				log.Errorf("WINSTON: DecodeInfoHash error: %v\n", err)
				os.Exit(1)
			}

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
			if len(currentDownloads) == 0 {
				finished <- true
			}

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
	//TODO: implement
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

			time.Sleep(15 * time.Second)                           //TODO: remove after debugging
			eventsChannel <- downloadEvent{infoHash, eventTimeout} //TODO: remove after debugging
			return                                                 //TODO: remove after debugging

		case <-tick:
			log.V(2).Infof("WINSTON: Tick-tack %x...\n", infoHash)

		case <-timeout:
			log.V(2).Infof("WINSTON: Torrent %x timed out...\n", infoHash)
			eventsChannel <- downloadEvent{infoHash, eventTimeout}
			return
		}
	}
}

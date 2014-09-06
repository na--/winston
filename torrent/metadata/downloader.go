// Package metadata is used for managing paralel downloads of torrent metadata from
// BitTorrent peers (found via the DHT network)

package metadata

import (
	"os"
	"strings"
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
func StartNewDownloadManager() (chan<- string, <-chan bool) {
	// Starts a DHT node with the default options, picks a random UDP port.
	d, err := dht.New(nil)
	if err != nil {
		log.Errorf("WINSTON: New DHT error: %v\n", err)
		os.Exit(1)
	}

	go d.Run()

	time.Sleep(7 * time.Second) //TODO: this seems to be necessary; remove after investigating the DHT lib

	filesToDownload := make(chan string)
	finished := make(chan bool)

	go downloadManager(d, filesToDownload, finished)

	return filesToDownload, finished
}

func downloadManager(d *dht.DHT, filesToDownload <-chan string, finished chan<- bool) {
	currentDownloads := make(map[dht.InfoHash]chan []string)
	downloadEvents := make(chan downloadEvent)

	for {
		select {
		case newInfoHashString := <-filesToDownload:
			newFile, err := dht.DecodeInfoHash(newInfoHashString)
			if err != nil {
				//TODO: better error handling
				log.Errorf("WINSTON: DecodeInfoHash error: %v\n", err)
				os.Exit(1)
			}

			if _, ok := currentDownloads[newFile]; ok {
				log.V(3).Infof("WINSTON: File %x is already downloading, skipping...\n", newFile)
				continue
			}
			log.V(3).Infof("WINSTON: Accepted %x for download...\n", newFile)

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
				panic("WINSTON: BORK!\n")
			}

			for ih, peers := range newPeers {
				// Check if download is still active
				if currentPeersChan, ok := currentDownloads[ih]; ok {
					log.V(3).Infof("WINSTON: Received %d new peers for file %x\n", len(peers), ih)
					currentPeersChan <- peers
				} else {
					log.V(3).Infof("WINSTON: Received %d peers for non-current file %x (probably completed or timed out)\n", len(peers), ih)
				}
			}
		}
	}
}

func downloadFile(infoHash dht.InfoHash, peerChannel <-chan string, eventsChannel chan<- downloadEvent) {
	//TODO: implement
	//TODO: get peers from buffered channel, connect to them, download torrent file
	//TODO: add some sure way to detect goroutine finished (defer send to channel?)
	peerCount := 0
	tick := time.Tick(10 * time.Second)
	timeout := time.After(10 * time.Minute)

	for {
		select {
		case newPeer, chanOk := <-peerChannel:
			if !chanOk {
				log.V(2).Infof("WINSTON: Peer channel for %x was closed, probably by torrent timeout. Killing download goroutine...\n", infoHash)
				return
			}

			peerCount++
			peerStr := dht.DecodePeerAddress(newPeer)
			if strings.HasSuffix(peerStr, ":1") {
				log.V(3).Infof("WINSTON: Skipping peer #%d for torrent %x for looking fake: %s\n", peerCount, infoHash, peerStr)
				continue
			}

			log.V(3).Infof("WINSTON: Peer #%d received for torrent %x: %s\n", peerCount, infoHash, peerStr)

			//TODO: run as paralel goroutines
			//TODO: taka care to have N parallel downloaders at all times, if possible
			//TODO: send requests for more peers periodically
			//TODO: gather results and stop everything once a successful result has been found
			peer.DownloadMetadataFromPeer(peerStr, string(infoHash))

		case <-tick:
			log.V(3).Infof("WINSTON: Tick-tack %x...\n", infoHash)

		case <-timeout:
			log.V(3).Infof("WINSTON: Torrent %x timed out...\n", infoHash)
			eventsChannel <- downloadEvent{infoHash, eventTimeout}
			return
		}
	}
}

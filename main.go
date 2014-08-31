package main

import (
	"flag"
	"os"
	"sync"

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
	flag.Parse()

	// To see logs, use the -logtostderr flag and change the verbosity with
	// -v 0 (less verbose) up to -v 5 (more verbose).
	if len(flag.Args()) != 1 {
		log.Errorf("Usage: %v <infohash>\n\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Parses the infohash from the commandline
	infoHash, err := dht.DecodeInfoHash(flag.Args()[0])
	if err != nil {
		log.Errorf("WINSTON: DecodeInfoHash error: %v\n", err)
		os.Exit(1)
	}

	// Starts a DHT node with the default options. It picks a random UDP port. To change this, see dht.NewConfig.
	d, err := dht.New(nil)
	if err != nil {
		log.Errorf("WINSTON: New DHT error: %v\n", err)
		os.Exit(1)
	}

	//go d.Run()

	var remainingDownloadsWg sync.WaitGroup
	filesToDownload := make(chan dht.InfoHash)

	go downloadManager(d, filesToDownload, &remainingDownloadsWg)

	//TODO: run this for every unresolved infohash (with throtling of course)
	remainingDownloadsWg.Add(1)
	filesToDownload <- infoHash

	remainingDownloadsWg.Add(1)
	filesToDownload <- infoHash

	remainingDownloadsWg.Wait()
}

func makePeerBuffer(in <-chan []string) chan string {
	out := make(chan string)

	go func() {
		defer close(out)
		bufferedPeers := make([]string, 5)

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
			} else {
				log.V(1).Infof("WINSTON: Accepted %x for download...\n", newFile)

				// Create a channel for all the found peers
				currentDownloads[newFile] = make(chan []string)

				bufferedPeerChannel := makePeerBuffer(currentDownloads[newFile])

				// Ask that nice DHT fellow to find those peers :)
				d.PeersRequest(string(newFile), false)

				// Create a new gorouite that manages the download for the specific file
				go downloadFile(newFile, bufferedPeerChannel, downloadEvents)

			}
		case newEvent := <-downloadEvents:
			if newEvent.eventType == eventSucessfulDownload {
				log.V(1).Infof("WINSTON: Download of %x completed :)\n", newEvent.infoHash)
			} else if newEvent.eventType == eventTimeout {
				log.V(1).Infof("WINSTON: Download of %x failed: time out :(\n", newEvent.infoHash)
			}
			close(currentDownloads[newEvent.infoHash])
			delete(currentDownloads, newEvent.infoHash)

			//TODO: case receive peers
		}
	}
}

func downloadFile(infoHash dht.InfoHash, peerChannel chan string, eventsChannel chan downloadEvent) {
	//TODO:implement
	//TODO: get peers from buffered channel, connect to them, download torrent file
}

/*
func downloadTorrent(d *dht.DHT, infoHash dht.InfoHash) {
	defer wg.Done()

	// This channel will be used to signal when the file is downloaded
	torrentDownloaded := make(chan bool)
	tick := time.Tick(15 * time.Second)
	timeout := time.After(120 * time.Second)

	// Launch the gorouitine that will contact peers and try to download the file
	go connectToPeers(d, infoHash, torrentDownloaded)



	for {
		select {
		case <-torrentDownloaded:
			log.V(1).Infof("WINSTON: Torrent '%x' was successfully downloaded!!!\n", infoHash)
			return

		case <-timeout:
			log.V(1).Infof("WINSTON: Could not download torrent '%x': timed out\n", infoHash)
			os.Exit(0)
			return

		case <-tick:
			// Repeat the request until a result appears, querying nodes that haven't been
			// consulted before and finding close-by candidates for the infohash.
			//log.V(1).Info("WINSTON: tick\n")
			d.PeersRequest(string(infoHash), false)
		}
	}
}

func connectToPeers(n *dht.DHT, infoHash dht.InfoHash, torrentDownloaded chan bool) {
	for r := range n.PeersRequestResults {
		for foundHash, foundPeers := range r {
			log.V(1).Infof("WINSTON: FOUND PEERS FOR '%x': %#v\n", foundHash, foundPeers)
		}
	}
}
*/

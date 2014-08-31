package main

import (
	"flag"
	"os"
	"sync"
	"time"

	log "github.com/golang/glog"

	"github.com/nictuku/dht"
)

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

	go d.Run()

	var wg sync.WaitGroup

	//TODO: run this for every unresolved infohash (with throtling of course)
	wg.Add(1)
	go downloadTorrent(d, &wg, infoHash)
	wg.Wait()
}

func downloadTorrent(d *dht.DHT, wg *sync.WaitGroup, infoHash dht.InfoHash) {
	defer wg.Done()

	// This channel will be used to signal when the file is downloaded
	torrentDownloaded := make(chan bool)

	// Launch the gorouitine that will contact peers and try to download the file
	go connectToPeers(d, infoHash, torrentDownloaded)

	tick := time.Tick(10 * time.Second)

	for {
		select {
		case <-torrentDownloaded:
			log.V(1).Infof("WINSTON: Torrent '%x' was successfully downloaded!!!\n", infoHash)
			return

		case <-time.After(30 * time.Second):
			log.V(1).Infof("WINSTON: Could not download torrent '%x': timed out\n", infoHash)
			os.Exit(0)
			return

		case <-tick:
			// Repeat the request until a result appears, querying nodes that haven't been
			// consulted before and finding close-by candidates for the infohash.
			d.PeersRequest(string(infoHash), false)
		}
	}
}

// drainresults loops, printing the address of nodes it has found.
func connectToPeers(n *dht.DHT, infoHash dht.InfoHash, torrentDownloaded chan bool) {
	//count := 0
	for r := range n.PeersRequestResults {
		for foundHash, foundPeers := range r {
			log.V(1).Infof("WINSTON: FOUND PEERS FOR '%x': %#v\n", foundHash, foundPeers)
			torrentDownloaded <- true
			/*
				for _, x := range peers {
					fmt.Printf("########## PEER %d ##########: %v\n", count, dht.DecodePeerAddress(x))
					count++
					if count >= 5 {
						os.Exit(0)
					}
				}
			*/
		}
	}
}

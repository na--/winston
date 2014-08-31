package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/nictuku/dht"
)

func main() {
	flag.Parse()

	// To see logs, use the -logtostderr flag and change the verbosity with
	// -v 0 (less verbose) up to -v 5 (more verbose).
	if len(flag.Args()) != 1 {
		fmt.Fprintf(os.Stderr, "Usage: %v <infohash>\n\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Parses the infohash from the commandline
	infohash, err := dht.DecodeInfoHash(flag.Args()[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "DecodeInfoHash error: %v\n", err)
		os.Exit(1)
	}

	// Starts a DHT node with the default options. It picks a random UDP port. To change this, see dht.NewConfig.
	d, err := dht.New(nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "New DHT error: %v", err)
		os.Exit(1)
	}

	go d.Run()
	go drainresults(d)
	for {
		d.PeersRequest(string(infohash), false)
		time.Sleep(15 * time.Second)
	}
}

// drainresults loops, printing the address of nodes it has found.
func drainresults(n *dht.DHT) {
	count := 0
	for r := range n.PeersRequestResults {
		for _, peers := range r {
			for _, x := range peers {
				fmt.Printf("########## PEER %d ##########: %v\n", count, dht.DecodePeerAddress(x))
				count++
				if count >= 5 {
					os.Exit(0)
				}
			}
		}
	}
}

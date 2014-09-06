package metadata

import (
	"flag"
	"fmt"
	"os"
)

var outputFolder = flag.String("output_folder", "./tmp/", "Folder where you want to save the downloaded torrent files.")

// This function accepts found peers in bulk through the in channel, buffers them
// and passes them one by one to the out channel
func makePeerBuffer(in <-chan []string) <-chan string {
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

func saveMetaInfo(infoHash string, metadata []byte) (err error) {

	err = os.MkdirAll(*outputFolder, os.ModeDir|os.ModePerm)
	if err != nil {
		err = fmt.Errorf("Could not create folder '%s': %s", *outputFolder, err)
		return
	}

	f, err := os.Create(fmt.Sprintf("%s/%x.torrent", *outputFolder, infoHash))
	if err != nil {
		err = fmt.Errorf("Error when opening file for creation: %s", err)
		return
	}
	defer f.Close()

	//TODO: more metadata; better meta-metainfo saving; better error handling
	f.WriteString("d4:info")
	_, err = f.Write(metadata)
	if err != nil {
		err = fmt.Errorf("Error when saving torrent file: %s", err)
		return
	}
	f.WriteString("e")

	return
}

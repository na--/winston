package metadata

// This function accepts found peers in bulk through the in channel, buffers them
// and passes them one by one to the out channel
func makePeerBuffer(in <-chan []string) (out chan string) {
	out = make(chan string)

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

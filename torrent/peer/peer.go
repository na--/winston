// Package peer is used for connecting to BitTorrent peers and downloading torrent metadata from them
//
// Most of the code in this file is copied from or heavily influenced by Taipei-Torrent: https://github.com/jackpal/Taipei-Torrent
// Taipei-Torrent is copyrighted (c) Jack Palevich and others: https://github.com/jackpal/Taipei-Torrent/graphs/contributors

package peer

import (
	"crypto/sha1"
	"fmt"
	"io"
	"net"
	"time"

	log "github.com/golang/glog"
)

func createPeerReader(conn net.Conn) (<-chan []byte, <-chan error) {
	msgChan := make(chan []byte)
	errChan := make(chan error)

	go func() {
		defer log.V(3).Infof("WINSTON: Peer reader goroutine exited\n")

		defer close(msgChan)
		defer close(errChan)

		for {
			// Set a deadline for receiving the next message and refresh it before each message
			conn.SetReadDeadline(time.Now().Add(60 * time.Second))

			var n uint32
			n, err := netReadUint32(conn)
			if err != nil {
				errChan <- fmt.Errorf("Could not read first byte of new message: '%s'", err)
				break
			}
			if n > 130*1024 {
				errChan <- fmt.Errorf("Received message was too large: %d", n)
				break
			}

			var buf []byte
			if n == 0 {
				// keep-alive - we want an empty message
				buf = make([]byte, 1)
			} else {
				buf = make([]byte, n)
			}

			_, err = io.ReadFull(conn, buf)
			if err != nil {
				errChan <- fmt.Errorf("Could not get the whole mssage: '%s'", err)
			}
			msgChan <- buf
		}
	}()

	return msgChan, errChan
}

func createPeerWriter(conn net.Conn) (chan<- []byte, <-chan error) {
	msgChan := make(chan []byte)
	errChan := make(chan error)

	go func() {
		defer log.V(3).Infof("WINSTON: Peer writer goroutine exited\n")
		defer close(errChan)
		// msgChan should be closed by the caller

		for msg := range msgChan {
			// Set a deadline for sending the next message and refresh it before each message
			conn.SetWriteDeadline(time.Now().Add(30 * time.Second))

			err := netWriteUint32(conn, uint32(len(msg)))
			if err != nil {
				errChan <- fmt.Errorf("Could not send byte of new message: '%s'", err)
				break
			}
			_, err = conn.Write(msg)
			if err != nil {
				errChan <- fmt.Errorf("Could not send a message: '%s'", err)
				break
			}
		}
	}()

	return msgChan, errChan
}

// DownloadMetadataFromPeer is used to connect to the specified peer
// and download the torrent metadata for the specified infoHash from them
func DownloadMetadataFromPeer(remotePeer, infoHash string) (downloadedTorrent []byte) {
	ourPeerID := getNewPeerID()

	log.V(2).Infof("WINSTON (peer %s): Connecting to %s for torrent %x\n", ourPeerID, remotePeer, infoHash)

	conn, theirFlags, theirInfoHash, theirPeerID, err := initiateConnectionToPeer(remotePeer, ourPeerID, infoHash)
	if err != nil {
		log.V(2).Infof("WINSTON (peer %s): Error connecting to peer %s: '%s'\n", ourPeerID, remotePeer, err)
		return
	}
	defer conn.Close()
	log.V(2).Infof("WINSTON (peer %s): Connection successful! Remote peer %s is %q, has torrent '%x' and flags '%x'\n", ourPeerID, remotePeer, theirPeerID, theirInfoHash, theirFlags)

	readChan, readErrors := createPeerReader(conn)
	writeChan, writeErrors := createPeerWriter(conn)
	//TODO: add keep alive ticker
	defer close(writeChan)

	// Send the BEP10 handshake message
	writeChan <- getExtensionsHandshakeMsg()

	//TODO: refactor method, this is getting too long and complicated

	receivedHandshakeInfo := false
	expectedMetadataPiece := 0

	// These will be initialized once we receive the extension handshake
	var theirExtensionHandshake extensionHandshake
	var receivedMetadata []byte
	var totalPieces int

	for {
		select {
		case newMessage, chanOk := <-readChan:
			if !chanOk {
				log.V(2).Infof("WINSTON (peer %s): Reader channel unexpectedly closed!\n", ourPeerID)
				return
			}

			log.V(3).Infof("WINSTON (peer %s): Received new message from %s: %q\n", ourPeerID, remotePeer, newMessage)
			// Ignore every message except BEP10 extension messages
			// TODO: handle other types of messages, if only for statistical purposes
			if newMessage[0] != msgExtension {
				continue
			}

			// Check if this is the handshake message for the extension protocol
			if newMessage[1] == 0 {
				//TODO: handle multiple extension handshame messages from the same peer? BEP10 allows it
				log.V(3).Infof("WINSTON (peer %s): Received extensions handshake from %s. Parsing...\n", ourPeerID, remotePeer)

				theirExtensionHandshake, err = parseAndValidateExtensionHandshake(newMessage[2:])
				if err != nil {
					log.V(2).Infof("WINSTON (peer %s): Could not parse extensions handshake from %s (%s)\n", ourPeerID, remotePeer, err)
					return
				}
				receivedHandshakeInfo = true
				log.V(2).Infof("WINSTON (peer %s): Parsed extension message from %s: %+v\n", ourPeerID, remotePeer, theirExtensionHandshake)

				// Prepare for receiving the metadata
				expectedMetadataPiece = 0
				totalPieces = int(theirExtensionHandshake.MetadataSize / (16 * 1024))
				receivedMetadata = make([]byte, theirExtensionHandshake.MetadataSize)

				// Request the first metadata piece
				writeChan <- getMetadataRequestPieceMsg(expectedMetadataPiece, theirExtensionHandshake.M["ut_metadata"])
				continue
			} else if newMessage[1] != winstonExtensionUtMetadata {
				log.V(2).Infof("WINSTON (peer %s): Received unsupported extension message from %s: %q\n", ourPeerID, remotePeer, newMessage)
				return
			}

			if !receivedHandshakeInfo {
				log.V(2).Infof("WINSTON (peer %s): Peer %s tried to send ut_metadata message before handshake...\n", ourPeerID, remotePeer)
				return
			}

			err := receiveMetadataPiece(expectedMetadataPiece, receivedMetadata, newMessage[2:])
			if err != nil {
				log.V(2).Infof("WINSTON (peer %s): Error receiving metadata piece %d of %d: %s\n", ourPeerID, expectedMetadataPiece+1, totalPieces+1, err)
				return
			}
			log.V(2).Infof("WINSTON (peer %s): Successfully received piece %d of %d for %x from %s!\n", ourPeerID, expectedMetadataPiece+1, totalPieces+1, infoHash, remotePeer)

			if expectedMetadataPiece == totalPieces {
				sha := sha1.New()
				sha.Write(receivedMetadata)
				actualHash := string(sha.Sum(nil))
				if actualHash != infoHash {
					log.V(2).Infof("WINSTON (peer %s): Received invalid metadata from %s: received %x, expected %x\n", ourPeerID, remotePeer, actualHash, infoHash)
				} else {
					log.V(1).Infof("WINSTON (peer %s): SUCCESSFULLY DOWNLOADED %x from %s\n", ourPeerID, infoHash, remotePeer)
					downloadedTorrent = receivedMetadata
				}
				return
			}

			// Request the next metadata piece
			expectedMetadataPiece++
			writeChan <- getMetadataRequestPieceMsg(expectedMetadataPiece, theirExtensionHandshake.M["ut_metadata"])

		case readErr := <-readErrors:
			log.V(2).Infof("WINSTON (peer %s): Read error: %s\n", ourPeerID, readErr)
			return

		case writeErr := <-writeErrors:
			log.V(2).Infof("WINSTON (peer %s): Write error: %s\n", ourPeerID, writeErr)
			return
		}
	}
}

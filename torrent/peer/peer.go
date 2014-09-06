// Package peer is used for connecting to BitTorrent peers and downloading torrent metadata from them
//
// Most of the code in this file is copied from or heavily influenced by Taipei-Torrent: https://github.com/jackpal/Taipei-Torrent
// Taipei-Torrent is copyrighted (c) Jack Palevich and others: https://github.com/jackpal/Taipei-Torrent/graphs/contributors

package peer

import (
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"strconv"
	"time"

	log "github.com/golang/glog"
)

func getNewPeerID() string {
	sid := "-md" + strconv.Itoa(os.Getpid()) + "_" + strconv.FormatInt(rand.Int63(), 10)
	return sid[0:20]
}

func getSessionHeader(infoHash string, peerID string) []byte {
	header := make([]byte, 68)
	copy(header, bitTorrentHeader[0:])

	header[25] |= 0x10 // Support Extension Protocol (BEP-0010)

	// TODO: enable this again when DHT is natively supported
	// header[27] |= 0x01 // Support DHT
	copy(header[28:48], string2Bytes(infoHash))
	copy(header[48:68], string2Bytes(peerID))
	return header
}

func createPeerReader(conn net.Conn) (<-chan []byte, <-chan error) {
	msgChan := make(chan []byte)
	errChan := make(chan error)

	go func() {
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
		// msgChan should be closed by the calee
		defer close(errChan)
		defer conn.Close() // If the write channel is closed, close the connection with the peer

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
				errChan <- fmt.Errorf("Could not sed a  message: '%s'", err)
				break
			}
		}
	}()

	return msgChan, errChan
}

func readHeader(conn net.Conn) (h []byte, err error) {
	header := make([]byte, 68)
	_, err = conn.Read(header[0:1])
	if err != nil {
		err = fmt.Errorf("Couldn't read 1st byte: %v", err)
		return
	}
	if header[0] != 19 {
		err = fmt.Errorf("First byte is not 19")
		return
	}
	_, err = conn.Read(header[1:20])
	if err != nil {
		err = fmt.Errorf("Couldn't read magic string: %v", err)
		return
	}
	if string(header[1:20]) != "BitTorrent protocol" {
		err = fmt.Errorf("Magic string is not correct: %v", string(header[1:20]))
		return
	}
	// Read rest of header
	_, err = conn.Read(header[20:])
	if err != nil {
		err = fmt.Errorf("Couldn't read rest of header")
		return
	}

	h = make([]byte, 48)
	copy(h, header[20:])
	return
}

func initiateConnectionToPeer(remotePeer, ourPeerID, wantedInfoHash string) (theirFlags []byte, theirInfoHash, theirPeerID string, err error) {
	ourSessionHader := getSessionHeader(wantedInfoHash, ourPeerID)

	conn, err := net.DialTimeout("tcp", remotePeer, 5*time.Second)
	if err != nil {
		err = fmt.Errorf("Could not connect (%s)", err)
		return
	}
	log.V(3).Infof("WINSTON (peer %s): Connected to peer %s!\n", ourPeerID, remotePeer)

	// We want the connection operations to finish in the next 20 seconds
	conn.SetDeadline(time.Now().Add(20 * time.Second))

	_, err = conn.Write(ourSessionHader)
	if err != nil {
		err = fmt.Errorf("Failed to send header (%s)", err)
		return
	}

	theirHeader, err := readHeader(conn)
	if err != nil {
		err = fmt.Errorf("Error reading header (%s)", err)
		return
	}

	theirFlags = theirHeader[0:8]
	theirInfoHash = string(theirHeader[8:28])
	theirPeerID = string(theirHeader[28:48])

	if theirInfoHash != wantedInfoHash {
		err = fmt.Errorf("Remote infohash is %x", theirInfoHash)
		return
	}

	if int(theirFlags[5])&0x10 != 0x10 {
		err = fmt.Errorf("Remote torrent client does not support the extension protocol; flags are %x", theirFlags)
		return
	}

	return
}

// DownloadMetadataFromPeer is used to connect to the specified peer
// and download the torrent metadata for the specified infoHash from them
func DownloadMetadataFromPeer(remotePeer, infoHash string) {
	ourPeerID := getNewPeerID()

	log.V(2).Infof("WINSTON (peer %s): Connecting to %s for torrent %x\n", ourPeerID, remotePeer, infoHash)

	theirFlags, theirInfoHash, theirPeerID, err := initiateConnectionToPeer(remotePeer, ourPeerID, infoHash)
	if err != nil {
		log.V(2).Infof("WINSTON (peer %s): Error connecting to peer %s: '%s'\n", ourPeerID, remotePeer, err)
		return
	}

	log.V(2).Infof("WINSTON (peer %s): Connection successful! Remote peer is '%q', has torrent '%x' and flags '%x'\n", ourPeerID, theirPeerID, theirInfoHash, theirFlags)

	//TODO: send extensions
	//TODO: listen for responses
	//TODO: send

	os.Exit(1)
}

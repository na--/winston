// Package peer is used for connecting to BitTorrent peers and downloading torrent metadata from them
//
// Most of the code in this file is copied from or heavily influenced by Taipei-Torrent: https://github.com/jackpal/Taipei-Torrent
// Taipei-Torrent is copyrighted (c) Jack Palevich and others: https://github.com/jackpal/Taipei-Torrent/graphs/contributors

package peer

import (
	"fmt"
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
	header[27] |= 0x01 // Support DHT
	copy(header[28:48], string2Bytes(infoHash))
	copy(header[48:68], string2Bytes(peerID))
	return header
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

// DownloadMetadataFromPeer is used to connect to the specified peer
// and download the torrent metadata for the specified infoHash from them
func DownloadMetadataFromPeer(remotePeer string, infoHash string) {
	ourPeerID := getNewPeerID()
	ourSessionHader := getSessionHeader(infoHash, ourPeerID)

	log.V(2).Infof("WINSTON (peer %s): ============================================\n", ourPeerID)
	log.V(2).Infof("WINSTON (peer %s): Connecting to %s for torrent %x...\n", ourPeerID, remotePeer, infoHash)
	log.V(2).Infof("WINSTON (peer %s): Header %q...\n", ourPeerID, ourSessionHader)

	conn, err := net.DialTimeout("tcp", remotePeer, 5*time.Second)
	if err != nil {
		log.V(2).Infof("WINSTON (peer %s): Could not connect to peer %s: '%s'\n", ourPeerID, remotePeer, err)
		return
	}
	log.V(2).Infof("WINSTON (peer %s): Connected to peer %s!\n", ourPeerID, remotePeer)

	_, err = conn.Write(ourSessionHader)
	if err != nil {
		log.V(2).Infof("WINSTON (peer %s): Failed to send header to peer %s :(\n", ourPeerID, remotePeer)
		return
	}

	theirHeader, err := readHeader(conn)
	if err != nil {
		log.V(2).Infof("WINSTON (peer %s): Error reading header for peer %s : '%s'\n", ourPeerID, remotePeer, err)
		return
	}

	theirFlags := theirHeader[0:8]
	theirInfoHash := string(theirHeader[8:28])
	theirPeerID := string(theirHeader[28:48])

	log.V(2).Infof("WINSTON (peer %s): Connection successful! Remote peer is '%s', has torrent '%x' and flags '%x'\n", ourPeerID, theirPeerID, theirInfoHash, theirFlags)

	if theirInfoHash != infoHash {
		log.V(2).Infof("WINSTON (peer %s): Error remote infohash does not match: '%x'\n", ourPeerID, theirInfoHash)
		return
	}

	if int(theirFlags[5])&0x10 != 0x10 {
		log.V(2).Infof("WINSTON (peer %s): Error torrent client does not support the extension protocol\n", ourPeerID)
		return
	}

	log.V(2).Infof("WINSTON (peer %s): Happy dance!\n", ourPeerID)

	os.Exit(1)
}

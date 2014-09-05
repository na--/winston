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

const (
	extMessageMetadataRequest = iota
	extMessageMetadataData
	extMessageMetadataReject
)

var bitTorrentHeader = []byte{'\x13', 'B', 'i', 't', 'T', 'o', 'r',
	'r', 'e', 'n', 't', ' ', 'p', 'r', 'o', 't', 'o', 'c', 'o', 'l'}

func getNewPeerID() string {
	sid := "-md" + strconv.Itoa(os.Getpid()) + "_" + strconv.FormatInt(rand.Int63(), 10)
	return sid[0:20]
}

func getSessionHeader(infoHash string, peerID string) []byte {
	header := make([]byte, 68)
	copy(header, bitTorrentHeader[0:])

	header[25] |= 0x10             // Support Extension Protocol (BEP-0010)
	header[27] = header[27] | 0x01 // Support DHT
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
func DownloadMetadataFromPeer(peer string, infoHash string) {
	peerID := getNewPeerID()
	sessionHader := getSessionHeader(infoHash, peerID)

	log.V(2).Infof("WINSTON: ============================================\n")
	log.V(2).Infof("WINSTON: Connecting to %s for torrent %x...\n", peer, infoHash)
	log.V(2).Infof("WINSTON: Peer id %s...\n", peerID)
	log.V(2).Infof("WINSTON: Header %s...\n", sessionHader)
	defer log.V(2).Infof("WINSTON: ============================================\n")

	conn, err := net.DialTimeout("tcp", peer, 10*time.Second)
	if err != nil {
		log.V(2).Infof("WINSTON: Could not connect to peer %s: '%s'\n", peer, err)
		return
	}
	log.V(2).Infof("WINSTON: Connected to peer %s!\n", peer)

	_, err = conn.Write(sessionHader)
	if err != nil {
		log.V(2).Infof("WINSTON: Failed to send header to peer %s :(\n", peer)
		return
	}

	theirheader, err := readHeader(conn)
	if err != nil {
		log.V(2).Infof("WINSTON: Error reading header for peer %s : '%s'\n", peer, err)
		return
	}

	peersInfoHash := string(theirheader[8:28])
	id := string(theirheader[28:48])

	log.V(2).Infof("WINSTON: Success!!! Remote peer is '%s' with id '%s'\n", peersInfoHash, id)
	os.Exit(1)
}

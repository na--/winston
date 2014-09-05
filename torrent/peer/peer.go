// Package peer is used for connecting to BitTorrent peers and downloading torrent metadata from them
//
// Most of the code in this file is copied from or heavily influenced by Taipei-Torrent: https://github.com/jackpal/Taipei-Torrent
// Taipei-Torrent is copyrighted (c) Jack Palevich and others: https://github.com/jackpal/Taipei-Torrent/graphs/contributors

package peerconnector

import (
	"bytes"
	"math/rand"
	"os"
	"strconv"

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

func string2Bytes(s string) []byte {
	return bytes.NewBufferString(s).Bytes()
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

// DownloadMetadataFromPeer is used to connect to the specified peer
// and download the torrent metadata for the specified infoHash from them
func DownloadMetadataFromPeer(peer string, infoHash string) {
	peerID := getNewPeerID()

	log.V(2).Infof("WINSTON: ============================================\n")
	log.V(2).Infof("WINSTON: Connecting to %s for torrent %x...\n", peer, infoHash)
	log.V(2).Infof("WINSTON: Peer id %s...\n", peerID)
	log.V(2).Infof("WINSTON: Header %s...\n", getSessionHeader(infoHash, peerID))
	log.V(2).Infof("WINSTON: ============================================\n")
}

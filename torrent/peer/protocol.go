// Most of the code in this file is copied from or heavily influenced by Taipei-Torrent: https://github.com/jackpal/Taipei-Torrent
// Taipei-Torrent is copyrighted (c) Jack Palevich and others: https://github.com/jackpal/Taipei-Torrent/graphs/contributors

package peer

import (
	"bytes"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/jackpal/bencode-go"

	log "github.com/golang/glog"
)

type extensionHandshake struct {
	M            map[string]int `bencode:"m"`
	P            uint16         `bencode:"p"`
	V            string         `bencode:"v"`
	Yourip       string         `bencode:"yourip"`
	Ipv6         string         `bencode:"ipv6"`
	Ipv4         string         `bencode:"ipv4"`
	Reqq         uint16         `bencode:"reqq"`
	MetadataSize uint           `bencode:"metadata_size"`
}

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

func getExtensionsHandshakeMsg() []byte {
	//TODO: add port and other extensions
	handshake := map[string]interface{}{
		"m": map[string]int{
			"ut_metadata": 1,
		},
		"v": "Winston 0.01",
	}

	var buf bytes.Buffer
	err := bencode.Marshal(&buf, handshake)
	if err != nil {
		panic("Something went terribly wrong with bencoding a simple message...")
	}

	msg := make([]byte, 2+buf.Len())
	msg[0] = msgExtension
	msg[1] = 0 // This is a handshake message
	copy(msg[2:], buf.Bytes())

	return buf.Bytes()
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

func initiateConnectionToPeer(remotePeer, ourPeerID, wantedInfoHash string) (conn net.Conn, theirFlags []byte, theirInfoHash, theirPeerID string, err error) {
	ourSessionHader := getSessionHeader(wantedInfoHash, ourPeerID)

	conn, err = net.DialTimeout("tcp", remotePeer, 5*time.Second)
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

func parseExtensionHandshake(msg []byte) (result extensionHandshake, err error) {

	err = bencode.Unmarshal(bytes.NewReader(msg), &result)
	if err != nil {
		err = fmt.Errorf("Error when unmarshaling extension handshake (%s)", err)
		return
	}

	if _, ok := result.M["ut_metadata"]; !ok {
		err = fmt.Errorf("Metadata extension is not supported; only supported %v", result.M)
		return
	}

	if result.MetadataSize <= 0 {
		err = fmt.Errorf("Metadata size is %d", result.MetadataSize)
		return
	}

	return
}

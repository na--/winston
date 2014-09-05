// Most of the code in this file is copied from or heavily influenced by Taipei-Torrent: https://github.com/jackpal/Taipei-Torrent
// Taipei-Torrent is copyrighted (c) Jack Palevich and others: https://github.com/jackpal/Taipei-Torrent/graphs/contributors

package peer

import "bytes"

func uint32ToBytes(buf []byte, n uint32) {
	buf[0] = byte(n >> 24)
	buf[1] = byte(n >> 16)
	buf[2] = byte(n >> 8)
	buf[3] = byte(n)
}

func bytesToUint32(buf []byte) uint32 {
	return (uint32(buf[0]) << 24) |
		(uint32(buf[1]) << 16) |
		(uint32(buf[2]) << 8) | uint32(buf[3])
}

func string2Bytes(s string) []byte {
	return bytes.NewBufferString(s).Bytes()
}

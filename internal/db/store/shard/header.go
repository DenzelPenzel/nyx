package shard

import (
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/denzelpenzel/nyx/internal/utils"
	"io"
)

const (
	versionMarker   = 255
	currentShardVer = 1
	deleted         = 42
)

var (
	sizeHeaders = map[int]uint32{0: 8, 1: 12}
	sizeHead    = sizeHeaders[currentShardVer]
)

type Header struct {
	sizeByte  uint8
	status    uint8
	keyLength uint16
	valLength uint32
	expire    uint32
}

func makeHeader(k, v []byte, expire uint32) *Header {
	header := &Header{
		status:    0,
		keyLength: uint16(len(k)),
		valLength: uint32(len(v)),
		expire:    expire,
	}
	sizeByte, _ := utils.NextPowerOf2(uint32(header.keyLength) + header.valLength + sizeHead)
	header.sizeByte = sizeByte
	return header
}

func parseHeader(b []byte) *Header {
	header := &Header{}
	header.sizeByte = b[0]
	header.status = b[1]
	header.keyLength = binary.BigEndian.Uint16(b[2:4])
	header.valLength = binary.BigEndian.Uint32(b[4:8])
	header.expire = binary.BigEndian.Uint32(b[8:12])
	return header
}

func readHeader(r io.Reader, ver int) (*Header, error) {
	var header *Header
	var err error
	b := make([]byte, sizeHeaders[ver])
	n, err := io.ReadFull(r, b)
	if n != int(sizeHeaders[ver]) {
		if errors.Is(err, io.EOF) {
			err = nil
		}
		return header, err
	}
	if ver == currentShardVer {
		header = parseHeader(b)
	} else {
		err = fmt.Errorf("wrong header version %d", ver)
	}
	return header, err
}

func writeHeader(b []byte, header *Header) {
	b[0] = header.sizeByte
	b[1] = header.status
	binary.BigEndian.PutUint16(b[2:4], header.keyLength)
	binary.BigEndian.PutUint32(b[4:8], header.valLength)
	binary.BigEndian.PutUint32(b[8:12], header.expire)
}

func marshal(k, v []byte, expire uint32) (*Header, []byte) {
	header := makeHeader(k, v, expire)
	size := 1 << header.sizeByte
	b := make([]byte, size)
	writeHeader(b, header)
	copy(b[sizeHead:], v)
	copy(b[sizeHead+header.valLength:], k)
	return header, b
}

func unmarshal(b []byte) (*Header, []byte, []byte) {
	header := parseHeader(b)
	k := b[sizeHead+header.valLength : sizeHead+header.valLength+uint32(header.keyLength)]
	v := b[sizeHead : sizeHead+header.valLength]
	return header, k, v
}

package record_header

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/DenzelPenzel/nyx/internal/utils"
)

// RecordVersionMarker is a constant marker used in the record header
const RecordVersionMarker = 255

// CurrentRecordVersion defines the current version for record headers
const CurrentRecordVersion = 1

// RecordDeletedMarker indicates that the record has been marked as RecordDeletedMarker
const RecordDeletedMarker = 42

// HeaderSizes maps a version to its fixed header size in bytes
var HeaderSizes = map[int]uint32{
	0: 8,
	1: 12,
}

// HeaderFixedSize is the fixed header size for the current record version
var HeaderFixedSize = HeaderSizes[CurrentRecordVersion]

// RecordHeader represents the metadata for a record stored in a shard
type RecordHeader struct {
	SizeByte  uint8  // Exponent: record size = 1 << SizeByte
	Status    uint8  // 0 means active; RecordDeletedMarker means RecordDeletedMarker
	KeyLength uint16 // Length of the key
	ValLength uint32 // Length of the value
	Expire    uint32 // Expiration time (Unix timestamp in seconds)
}

func NewRecordHeader(key, val []byte, expire uint32) *RecordHeader {
	header := &RecordHeader{
		Status:    0,
		KeyLength: uint16(len(key)),
		ValLength: uint32(len(val)),
		Expire:    expire,
	}
	// Determine the smallest power-of-two size that fits the entire record
	sizeByte, _ := utils.NextPowerOf2(uint32(header.KeyLength) + header.ValLength + HeaderFixedSize)
	header.SizeByte = sizeByte
	return header
}

func ParseRecordHeader(b []byte) *RecordHeader {
	return &RecordHeader{
		SizeByte:  b[0],
		Status:    b[1],
		KeyLength: binary.BigEndian.Uint16(b[2:4]),
		ValLength: binary.BigEndian.Uint32(b[4:8]),
		Expire:    binary.BigEndian.Uint32(b[8:12]),
	}
}

// ReadRecordHeader reads a RecordHeader from the provided reader based on the version
// If the reader reaches EOF without any data, a nil header and nil error are returned
func ReadRecordHeader(r io.Reader, ver int) (*RecordHeader, error) {
	size, ok := HeaderSizes[ver]
	if !ok {
		return nil, fmt.Errorf("unsupported header version %d", ver)
	}
	b := make([]byte, size)
	n, err := io.ReadFull(r, b)
	if n != int(size) {
		// If we hit EOF before reading any header, return nil header with no error
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		return nil, err
	}
	if ver == CurrentRecordVersion {
		return ParseRecordHeader(b), nil
	}
	return nil, fmt.Errorf("unexpected header version %d", ver)
}

func WriteRecordHeader(b []byte, header *RecordHeader) {
	b[0] = header.SizeByte
	b[1] = header.Status
	binary.BigEndian.PutUint16(b[2:4], header.KeyLength)
	binary.BigEndian.PutUint32(b[4:8], header.ValLength)
	binary.BigEndian.PutUint32(b[8:12], header.Expire)
}

// Marshal builds a complete binary record for the given key, value, and expiration
// The resulting record consists of the fixed header, followed by the value, and then the key
func Marshal(key, value []byte, expire uint32) (*RecordHeader, []byte) {
	header := NewRecordHeader(key, value, expire)
	recordSize := 1 << header.SizeByte
	b := make([]byte, recordSize)
	WriteRecordHeader(b, header)
	// Layout: fixed header | value | key
	copy(b[HeaderFixedSize:], value)
	copy(b[HeaderFixedSize+header.ValLength:], key)
	return header, b
}

func Unmarshal(b []byte) (*RecordHeader, []byte, []byte) {
	header := ParseRecordHeader(b)
	value := b[HeaderFixedSize : HeaderFixedSize+header.ValLength]
	key := b[HeaderFixedSize+header.ValLength : HeaderFixedSize+header.ValLength+uint32(header.KeyLength)]
	return header, key, value
}

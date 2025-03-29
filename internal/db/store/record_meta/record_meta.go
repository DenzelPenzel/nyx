package record_meta

import (
	"errors"
	"os"

	"github.com/DenzelPenzel/nyx/internal/db/store/record_header"
)

// Encode packs the given record pointer components into a single uint64 value
// The encoding layout is as follows:
//   - Upper 32 bits: file address (addr)
//   - Next 8 bits: record size (size)
//   - Lower 24 bits: expiration time, stored as (expire >> 9)
//
// Note: The expiration value is compressed by discarding the lower 9 bits
func Encode(addr uint32, size byte, expire uint32) uint64 {
	return uint64(addr)<<32 | uint64(size)<<24 | uint64(expire)>>9
}

// Decode unpacks the encoded record meta into its components
// The expiration value is reconstructed by left-shifting the stored 24-bit value by 9 bits
// and then, if non-zero, adding (1<<9)-1 (i.e. 511) seconds as an adjustment
func Decode(meta uint64) (uint32, byte, uint32) {
	add := uint32(meta >> 32)
	size := byte(meta >> 24 & 0xff)
	expire := uint32(meta&0xffffff) << 9
	if expire != 0 {
		expire += (1 << 9) - 1 // added 511 sec
	}
	return add, size, expire
}

// ReadFileVersion reads the first two bytes from the given file to determine the shard file version.
// It returns 0 if the header indicates an old version or if the second byte is zero or equals the deleted marker.
// Returns an error if the file is too short or an unexpected marker is found
func ReadFileVersion(file *os.File) (int, error) {
	b := make([]byte, 2)
	n, err := file.Read(b)
	if err != nil {
		return -1, err
	}
	if n != 2 {
		return -1, errors.New("file too short to determine version")
	}
	if b[0] == record_header.RecordVersionMarker {
		if b[1] == 0 || b[1] == record_header.RecordDeletedMarker {
			return 0, nil
		}
		return int(b[1]), nil
	}
	if b[1] == 0 || b[1] == record_header.RecordDeletedMarker {
		return 0, nil
	}
	return -1, nil
}

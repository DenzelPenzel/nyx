package shard

import (
	"errors"
	"os"
)

func Encode(addr uint32, size byte, expire uint32) uint64 {
	return uint64(addr)<<32 | uint64(size)<<24 | uint64(expire)>>9
}

func Decode(key uint64) (uint32, byte, uint32) {
	add := uint32(key >> 32)
	size := byte(key >> 24 & 0xff)
	expire := uint32(key&0xffffff) << 9
	if expire != 0 {
		expire += 1<<9 - 1 // added 511 sec
	}
	return add, size, expire
}

func getFileVer(file *os.File) (int, error) {
	b := make([]byte, 2)
	n, err := file.Read(b)
	if err != nil {
		return -1, err
	}
	if n != 2 {
		return -1, errors.New("short file")
	}
	if b[0] == versionMarker {
		if b[1] == 0 || b[1] == deleted {
			return 0, nil
		}
		return int(b[1]), nil
	}
	if b[1] == 0 || b[1] == deleted {
		return 0, nil
	}
	return -1, nil
}

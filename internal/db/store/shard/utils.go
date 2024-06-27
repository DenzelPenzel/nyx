package shard

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

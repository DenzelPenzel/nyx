package shard

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/denzelpenzel/nyx/internal/common"
	"github.com/denzelpenzel/nyx/internal/utils"
	"github.com/spaolacci/murmur3"
	"io"
	"os"
	"sync"
	"time"
)

type Shard struct {
	sync.RWMutex
	f         *os.File          // file storage
	mapping   map[uint32]uint64 // keys mapping
	remapping map[uint32]byte   // space remapping: addr /size
	useFsync  bool
}

var forceExit bool

func getShardVer(file *os.File) (int, error) {
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

func (s *Shard) Run(name string) error {
	s.Lock()
	defer s.Unlock()

	forceExit = false

	f, err := os.OpenFile(name, os.O_CREATE|os.O_RDWR, os.FileMode(0644))
	if err != nil {
		return err
	}

	err = f.Sync()
	if err != nil {
		return err
	}

	s.f = f
	s.mapping = make(map[uint32]uint64)
	s.remapping = make(map[uint32]byte)

	if fi, err := s.f.Stat(); err == nil {
		// create a new file
		if fi.Size() == 0 {
			// write shard info to the file
			_, err = s.f.Write([]byte{versionMarker, currentShardVer})
			if err != nil {
				return err
			}
			return nil
		}

		// read file
		var seek uint32
		ver, err := getShardVer(s.f)
		if err != nil {
			return err
		}

		if ver < 0 || ver > currentShardVer {
			return errors.New("unknown Shard version in file " + name)
		}

		if ver == 0 {
			_, err = s.f.Seek(0, 0)
			if err != nil {
				return err
			}
		} else {
			seek = 2
		}

		if ver > currentShardVer {
			return fmt.Errorf("unsupported Shard: name: %s version: %d", name, ver)
		}

		if ver < currentShardVer {
			var newFile *os.File
			newName := name + ".new"
			newFile, err := os.OpenFile(newName, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.FileMode(0644))
			if err != nil {
				return err
			}
			// write version
			_, err = newFile.Write([]byte{versionMarker, currentShardVer})
			if err != nil {
				return err
			}
			seek = 2
			oldSizeHead := sizeHeaders[ver]
			sizeDiff := sizeHead - oldSizeHead
			for {
				header, err := readHeader(s.f, ver)
				if err != nil {
					newFile.Close()
					return err
				}
				if header == nil {
					break
				}
				oldSizeData := (1 << header.sizeByte) - oldSizeHead
				sizeb, size := utils.NextPowerOf2(sizeHead + uint32(header.keyLength) + header.valLength)
				header.sizeByte = sizeb

				b := make([]byte, size+sizeDiff)
				writeHeader(b, header)
				n, err := s.f.Read(b[sizeHead : sizeHead+oldSizeData])
				if err != nil {
					return err
				}
				if n != int(oldSizeData) {
					return fmt.Errorf("wrong shart len: %d", n)
				}

				if header.status == deleted || (header.expire != 0 && int64(header.expire) < time.Now().Unix()) {
					continue
				}

				startPos := int(sizeHead) + int(header.valLength)
				endPos := int(sizeHead) + int(header.valLength) + int(header.keyLength)
				h := murmur3.Sum32WithSeed(b[startPos:endPos], 0)

				s.mapping[h] = Encode(seek, header.sizeByte, header.expire)
				n, err = newFile.Write(b[0:size])
				if err != nil {
					return err
				}
				seek += uint32(n)
			}

			err = s.f.Close()
			if err != nil {
				return err
			}

			s.f = newFile
			err = os.Remove(name)
			if err != nil {
				return err
			}

			err = os.Rename(newName, name)
			if err != nil {
				return err
			}
			return nil
		}

		for {
			header, err := readHeader(s.f, ver)
			if err != nil {
				return err
			}
			if header == nil {
				break
			}

			_, err = s.f.Seek(int64(header.valLength), 1)
			if err != nil {
				return err
			}

			// read key
			key, err := s.readKey(header.keyLength)
			if err != nil {
				return err
			}
			shift := 1 << header.sizeByte
			// skip empty tail
			res, err := s.f.Seek(int64(shift-int(header.keyLength)-int(header.valLength)-int(sizeHead)), 1) // skip empty tail
			if err != nil {
				return err
			}

			if header.status != deleted && (header.expire == 0 || int64(header.expire) >= time.Now().Unix()) {
				h := murmur3.Sum32WithSeed(key, 0)
				s.mapping[h] = Encode(seek, header.sizeByte, header.expire)
			} else {
				s.remapping[seek] = header.sizeByte
			}

			seek = uint32(res)
		}
	}
	return nil
}

func (s *Shard) readKey(keyLen uint16) ([]byte, error) {
	key := make([]byte, keyLen)
	n, err := s.f.Read(key)
	if err != nil {
		return nil, err
	}
	if n != int(keyLen) {
		return nil, fmt.Errorf("invalid read length n != key: %d", n)
	}
	return key, nil
}

// Fsync ... Commit the current state to the persistent storage
func (s *Shard) Fsync() error {
	if s.useFsync {
		s.Lock()
		defer s.Unlock()
		s.useFsync = false
		return s.f.Sync()
	}
	return nil
}

func (s *Shard) ExpireKeys(maxRuntime time.Duration) error {
	startTime := time.Now().UnixMilli()
	current := startTime / 1000
	expired := make([]uint32, 0, 1024)

	if maxRuntime.Seconds() > 1000 {
		maxRuntime = time.Duration(1000) * time.Second
	}

	endTime := startTime + maxRuntime.Milliseconds()

	s.RLock()

	for key, val := range s.mapping {
		_, _, expire := Decode(val)
		if expire != 0 && current > int64(expire) {
			expired = append(expired, key)
		}
	}

	s.RUnlock()
	if len(expired) == 0 {
		return nil
	}

	sleepTime := maxRuntime.Milliseconds() / int64(len(expired)) / 2
	totalBulk := 1

	if sleepTime < 1 {
		totalBulk = len(expired)/int(maxRuntime.Milliseconds()+1) + 1
		sleepTime = 1
	} else if sleepTime > 10 {
		sleepTime = 10
	}

	if maxRuntime == time.Duration(0) {
		totalBulk = 1000
		sleepTime = 0
		endTime = startTime + 300000
	}

	s.Lock()

	cnt := 0
	bulkCount := 0
	for _, h := range expired {
		if forceExit || time.Now().UnixMilli() >= endTime {
			break
		}
		cnt++
		data, ok := s.mapping[h]
		if ok {
			addr, sizeb, expire := Decode(data)
			if expire != 0 && current > int64(expire) {
				delete(s.mapping, h)
				s.remapping[addr] = sizeb
			}
		}
		bulkCount++
		if bulkCount >= totalBulk {
			s.Unlock()
			time.Sleep(time.Duration(sleepTime) * time.Millisecond)
			s.Lock()
			bulkCount = 0
		}
	}

	s.Unlock()

	return nil
}

func (s *Shard) Set(k, v []byte, h, expire uint32) error {
	s.Lock()
	defer s.Unlock()
	return s.write(k, v, h, expire)
}

func (s *Shard) write(k, v []byte, h, expire uint32) error {
	var err error
	s.useFsync = true
	header, b := marshal(k, v, expire)
	// write at file
	pos := int64(-1)

	if data, ok := s.mapping[h]; ok {
		addr, size, _ := Decode(data)
		bb := make([]byte, 1<<size)
		_, err := s.f.ReadAt(bb, int64(addr))
		if err != nil {
			return err
		}
		oldHeader, key, _ := unmarshal(bb)
		if !bytes.Equal(key, k) {
			return common.ErrCollision
		}

		if oldHeader.sizeByte == header.sizeByte {
			pos = int64(addr)
		} else {
			delByte := []byte{deleted}
			_, err := s.f.WriteAt(delByte, int64(addr+1))
			if err != nil {
				return err
			}
			s.remapping[addr] = oldHeader.sizeByte

			for addrKey, sizeh := range s.remapping {
				if sizeh == header.sizeByte {
					pos = int64(addrKey)
					delete(s.remapping, addrKey)
					break
				}
			}
		}
	}

	if pos < 0 {
		// append to the end of file
		pos, err = s.f.Seek(0, 2)
	}

	_, err = s.f.WriteAt(b, pos)
	if err != nil {
		return err
	}

	s.mapping[h] = Encode(uint32(pos), header.sizeByte, header.expire)
	return nil
}

func (s *Shard) Touch(k []byte, h, expire uint32) error {
	s.Lock()
	defer s.Unlock()

	if data, ok := s.mapping[h]; ok {
		addr, size, _ := Decode(data)
		bb := make([]byte, 1<<size)
		_, err := s.f.ReadAt(bb, int64(addr))
		if err != nil {
			return err
		}
		header, key, _ := unmarshal(bb)
		if !bytes.Equal(key, k) {
			return common.ErrCollision
		}

		if header.expire != 0 && int64(header.expire) < time.Now().Unix() {
			return errors.New("invalid key")
		}

		header.expire = expire
		b := make([]byte, sizeHead)
		writeHeader(b, header)
		_, err = s.f.WriteAt(b, int64(addr))
		if err != nil {
			return err
		}
		s.useFsync = true
	} else {
		return common.ErrKeyNotFound
	}

	return nil
}

func (s *Shard) Get(k []byte, h uint32) ([]byte, *Header, error) {
	s.Lock()
	defer s.Unlock()
	return s.get(k, h)
}

func (s *Shard) get(k []byte, h uint32) ([]byte, *Header, error) {
	if data, ok := s.mapping[h]; ok {
		addr, size, expire := Decode(data)

		if expire != 0 && int64(expire) < time.Now().Unix() {
			delete(s.mapping, h)
			s.remapping[addr] = size
			return nil, nil, errors.New("key expired")
		}

		bb := make([]byte, 1<<size)
		_, err := s.f.ReadAt(bb, int64(addr))
		if err != nil {
			return nil, nil, err
		}

		header, key, val := unmarshal(bb)
		if !bytes.Equal(key, k) {
			return nil, nil, common.ErrCollision
		}

		if header.expire != 0 && int64(header.expire) < time.Now().Unix() {
			delete(s.mapping, h)
			s.remapping[addr] = size
			return nil, nil, errors.New("key expired")
		}

		return val, header, nil
	}

	return nil, nil, common.ErrKeyNotFound
}

func (s *Shard) length() int {
	s.RLock()
	defer s.RUnlock()
	return len(s.mapping)
}

func (s *Shard) Close() error {
	forceExit = true
	s.Lock()
	defer s.Unlock()
	return s.f.Close()
}

func (s *Shard) FileSize() (int64, error) {
	s.Lock()
	defer s.Unlock()
	f, err := s.f.Stat()
	if err != nil {
		return -1, err
	}
	return f.Size(), nil
}

func (s *Shard) Delete(k []byte, h uint32) (bool, error) {
	s.Lock()
	defer s.Unlock()
	if data, ok := s.mapping[h]; ok {
		addr, size, _ := Decode(data)
		bb := make([]byte, 1<<size)
		_, err := s.f.ReadAt(bb, int64(addr))
		if err != nil {
			return false, err
		}
		header, key, _ := unmarshal(bb)
		if !bytes.Equal(key, k) {
			return false, common.ErrCollision
		}
		// found the key now can delete it
		_, err = s.f.WriteAt([]byte{deleted}, int64(addr+1))
		if err != nil {
			return false, err
		}
		delete(s.mapping, h)
		s.remapping[addr] = header.sizeByte
		return true, nil
	}
	return false, nil
}

func (s *Shard) Counter(k []byte, h uint32, v uint64, inc bool) (uint64, error) {
	s.Lock()
	defer s.Unlock()
	old, header, err := s.get(k, h)
	expire := uint32(0)
	if header != nil {
		expire = header.expire
	}

	if errors.Is(err, common.ErrKeyNotFound) {
		old = make([]byte, 8)
		err = nil
	}

	if len(old) != 8 {
		return 0, errors.New("wrong format")
	}

	if err != nil {
		return 0, err
	}

	cnt := binary.BigEndian.Uint64(old)
	if inc {
		cnt += v
	} else {
		cnt -= v
	}

	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, cnt)
	err = s.write(k, b, h, expire)
	return cnt, err
}

func (s *Shard) Count() int {
	s.RLock()
	defer s.RUnlock()
	return len(s.mapping)
}

func (s *Shard) Backup(w io.Writer) error {
	s.Lock()
	defer s.Unlock()

	_, err := s.f.Seek(2, 0)
	if err != nil {
		return err
	}

	for {
		header, err := readHeader(s.f, currentShardVer)
		if err != nil {
			return err
		}
		if header == nil {
			break
		}

		size := int(sizeHead) + int(header.valLength) + int(header.keyLength)
		b := make([]byte, size)
		writeHeader(b, header)
		n, err := s.f.Read(b[sizeHead:])
		if err != nil {
			return err
		}

		if n != size-int(sizeHead) {
			return fmt.Errorf("wrong file size format, got: %d, expect: %d", n, size-int(sizeHead))
		}

		shift := 1 << header.sizeByte
		// move cursor pointer
		_, err = s.f.Seek(int64(shift-int(header.keyLength)-int(header.valLength)-int(sizeHead)), 1)
		if err != nil {
			return err
		}

		if header.status == deleted || (header.expire != 0 && int64(header.expire) < time.Now().Unix()) {
			continue
		}

		_, err = w.Write(b)
		if err != nil {
			return err
		}
	}

	return nil
}

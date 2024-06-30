package shard

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/DenzelPenzel/nyx/internal/common"
	"github.com/DenzelPenzel/nyx/internal/db/store/record_header"
	"github.com/DenzelPenzel/nyx/internal/db/store/record_meta"
	"github.com/DenzelPenzel/nyx/internal/utils"
	"github.com/spaolacci/murmur3"
)

type DataShard struct {
	sync.RWMutex
	file       *os.File          // file storage
	recordMap  map[uint32]uint64 // mapping: key hash -> encoded record (position, size, Expire)
	freeSpaces map[uint32]byte   // free space map: position -> size bucket available for reuse
	syncFSync  bool              // flag to trigger fsync on next sync
	exitExpire bool              // flag to signal expiration routine to exit
}

// upgradeFileFormat upgrades the file format from an older version to the current one.
func (ds *DataShard) upgradeFileFormat(oldVer int, filename string) error {
	newFilename := filename + ".new"
	newFile, err := os.OpenFile(newFilename, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.FileMode(0644))
	if err != nil {
		return err
	}

	// writeRecord the new version header
	_, err = newFile.Write([]byte{record_header.RecordVersionMarker, record_header.CurrentRecordVersion})
	if err != nil {
		newFile.Close()
		return err
	}

	newOffset := uint32(2)
	oldSizeHead := record_header.HeaderSizes[oldVer]
	sizeDiff := record_header.HeaderFixedSize - oldSizeHead

	for {
		header, err := record_header.ReadRecordHeader(ds.file, oldVer)
		if err != nil {
			newFile.Close()
			return err
		}
		if header == nil {
			break
		}
		oldDataSize := (1 << header.SizeByte) - oldSizeHead
		newSizeByte, newSize := utils.NextPowerOf2(record_header.HeaderFixedSize + uint32(header.KeyLength) + header.ValLength)
		header.SizeByte = newSizeByte

		buffer := make([]byte, newSize+sizeDiff)
		record_header.WriteRecordHeader(buffer, header)
		n, err := ds.file.Read(buffer[record_header.HeaderFixedSize : record_header.HeaderFixedSize+oldDataSize])
		if err != nil {
			newFile.Close()
			return err
		}
		if n != int(oldDataSize) {
			newFile.Close()
			return fmt.Errorf("invalid shard record length: expected %d, got %d", oldDataSize, n)
		}

		// Skip records marked as RecordDeletedMarker or expired
		if header.Status == record_header.RecordDeletedMarker || (header.Expire != 0 && int64(header.Expire) < time.Now().Unix()) {
			continue
		}

		startPos := int(record_header.HeaderFixedSize) + int(header.ValLength)
		endPos := int(record_header.HeaderFixedSize) + int(header.KeyLength) + int(header.ValLength)

		hash := murmur3.Sum32WithSeed(buffer[startPos:endPos], 0)

		ds.recordMap[hash] = record_meta.Encode(newOffset, header.SizeByte, header.Expire)
		n, err = newFile.Write(buffer[0:newSize])
		if err != nil {
			newFile.Close()
			return err
		}
		newOffset += uint32(n)
	}

	// Close old file
	if err := ds.file.Close(); err != nil {
		newFile.Close()
		return err
	}

	// Replace old file with new file
	ds.file = newFile
	// remove the old file from the disk
	if err := os.Remove(filename); err != nil {
		return err
	}
	// rename the new file filename to the old file filename
	return os.Rename(newFilename, filename)
}

func (ds *DataShard) processHeaders(ver int, offset uint32) error {
	for {
		header, err := record_header.ReadRecordHeader(ds.file, ver)
		if err != nil {
			return err
		}
		if header == nil {
			break
		}

		if _, err := ds.file.Seek(int64(header.ValLength), io.SeekCurrent); err != nil {
			return err
		}

		// read key
		key, err := ds.readKeyData(header.KeyLength)
		if err != nil {
			return err
		}

		shift := 1 << header.SizeByte

		// Skip the tail padding
		pos, err := ds.file.Seek(int64(shift-int(header.KeyLength)-int(header.ValLength)-int(record_header.HeaderFixedSize)), io.SeekCurrent)
		if err != nil {
			return err
		}

		if header.Status != record_header.RecordDeletedMarker && (header.Expire == 0 || int64(header.Expire) >= time.Now().Unix()) {
			h := murmur3.Sum32WithSeed(key, 0)
			ds.recordMap[h] = record_meta.Encode(offset, header.SizeByte, header.Expire)
		} else {
			ds.freeSpaces[offset] = header.SizeByte
		}

		// ????
		offset = uint32(pos)
	}

	return nil
}

// OpenShard opens (or creates) the shard file and initializes the in-memory index
func (ds *DataShard) OpenShard(filename string) error {
	ds.Lock()
	defer ds.Unlock()

	ds.exitExpire = false

	f, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, os.FileMode(0644))
	if err != nil {
		return err
	}

	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}

	ds.file = f
	ds.recordMap = make(map[uint32]uint64)
	ds.freeSpaces = make(map[uint32]byte)

	fi, err := ds.file.Stat()
	if err != nil {
		return err
	}

	// create a new file
	if fi.Size() == 0 {
		// New file: writeRecord version header.
		_, err = ds.file.Write([]byte{record_header.RecordVersionMarker, record_header.CurrentRecordVersion})
		return err
	}

	// read file
	var offset uint32
	ver, err := record_meta.ReadFileVersion(ds.file)
	if err != nil {
		return err
	}
	if ver < 0 || ver > record_header.CurrentRecordVersion {
		return errors.New("unknown shard version in file " + filename)
	}
	if ver == 0 {
		ds.file.Seek(0, 0)
	} else {
		offset = 2
	}
	if ver < record_header.CurrentRecordVersion {
		return ds.upgradeFileFormat(ver, filename)
	}
	return ds.processHeaders(ver, offset)
}

func (ds *DataShard) readKeyData(keyLen uint16) ([]byte, error) {
	key := make([]byte, keyLen)
	n, err := ds.file.Read(key)
	if err != nil {
		return nil, err
	}
	if n != int(keyLen) {
		return nil, fmt.Errorf("invalid key read length: expected %d, got %d", keyLen, n)
	}
	return key, nil
}

// Sync persists pending changes if needed
func (ds *DataShard) Sync() error {
	if ds.syncFSync {
		ds.Lock()
		defer ds.Unlock()
		ds.syncFSync = false
		return ds.file.Sync()
	}
	return nil
}

// ExpireExpiredKeys scans for expired records and removes them from the index
func (ds *DataShard) ExpireExpiredKeys(maxRuntime time.Duration) error {
	startTime := time.Now().UnixMilli()
	currentSec := startTime / 1000
	expired := make([]uint32, 0, 1024)

	if maxRuntime.Seconds() > 1000 {
		maxRuntime = 1000 * time.Second
	}
	endTime := startTime + maxRuntime.Milliseconds()

	ds.RLock()
	for h, val := range ds.recordMap {
		_, _, expire := record_meta.Decode(val)
		if expire != 0 && currentSec > int64(expire) {
			expired = append(expired, h)
		}
	}

	ds.RUnlock()
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

	if maxRuntime == 0 {
		totalBulk = 1000
		sleepTime = 0
		endTime = startTime + 300000
	}

	ds.Lock()

	bulkCount := 0
	for _, h := range expired {
		if ds.exitExpire || time.Now().UnixMilli() >= endTime {
			break
		}
		if data, ok := ds.recordMap[h]; ok {
			addr, sizeb, expire := record_meta.Decode(data)
			if expire != 0 && currentSec > int64(expire) {
				delete(ds.recordMap, h)
				ds.freeSpaces[addr] = sizeb
			}
		}
		bulkCount++
		if bulkCount >= totalBulk {
			ds.Unlock()
			time.Sleep(time.Duration(sleepTime) * time.Millisecond)
			ds.Lock()
			bulkCount = 0
		}
	}
	ds.Unlock()
	return nil
}

func (ds *DataShard) Set(key, value []byte, hash, expire uint32) error {
	ds.Lock()
	defer ds.Unlock()
	return ds.writeRecord(key, value, hash, expire)
}

// writeRecord write data on dist and in memory map
func (ds *DataShard) writeRecord(key, value []byte, hash, expire uint32) error {
	ds.syncFSync = true
	header, recordBytes := record_header.Marshal(key, value, expire)
	var pos int64 = -1

	if meta, ok := ds.recordMap[hash]; ok {
		addr, size, _ := record_meta.Decode(meta)
		bb := make([]byte, 1<<size)

		if _, err := ds.file.ReadAt(bb, int64(addr)); err != nil {
			return err
		}

		oldHeader, storedKey, _ := record_header.Unmarshal(bb)
		if !bytes.Equal(key, storedKey) {
			return common.ErrCollision
		}

		if oldHeader.SizeByte == header.SizeByte {
			pos = int64(addr)
		} else {
			// Mark existing record as RecordDeletedMarker
			if _, err := ds.file.WriteAt([]byte{record_header.RecordDeletedMarker}, int64(addr+1)); err != nil {
				return err
			}
			ds.freeSpaces[addr] = oldHeader.SizeByte
			// Try to find free space matching the new record's size
			for addrKey, sizeh := range ds.freeSpaces {
				if sizeh == header.SizeByte {
					pos = int64(addrKey)
					delete(ds.freeSpaces, addrKey)
					break
				}
			}
		}
	}

	if pos < 0 {
		var err error
		pos, err = ds.file.Seek(0, io.SeekEnd)
		if err != nil {
			return err
		}
	}

	if _, err := ds.file.WriteAt(recordBytes, pos); err != nil {
		return err
	}

	ds.recordMap[hash] = record_meta.Encode(uint32(pos), header.SizeByte, header.Expire)
	return nil
}

// Touch updates the expiration time of an existing record
func (ds *DataShard) Touch(key []byte, hash, expire uint32) error {
	ds.Lock()
	defer ds.Unlock()

	if data, ok := ds.recordMap[hash]; ok {
		addr, size, _ := record_meta.Decode(data)
		bb := make([]byte, 1<<size)

		if _, err := ds.file.ReadAt(bb, int64(addr)); err != nil {
			return err
		}

		header, storedKey, _ := record_header.Unmarshal(bb)
		if !bytes.Equal(storedKey, key) {
			return common.ErrCollision
		}

		if header.Expire != 0 && int64(header.Expire) < time.Now().Unix() {
			return errors.New("key expired")
		}

		header.Expire = expire
		b := make([]byte, record_header.HeaderFixedSize)
		record_header.WriteRecordHeader(b, header)
		if _, err := ds.file.WriteAt(b, int64(addr)); err != nil {
			return err
		}
		ds.syncFSync = true
	} else {
		return common.ErrKeyNotFound
	}

	return nil
}

func (ds *DataShard) Get(key []byte, hash uint32) ([]byte, *record_header.RecordHeader, error) {
	ds.Lock()
	defer ds.Unlock()
	return ds.get(key, hash)
}

func (ds *DataShard) get(key []byte, hash uint32) ([]byte, *record_header.RecordHeader, error) {
	if data, ok := ds.recordMap[hash]; ok {
		addr, size, expire := record_meta.Decode(data)

		if expire != 0 && int64(expire) < time.Now().Unix() {
			delete(ds.recordMap, hash)
			ds.freeSpaces[addr] = size
			return nil, nil, errors.New("key expired")
		}

		bb := make([]byte, 1<<size)
		if _, err := ds.file.ReadAt(bb, int64(addr)); err != nil {
			return nil, nil, err
		}

		header, storedKey, val := record_header.Unmarshal(bb)
		if !bytes.Equal(storedKey, key) {
			return nil, nil, common.ErrCollision
		}

		if header.Expire != 0 && int64(header.Expire) < time.Now().Unix() {
			delete(ds.recordMap, hash)
			ds.freeSpaces[addr] = size
			return nil, nil, errors.New("key expired")
		}

		return val, header, nil
	}

	return nil, nil, common.ErrKeyNotFound
}

func (ds *DataShard) Delete(key []byte, hash uint32) (bool, error) {
	ds.Lock()
	defer ds.Unlock()
	if data, ok := ds.recordMap[hash]; ok {
		addr, size, _ := record_meta.Decode(data)
		bb := make([]byte, 1<<size)

		if _, err := ds.file.ReadAt(bb, int64(addr)); err != nil {
			return false, err
		}

		header, storedKey, _ := record_header.Unmarshal(bb)
		if !bytes.Equal(storedKey, key) {
			return false, common.ErrCollision
		}

		// found the key now can delete it
		if _, err := ds.file.WriteAt([]byte{record_header.RecordDeletedMarker}, int64(addr+1)); err != nil {
			return false, err
		}

		delete(ds.recordMap, hash)
		ds.freeSpaces[addr] = header.SizeByte
		return true, nil
	}
	return false, nil
}

// Counter updates a numeric counter stored as an 8-byte value
func (ds *DataShard) Counter(key []byte, hash uint32, delta uint64, increment bool) (uint64, error) {
	ds.Lock()
	defer ds.Unlock()

	oldVal, header, err := ds.get(key, hash)
	expire := uint32(0)
	if header != nil {
		expire = header.Expire
	}

	if errors.Is(err, common.ErrKeyNotFound) {
		oldVal = make([]byte, 8)
		err = nil
	}

	if len(oldVal) != 8 {
		return 0, errors.New("invalid counter format")
	}

	if err != nil {
		return 0, err
	}

	cnt := binary.BigEndian.Uint64(oldVal)
	if increment {
		cnt += delta
	} else {
		cnt -= delta
	}
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, cnt)
	err = ds.writeRecord(key, b, hash, expire)
	return cnt, err
}

// Count returns the number of active records
func (ds *DataShard) Count() int {
	ds.RLock()
	defer ds.RUnlock()
	return len(ds.recordMap)
}

// Backup writes the shard's data to the provided writer
func (ds *DataShard) Backup(w io.Writer) error {
	ds.Lock()
	defer ds.Unlock()

	if _, err := ds.file.Seek(2, 0); err != nil {
		return err
	}

	for {
		header, err := record_header.ReadRecordHeader(ds.file, record_header.CurrentRecordVersion)
		if err != nil {
			return err
		}
		if header == nil {
			break
		}

		size := int(record_header.HeaderFixedSize) + int(header.ValLength) + int(header.KeyLength)
		buffer := make([]byte, size)
		record_header.WriteRecordHeader(buffer, header)
		n, err := ds.file.Read(buffer[record_header.HeaderFixedSize:])
		if err != nil {
			return err
		}

		if n != size-int(record_header.HeaderFixedSize) {
			return fmt.Errorf("unexpected record size: got %d, expected %d", n, size-int(record_header.HeaderFixedSize))
		}

		shift := 1 << header.SizeByte
		// move cursor pointer
		if _, err = ds.file.Seek(int64(shift-int(header.KeyLength)-int(header.ValLength)-int(record_header.HeaderFixedSize)), io.SeekCurrent); err != nil {
			return err
		}

		if header.Status == record_header.RecordDeletedMarker || (header.Expire != 0 && int64(header.Expire) < time.Now().Unix()) {
			continue
		}

		if _, err = w.Write(buffer); err != nil {
			return err
		}
	}

	return nil
}

// Close closes the underlying file
func (ds *DataShard) Close() error {
	ds.exitExpire = true
	ds.Lock()
	defer ds.Unlock()
	return ds.file.Close()
}

// FileSize returns the current size of the shard file
func (ds *DataShard) FileSize() (int64, error) {
	ds.Lock()
	defer ds.Unlock()
	fi, err := ds.file.Stat()
	if err != nil {
		return -1, err
	}
	return fi.Size(), nil
}

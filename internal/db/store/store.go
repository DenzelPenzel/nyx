package store

import (
	"compress/gzip"
	"errors"
	"fmt"
	"github.com/denzelpenzel/nyx/internal/common"
	"github.com/denzelpenzel/nyx/internal/db/store/shard"
	"github.com/denzelpenzel/nyx/internal/interval"
	"github.com/google/btree"
	"github.com/spaolacci/murmur3"
	"io"
	"os"
	"sync"
	"time"
)

type Store struct {
	sync.RWMutex
	shards         []shard.Shard
	shardsCount    int
	prefix         string
	shardColCnt    int
	expireShardSeq int

	dir            string
	syncInterval   time.Duration
	interv         interval.Interval
	expireInterval time.Duration
	expInterv      interval.Interval
	btree          *btree.BTree
}

// OptStore is a store options
type OptStore func(*Store) error

func Dir(dir string) OptStore {
	return func(s *Store) error {
		if dir == "" {
			dir = "."
		}
		_, err := os.Stat(dir)
		if err != nil {
			if os.IsNotExist(err) {
				if dir != "." {
					err = os.MkdirAll(dir, os.FileMode(0755))
					if err != nil {
						return err
					}
				}
			} else {
				return err
			}
		}
		s.dir = dir
		return nil
	}
}

// ShardsCollision ... Represents the number of shards used for resolving collisions
// The default value is 4, which is suitable for handling more than 1 billion keys
// with 8-byte alphabet keys without encountering collision errors.
// It's important to note that different keys may result in the same hash value,
// and collision shards are necessary for resolving such collisions without errors.
// If ShardCollisionCnt is set to zero, ErrCollision will be returned in case of a collision
func ShardsCollision(shards int) OptStore {
	return func(s *Store) error {
		s.shardColCnt = shards
		return nil
	}
}

func ShardsTotal(shards int) OptStore {
	return func(s *Store) error {
		s.shardsCount = shards
		return nil
	}
}

func ShardPrefix(prefix string) OptStore {
	return func(s *Store) error {
		s.prefix = prefix
		return nil
	}
}

// SyncInterval - how often fsync do, default 0 - OS will do it
func SyncInterval(interv time.Duration) OptStore {
	return func(s *Store) error {
		s.syncInterval = interv
		if interv > 0 {
			s.interv = interval.SetInterval(func(_ time.Time) {
				for i := range s.shards {
					err := s.shards[i].Fsync()
					if err != nil {
						panic(err)
					}
				}
			}, interv)
		}
		return nil
	}
}

func ExpireInterval(interv time.Duration) OptStore {
	return func(s *Store) error {
		s.expireInterval = interv
		if interv > 0 {
			s.expInterv = interval.SetInterval(func(_ time.Time) {
				err := s.shards[s.expireShardSeq].ExpireKeys(interv)
				if err != nil {
					fmt.Printf("Error expire:%s\n", err)
				}
				s.expireShardSeq++
				if s.expireShardSeq >= s.shardsCount {
					s.expireShardSeq = 0
				}
			}, interv)
		}
		return nil
	}
}

func Open(opts ...OptStore) (*Store, error) {
	s := &Store{
		syncInterval:   0,
		expireInterval: 0,
		shardColCnt:    4,
		shardsCount:    256,
		btree:          btree.New(32),
	}

	for _, opt := range opts {
		err := opt(s)
		if err != nil {
			return nil, err
		}
	}

	if s.shardsCount-s.shardColCnt < 1 {
		return nil, errors.New("shardsCount must be more then shardColCount at min 1")
	}

	stopWorkers := false
	s.shards = make([]shard.Shard, s.shardsCount)
	shChan := make(chan int, s.shardsCount)
	errChan := make(chan error, 4)

	var wg sync.WaitGroup

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			for i := range shChan {
				if stopWorkers {
					break
				}

				var filename string
				if s.prefix != "" {
					filename = fmt.Sprintf("%s/%s-%d", s.dir, s.prefix, i)
				} else {
					filename = fmt.Sprintf("%s/%d", s.dir, i)
				}

				err := s.shards[i].Run(filename)
				if err != nil {
					errChan <- err
					stopWorkers = true
					break
				}
			}
			wg.Done()
		}()
	}

	for i := range s.shards {
		shChan <- i
	}

	close(shChan)

	wg.Wait()

	if len(errChan) > 0 {
		err := <-errChan
		return s, err
	}

	s.btree = btree.New(32)

	return s, nil
}

func (s *Store) idx(h uint32) uint32 {
	return uint32((int(h) % (s.shardsCount - s.shardColCnt)) + s.shardColCnt)
}

// Set ...store key and val in shard, max packet size 2^19, 512kb (524288)
// packet size = len(key) + len(val) + 8
func (s *Store) Set(key, val []byte, expire uint32) error {
	h := murmur3.Sum32WithSeed(key, 0)
	err := s.shards[s.idx(h)].Set(key, val, h, expire)
	// handle collision issue
	if errors.Is(err, common.ErrCollision) {
		for i := 0; i < s.shardColCnt; i++ {
			err = s.shards[i].Set(key, val, h, expire)
			if errors.Is(err, common.ErrCollision) {
				continue
			}
			break
		}
	}
	return err
}

// Touch ... update key expire time
func (s *Store) Touch(key []byte, expire uint32) error {
	h := murmur3.Sum32WithSeed(key, 0)
	err := s.shards[s.idx(h)].Touch(key, h, expire)
	// handle hash collision issue
	if errors.Is(err, common.ErrCollision) {
		for i := 0; i < s.shardColCnt; i++ {
			err = s.shards[i].Touch(key, h, expire)
			if errors.Is(err, common.ErrCollision) {
				continue
			}
			break
		}
	}
	return err
}

func (s *Store) Get(key []byte) ([]byte, error) {
	h := murmur3.Sum32WithSeed(key, 0)
	v, _, err := s.shards[s.idx(h)].Get(key, h)
	// handle collision issue
	if errors.Is(err, common.ErrCollision) {
		for i := 0; i < s.shardColCnt; i++ {
			v, _, err = s.shards[i].Get(key, h)
			if errors.Is(err, common.ErrCollision) || errors.Is(err, common.ErrKeyNotFound) {
				continue
			}
			break
		}
	}
	return v, err
}

func (s *Store) Delete(key []byte) (bool, error) {
	h := murmur3.Sum32WithSeed(key, 0)
	idx := s.idx(h)
	isDeleted, err := s.shards[idx].Delete(key, h)
	if errors.Is(err, common.ErrCollision) {
		for i := 0; i < s.shardColCnt; i++ {
			isDeleted, err = s.shards[i].Delete(key, h)
			if errors.Is(err, common.ErrCollision) || errors.Is(err, common.ErrKeyNotFound) {
				continue
			}
			if isDeleted {
				err = nil
			}
			break
		}
	}
	return isDeleted, err
}

func (s *Store) Count() int {
	res := 0
	for i := range s.shards {
		res += s.shards[i].Count()
	}
	return res
}

// Close ... close related shards
func (s *Store) Close() error {
	errStr := ""
	if s.syncInterval > 0 {
		s.interv.Clear()
	}
	if s.expireInterval > 0 {
		s.expInterv.Clear()
	}
	for i := range s.shards {
		err := s.shards[i].Close()
		if err != nil {
			errStr += err.Error() + "\r\n"
			return err
		}
	}
	if errStr != "" {
		return errors.New(errStr)
	}
	return nil
}

// FileSize ... total size of the disk storage used by the DB
func (s *Store) FileSize() (int64, error) {
	var res int64
	for i := range s.shards {
		sz, err := s.shards[i].FileSize()
		if err != nil {
			return -1, err
		}
		res += sz
	}
	return res, nil
}

func (s *Store) Incr(k []byte, v uint64) (uint64, error) {
	h := murmur3.Sum32WithSeed(k, 0)
	idx := s.idx(h)
	return s.shards[idx].Counter(k, h, v, true)
}

func (s *Store) Decr(k []byte, v uint64) (uint64, error) {
	h := murmur3.Sum32WithSeed(k, 0)
	idx := s.idx(h)
	return s.shards[idx].Counter(k, h, v, false)
}

func (s *Store) Backup(w io.Writer) error {
	_, err := w.Write([]byte{1})
	if err != nil {
		return err
	}
	for i := range s.shards {
		err = s.shards[i].Backup(w)
		if err != nil {
			return err
		}
	}
	return err
}

func (s *Store) BackupGZ(w io.Writer) error {
	gz := gzip.NewWriter(w)
	defer gz.Close()
	return s.Backup(gz)
}

func (s *Store) Restore(_ io.Reader) error {
	return nil
}

func (s *Store) Expire() error {
	for i := range s.shards {
		err := s.shards[i].ExpireKeys(time.Duration(0))
		if err != nil {
			return err
		}
	}
	return nil
}

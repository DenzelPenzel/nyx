package store

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/DenzelPenzel/nyx/internal/common"
	"github.com/DenzelPenzel/nyx/internal/db/store/shard"
	"github.com/DenzelPenzel/nyx/internal/interval"
	"github.com/DenzelPenzel/nyx/internal/logging"
	"github.com/google/btree"
	"github.com/spaolacci/murmur3"
	"go.uber.org/zap"
)

// DataStore is the main key/value store
type DataStore struct {
	sync.RWMutex
	shards              []shard.DataShard
	totalShards         int
	prefix              string
	shardCollisionCount int
	expireShardSeq      int

	dir            string
	syncInterval   time.Duration
	syncTicker     *interval.IntervalRunner
	expireInterval time.Duration
	expireTicker   *interval.IntervalRunner
	btree          *btree.BTree
}

type StoreOption func(*DataStore) error

// WithDirectory sets the directory where shard files are stored
func WithDirectory(dir string) StoreOption {
	return func(ds *DataStore) error {
		if dir == "" {
			dir = "."
		}
		_, err := os.Stat(dir)
		if err != nil {
			if os.IsNotExist(err) && dir != "." {
				err = os.MkdirAll(dir, os.FileMode(0755))
				if err != nil {
					return err
				}
			} else if err != nil {
				return err
			}
		}
		ds.dir = dir
		return nil
	}
}

// WithShardCollisionCount sets the number of collision shards.
// The default is 4; if set to zero, a collision will result in an error
func WithShardCollisionCount(count int) StoreOption {
	return func(ds *DataStore) error {
		ds.shardCollisionCount = count
		return nil
	}
}

// WithTotalShards sets the total number of shards.
// Ensure that totalShards > shardCollisionCount.
func WithTotalShards(total int) StoreOption {
	return func(ds *DataStore) error {
		ds.totalShards = total
		return nil
	}
}

// WithSyncInterval sets up a background interval to fsync shards.
// If interval > 0, a background task will periodically flush shard data.
func WithSyncInterval(intervalDur time.Duration) StoreOption {
	return func(ds *DataStore) error {
		ds.syncInterval = intervalDur
		if intervalDur > 0 {
			ds.syncTicker = interval.NewIntervalRunner(func(_ time.Time) {
				// Log and continue rather than panic on error.
				for i := range ds.shards {
					if err := ds.shards[i].Sync(); err != nil {
						logging.NoContext().Error("Sync error",
							zap.Int("shard", i),
							zap.Error(err),
						)
					}
				}
			}, intervalDur)
		}
		return nil
	}
}

// WithExpireInterval sets up a background interval to expire keys.
func WithExpireInterval(intervalDur time.Duration) StoreOption {
	return func(ds *DataStore) error {
		ds.expireInterval = intervalDur
		if intervalDur > 0 {
			ds.expireTicker = interval.NewIntervalRunner(func(_ time.Time) {
				err := ds.shards[ds.expireShardSeq].ExpireExpiredKeys(intervalDur)
				if err != nil {
					logging.NoContext().Warn("Expire error",
						zap.Int("shard", ds.expireShardSeq),
						zap.Error(err),
					)
				}
				ds.expireShardSeq = (ds.expireShardSeq + 1) % ds.totalShards
			}, intervalDur)
		}
		return nil
	}
}

func NewDataStore(opts ...StoreOption) (*DataStore, error) {
	ds := &DataStore{
		syncInterval:        0,
		expireInterval:      0,
		shardCollisionCount: 4,
		totalShards:         256,
		btree:               btree.New(32),
	}

	for _, opt := range opts {
		if err := opt(ds); err != nil {
			return nil, err
		}
	}

	if ds.totalShards-ds.shardCollisionCount < 1 {
		return nil, errors.New("totalShards must be greater than shardCollisionCount (min 1)")
	}

	ds.shards = make([]shard.DataShard, ds.totalShards)

	// OpenShard shards concurrently using a worker pool
	numWorkers := 4
	indexCh := make(chan int, ds.totalShards)
	errCh := make(chan error, numWorkers)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case idx, ok := <-indexCh:
					if !ok {
						return
					}
					var filename string
					if ds.prefix != "" {
						filename = fmt.Sprintf("%s/%s-%d", ds.dir, ds.prefix, idx)
					} else {
						filename = fmt.Sprintf("%s/%d", ds.dir, idx)
					}
					if err := ds.shards[idx].OpenShard(filename); err != nil {
						errCh <- err
						cancel() // cancel other workers
						return
					}
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	// Send shard indices to workers
	for i := 0; i < ds.totalShards; i++ {
		indexCh <- i
	}

	close(indexCh)
	wg.Wait()

	select {
	case err := <-errCh:
		return ds, err
	default:
	}

	return ds, nil
}

// shardIndex computes the primary shard index for a given key hash
func (ds *DataStore) shardIndex(hash uint32) uint32 {
	return uint32((int(hash) % (ds.totalShards - ds.shardCollisionCount)) + ds.shardCollisionCount)
}

// Set stores the key and value with an expiration time (in seconds), max packet size 2^19, 512kb (524288)
// packet size = len(key) + len(val) + 8
func (ds *DataStore) Set(key, value []byte, expire uint32) error {
	hash := murmur3.Sum32WithSeed(key, 0)
	err := ds.shards[ds.shardIndex(hash)].Set(key, value, hash, expire)
	// handle collision issue
	if errors.Is(err, common.ErrCollision) {
		for i := 0; i < ds.shardCollisionCount; i++ {
			err = ds.shards[i].Set(key, value, hash, expire)
			if errors.Is(err, common.ErrCollision) {
				continue
			}
			break
		}
	}
	return err
}

// Touch updates the expiration time of a key
func (ds *DataStore) Touch(key []byte, expire uint32) error {
	hash := murmur3.Sum32WithSeed(key, 0)
	err := ds.shards[ds.shardIndex(hash)].Touch(key, hash, expire)
	// handle hash collision issue
	if errors.Is(err, common.ErrCollision) {
		for i := 0; i < ds.shardCollisionCount; i++ {
			err = ds.shards[i].Touch(key, hash, expire)
			if errors.Is(err, common.ErrCollision) {
				continue
			}
			break
		}
	}
	return err
}

// Get retrieves the value for a given key
func (ds *DataStore) Get(key []byte) ([]byte, error) {
	hash := murmur3.Sum32WithSeed(key, 0)
	val, _, err := ds.shards[ds.shardIndex(hash)].Get(key, hash)
	if errors.Is(err, common.ErrCollision) {
		for i := 0; i < ds.shardCollisionCount; i++ {
			val, _, err = ds.shards[i].Get(key, hash)
			if errors.Is(err, common.ErrCollision) || errors.Is(err, common.ErrKeyNotFound) {
				continue
			}
			break
		}
	}
	return val, err
}

// Remove deletes a key from the store
func (ds *DataStore) Remove(key []byte) (bool, error) {
	hash := murmur3.Sum32WithSeed(key, 0)
	idx := ds.shardIndex(hash)
	deleted, err := ds.shards[idx].Delete(key, hash)
	if errors.Is(err, common.ErrCollision) {
		for i := 0; i < ds.shardCollisionCount; i++ {
			deleted, err = ds.shards[i].Delete(key, hash)
			if errors.Is(err, common.ErrCollision) || errors.Is(err, common.ErrKeyNotFound) {
				continue
			}
			if deleted {
				err = nil
			}
			break
		}
	}
	return deleted, err
}

// Count returns the total number of keys stored
func (ds *DataStore) Count() int {
	count := 0
	for i := range ds.shards {
		count += ds.shards[i].Count()
	}
	return count
}

// Close shuts down background tasks and closes all shard files
func (ds *DataStore) Close() error {
	if ds.syncInterval > 0 && ds.syncTicker != nil {
		ds.syncTicker.Stop()
	}
	if ds.expireInterval > 0 && ds.expireTicker != nil {
		ds.expireTicker.Stop()
	}
	for i := range ds.shards {
		if err := ds.shards[i].Close(); err != nil {
			return err
		}
	}
	return nil
}

// FileSize returns the total disk storage size used by the shards.
func (ds *DataStore) FileSize() (int64, error) {
	var total int64
	for i := range ds.shards {
		sz, err := ds.shards[i].FileSize()
		if err != nil {
			return -1, err
		}
		total += sz
	}
	return total, nil
}

func (ds *DataStore) Increment(key []byte, v uint64) (uint64, error) {
	hash := murmur3.Sum32WithSeed(key, 0)
	idx := ds.shardIndex(hash)
	return ds.shards[idx].Counter(key, hash, v, true)
}

func (ds *DataStore) Decrement(key []byte, v uint64) (uint64, error) {
	hash := murmur3.Sum32WithSeed(key, 0)
	idx := ds.shardIndex(hash)
	return ds.shards[idx].Counter(key, hash, v, false)
}

// Backup writes a backup of the store data to the provided writer
func (ds *DataStore) Backup(w io.Writer) error {
	if _, err := w.Write([]byte{1}); err != nil {
		return err
	}
	for i := range ds.shards {
		if err := ds.shards[i].Backup(w); err != nil {
			return err
		}
	}
	return nil
}

// BackupGZ writes a gzipped backup of the store
func (ds *DataStore) BackupGZ(w io.Writer) error {
	gz := gzip.NewWriter(w)
	defer gz.Close()
	return ds.Backup(gz)
}

// Restore restores the store from a backup
// TODO: implement restore logic.
func (ds *DataStore) Restore(r io.Reader) error {
	return nil
}

// Expire triggers an immediate expiration check on all shards
func (ds *DataStore) Expire() error {
	for i := range ds.shards {
		if err := ds.shards[i].ExpireExpiredKeys(0); err != nil {
			return err
		}
	}
	return nil
}

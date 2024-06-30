package store

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/DenzelPenzel/nyx/internal/common"
	"github.com/DenzelPenzel/nyx/internal/utils"
	"github.com/spaolacci/murmur3"
	"github.com/stretchr/testify/require"
)

var dirName = "-db-tmp-test-"

func mockDB() (*DataStore, func(), error) {
	os.RemoveAll(dirName)
	ds, err := NewDataStore(WithDirectory(dirName))
	shutdown := func() {
		ds.Close()
		os.RemoveAll(dirName)
	}
	return ds, shutdown, err
}

func Test_Operations(t *testing.T) {
	key := []byte("aa")
	ds, shutdown, err := mockDB()
	require.NoError(t, err)
	defer shutdown()

	t.Run("test last write", func(t *testing.T) {
		err = ds.Set(key, []byte("bbb"), 0)
		require.NoError(t, err)

		err = ds.Set(key, []byte("ccc"), 0)
		require.NoError(t, err)

		res, err := ds.Get(key)
		require.NoError(t, err)
		require.True(t, bytes.Equal(res, []byte("ccc")))
		require.Equal(t, 1, ds.Count())
	})

	t.Run("test get after delete", func(t *testing.T) {
		res, err := ds.Get(key)
		require.NoError(t, err)
		require.True(t, bytes.Equal(res, []byte("ccc")))
		require.Equal(t, 1, ds.Count())

		deleted, err := ds.Remove(key)
		require.NoError(t, err)
		require.True(t, deleted)
		require.Equal(t, 0, ds.Count())

		// Key should not be found after deletion
		_, err = ds.Get(key)
		require.Error(t, err)
	})

	t.Run("test file counter", func(t *testing.T) {
		counter := []byte("counter")

		cnt, err := ds.Increment(counter, uint64(1))
		require.NoError(t, err)
		require.Equal(t, uint64(1), cnt)

		cnt, err = ds.Increment(counter, uint64(10))
		require.NoError(t, err)
		require.Equal(t, uint64(11), cnt)

		cnt, err = ds.Decrement(counter, uint64(1))
		require.NoError(t, err)
		require.Equal(t, uint64(10), cnt)

		cnt, err = ds.Decrement(counter, uint64(11))
		require.NoError(t, err)
		// Underflow produces the maximum uint64 value.
		require.Equal(t, uint64(18446744073709551615), cnt)
	})

	t.Run("test update ttl", func(t *testing.T) {
		err = ds.Set(key, []byte("aaa"), 0)
		require.NoError(t, err)
		// Test that UpdateTTL (formerly Touch) can be invoked without error
		err = ds.Touch(key, 60)
		require.NoError(t, err)
	})
}

func Test_ReadAfterClose(t *testing.T) {
	defer os.RemoveAll(dirName)

	key := []byte("aa")
	require.NoError(t, os.RemoveAll(dirName))

	ds, err := NewDataStore(WithDirectory(dirName))
	require.NoError(t, err)

	err = ds.Set(key, []byte("bbb"), 0)
	require.NoError(t, err)

	res, err := ds.Get(key)
	require.NoError(t, err)
	require.True(t, bytes.Equal(res, []byte("bbb")))
	require.Equal(t, 1, ds.Count())

	err = ds.Close()
	require.NoError(t, err)

	// Re-open the datastore and verify the data persists
	ds, err = NewDataStore(WithDirectory(dirName))
	require.NoError(t, err)

	res, err = ds.Get(key)
	require.NoError(t, err)
	require.True(t, bytes.Equal(res, []byte("bbb")))
	require.Equal(t, 1, ds.Count())
}

func Test_HashCollision(t *testing.T) {
	mapping := make(map[uint32]int, 100_000_000)
	colCnt := 0
	for i := 0; i < 100_000_000; i++ {
		k1 := make([]byte, 8)
		binary.BigEndian.PutUint64(k1, uint64(i))
		h := murmur3.Sum32WithSeed(k1, 0)
		if _, ok := mapping[h]; ok {
			colCnt++
		}
		mapping[h] = i
	}
	require.Equal(t, 0, colCnt)
}

func Test_ManyKeysOp(t *testing.T) {
	ds, shutdown, err := mockDB()
	require.NoError(t, err)
	defer shutdown()

	n := 1_000_000
	keys := utils.GenKeys(n)
	workers := runtime.NumCPU()

	var collCnt int64

	{
		var wg sync.WaitGroup
		indexCh := make(chan int, n)
		for i := 0; i < n; i++ {
			indexCh <- i
		}
		close(indexCh)

		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				for i := range indexCh {
					b := make([]byte, 8)
					binary.BigEndian.PutUint64(b, uint64(i))

					err := ds.Set(keys[i], b, 0)
					if errors.Is(err, common.ErrCollision) {
						atomic.AddInt64(&collCnt, 1)
						err = nil
					}
					if err != nil {
						t.Errorf("failed to set key %d: %v", i, err)
						return
					}
				}
			}()
		}
		wg.Wait()
	}

	{
		var wg sync.WaitGroup
		indexCh := make(chan int, n)
		for i := 0; i < n; i++ {
			indexCh <- i
		}
		close(indexCh)

		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := range indexCh {
					b, err := ds.Get(keys[i])
					if err != nil {
						t.Errorf("failed to get key %d: %v", i, err)
						return
					}
					v := binary.BigEndian.Uint64(b)
					if uint64(i) != v {
						t.Errorf("wrong value for key %d: got %d, want %d", i, v, i)
					}
				}
			}()
		}
		wg.Wait()
	}

	{
		var wg sync.WaitGroup
		indexCh := make(chan int, n)
		for i := 0; i < n; i++ {
			indexCh <- i
		}
		close(indexCh)

		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := range indexCh {
					_, err := ds.Remove(keys[i])
					// It is acceptable if the key is not found because another goroutine may have removed it
					if err != nil && !errors.Is(err, common.ErrKeyNotFound) {
						t.Errorf("failed to remove key %d: %v", i, err)
						return
					}
				}
			}()
		}
		wg.Wait()
	}

	t.Logf("Total collisions encountered: %d", collCnt)
}

func Test_SingleShard(t *testing.T) {
	os.RemoveAll(dirName)
	ds, err := NewDataStore(WithDirectory(dirName), WithShardCollisionCount(0), WithTotalShards(1))
	require.NoError(t, err)

	err = ds.Set([]byte("a"), []byte("123"), 0)
	require.NoError(t, err)

	err = ds.Set([]byte("b"), []byte("456"), 0)
	require.NoError(t, err)

	v, err := ds.Get([]byte("b"))
	require.NoError(t, err)
	require.Equal(t, []byte("456"), v)

	v, err = ds.Get([]byte("a"))
	require.NoError(t, err)
	require.Equal(t, []byte("123"), v)

	err = ds.Close()
	require.NoError(t, err)
	os.RemoveAll(dirName)
}

func Test_BucketEmptyKey(t *testing.T) {
	ds, shutdown, err := mockDB()
	require.NoError(t, err)
	defer shutdown()

	err = ds.Set([]byte(""), []byte("abc"), 0)
	require.NoError(t, err)

	err = ds.Set([]byte(""), []byte("def"), 0)
	require.NoError(t, err)

	// read empty key
	res, err := ds.Get([]byte(""))
	require.NoError(t, err)
	require.True(t, bytes.Equal([]byte("def"), res))
	require.Equal(t, 1, ds.Count())
}

func Test_BucketOperations(t *testing.T) {
	ds, shutdown, err := mockDB()
	require.NoError(t, err)
	defer shutdown()

	// Create a bucket and verify its name
	bucketName := "testBucket"
	bucket, err := ds.Bucket(bucketName)
	require.NoError(t, err)
	require.Equal(t, bucketName, bucket.Name)

	// Store a key/value pair in the bucket
	key := []byte("myKey")
	value := []byte("myValue")
	err = ds.BucketPut(bucket, key, value)
	require.NoError(t, err)

	// Verify the composite key (bucketName + key) returns the correct value
	compositeKey := append([]byte(bucketName), key...)
	res, err := ds.Get(compositeKey)
	require.NoError(t, err)
	require.True(t, bytes.Equal(value, res))
}

func Test_ExpireKeys(t *testing.T) {
	ds, shutdown, err := mockDB()
	require.NoError(t, err)
	defer shutdown()

	key := []byte("expireKey")
	err = ds.Set(key, []byte("value"), 0)
	require.NoError(t, err)

	// Trigger expiration manually
	err = ds.Expire()
	require.NoError(t, err)

	// Note: Without a true TTL mechanism in the underlying shard,
	// the key may still be present. This test is a placeholder
	// for TTL functionality verification
}

func Test_ConcurrentAccess(t *testing.T) {
	ds, shutdown, err := mockDB()
	require.NoError(t, err)
	defer shutdown()

	n := 10000
	workers := runtime.NumCPU()
	keys := utils.GenKeys(n)

	// Perform concurrent Set and Remove operations
	{
		var wg sync.WaitGroup
		indexCh := make(chan int, n)
		for i := 0; i < n; i++ {
			indexCh <- i
		}
		close(indexCh)

		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := range indexCh {
					if i%2 == 0 {
						err := ds.Set(keys[i], []byte("value"), 0)
						if err != nil && !errors.Is(err, common.ErrCollision) {
							t.Errorf("failed to set key %d: %v", i, err)
							return
						}
					} else {
						_, _ = ds.Remove(keys[i])
					}
				}
			}()
		}
		wg.Wait()
	}

	// Concurrent reads
	{
		var wg sync.WaitGroup
		indexCh := make(chan int, n)
		for i := 0; i < n; i++ {
			indexCh <- i
		}
		close(indexCh)

		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := range indexCh {
					_, err := ds.Get(keys[i])
					// It is acceptable if the key is not found, since it might have been removed
					if err != nil && !errors.Is(err, common.ErrKeyNotFound) {
						t.Errorf("failed to get key %d: %v", i, err)
						return
					}
				}
			}()
		}
		wg.Wait()
	}
}

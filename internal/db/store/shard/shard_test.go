package shard

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/spaolacci/murmur3"
	"github.com/stretchr/testify/require"
)

func createTempShard(t *testing.T) (string, *DataShard, func()) {
	tmpDir, err := ioutil.TempDir("", "data_shard_test")
	require.NoError(t, err)

	filename := tmpDir + "/shard.db"
	ds := &DataShard{}
	err = ds.OpenShard(filename)
	require.NoError(t, err)
	cleanup := func() {
		ds.Close()
		os.RemoveAll(tmpDir)
	}
	return filename, ds, cleanup
}

func TestOpenAndWriteRead(t *testing.T) {
	_, ds, cleanup := createTempShard(t)
	defer cleanup()

	key := []byte("testKey")
	value := []byte("testValue")
	hash := murmur3.Sum32WithSeed(key, 0)

	err := ds.Set(key, value, hash, 0)
	require.NoError(t, err)

	gotVal, header, err := ds.Get(key, hash)
	require.NoError(t, err)
	require.Equal(t, value, gotVal)
	require.NotNil(t, header)
}

func TestUpdateExpiry(t *testing.T) {
	_, ds, cleanup := createTempShard(t)
	defer cleanup()

	key := []byte("expiryKey")
	value := []byte("value")
	hash := murmur3.Sum32WithSeed(key, 0)
	expiry := uint32(time.Now().Add(1 * time.Hour).Unix())
	err := ds.Set(key, value, hash, expiry)
	require.NoError(t, err)

	newExpiry := uint32(time.Now().Add(2 * time.Hour).Unix())
	err = ds.Touch(key, hash, newExpiry)
	require.NoError(t, err)

	_, header, err := ds.Get(key, hash)
	require.NoError(t, err)
	require.Equal(t, newExpiry, header.Expire)
}

func TestDelete(t *testing.T) {
	_, ds, cleanup := createTempShard(t)
	defer cleanup()

	key := []byte("deleteKey")
	value := []byte("deleteValue")
	hash := murmur3.Sum32WithSeed(key, 0)
	err := ds.Set(key, value, hash, 0)
	require.NoError(t, err)

	deleted, err := ds.Delete(key, hash)
	require.NoError(t, err)
	require.True(t, deleted)

	_, _, err = ds.Get(key, hash)
	require.Error(t, err)
}

func TestCounterOperations(t *testing.T) {
	_, ds, cleanup := createTempShard(t)
	defer cleanup()

	key := []byte("counterKey")
	hash := murmur3.Sum32WithSeed(key, 0)
	// Initialize counter (implicitly 0)
	cnt, err := ds.Counter(key, hash, 1, true)
	require.NoError(t, err)
	require.Equal(t, uint64(1), cnt)

	cnt, err = ds.Counter(key, hash, 10, true)
	require.NoError(t, err)
	require.Equal(t, uint64(11), cnt)

	cnt, err = ds.Counter(key, hash, 5, false)
	require.NoError(t, err)
	require.Equal(t, uint64(6), cnt)
}

func TestExpireKeys(t *testing.T) {
	_, ds, cleanup := createTempShard(t)
	defer cleanup()

	key := []byte("expireKey")
	value := []byte("willExpire")
	hash := murmur3.Sum32WithSeed(key, 0)

	// Set expiry in the past.
	expiry := uint32(time.Now().Add(-1 * time.Hour).Unix())
	err := ds.Set(key, value, hash, expiry)
	require.NoError(t, err)

	err = ds.ExpireExpiredKeys(2 * time.Second)
	require.NoError(t, err)

	_, _, err = ds.Get(key, hash)
	require.Error(t, err)
}

func TestBackup(t *testing.T) {
	_, ds, cleanup := createTempShard(t)
	defer cleanup()

	key := []byte("backupKey")
	value := []byte("backupValue")
	hash := murmur3.Sum32WithSeed(key, 0)
	err := ds.Set(key, value, hash, 0)
	require.NoError(t, err)

	var buf bytes.Buffer
	err = ds.Backup(&buf)
	require.NoError(t, err)
	require.True(t, buf.Len() > 0)

	backupFile := "test_backup.dat"
	err = os.WriteFile(backupFile, buf.Bytes(), 0644)
	require.NoError(t, err)
	info, err := os.Stat(backupFile)
	require.NoError(t, err)
	require.True(t, info.Size() > 0)
	os.Remove(backupFile)
}

func TestFileSize(t *testing.T) {
	_, ds, cleanup := createTempShard(t)
	defer cleanup()

	size, err := ds.FileSize()
	require.NoError(t, err)
	require.True(t, size > 0)
}

func TestConcurrentAccess(t *testing.T) {
	_, ds, cleanup := createTempShard(t)
	defer cleanup()

	var wg sync.WaitGroup
	n := 100
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := []byte(fmt.Sprintf("key-%d", i))
			value := []byte(fmt.Sprintf("value-%d", i))
			hash := murmur3.Sum32WithSeed(key, 0)
			err := ds.Set(key, value, hash, 0)
			require.NoError(t, err)
			gotVal, _, err := ds.Get(key, hash)
			require.NoError(t, err)
			require.Equal(t, value, gotVal)
		}(i)
	}
	wg.Wait()
}

func TestSync(t *testing.T) {
	_, ds, cleanup := createTempShard(t)
	defer cleanup()

	// Write a record and then call Sync
	key := []byte("syncKey")
	value := []byte("syncValue")
	hash := murmur3.Sum32WithSeed(key, 0)
	err := ds.Set(key, value, hash, 0)
	require.NoError(t, err)

	err = ds.Sync()
	require.NoError(t, err)
}

func TestBackupStream(t *testing.T) {
	_, ds, cleanup := createTempShard(t)
	defer cleanup()

	key := []byte("streamKey")
	value := []byte("streamValue")
	hash := murmur3.Sum32WithSeed(key, 0)
	err := ds.Set(key, value, hash, 0)
	require.NoError(t, err)

	reader, writer := io.Pipe()
	done := make(chan struct{})

	go func() {
		var total bytes.Buffer
		_, err := io.Copy(&total, reader)
		require.NoError(t, err)
		require.True(t, total.Len() > 0)
		close(done)
	}()

	err = ds.Backup(writer)
	require.NoError(t, err)
	writer.Close()
	<-done
}

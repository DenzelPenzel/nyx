package store

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"runtime"
	"testing"

	"github.com/DenzelPenzel/nyx/internal/common"
	"github.com/DenzelPenzel/nyx/internal/utils"
	"github.com/spaolacci/murmur3"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/lotsa"
)

var dirName = "-db-tmp-test-"

func mockDB() (*Store, func(), error) {
	os.RemoveAll(dirName)
	s, err := Open(Dir(dirName))
	shutdown := func() {
		s.Close()
		os.RemoveAll(dirName)
	}
	return s, shutdown, err
}

func Test_Operations(t *testing.T) {
	key := []byte("aa")
	s, shutdown, err := mockDB()
	require.NoError(t, err)
	defer shutdown()

	t.Run("test last write", func(t *testing.T) {
		err = s.Set(key, []byte("bbb"), 0)
		require.NoError(t, err)

		err = s.Set(key, []byte("ccc"), 0)
		require.NoError(t, err)

		res, err := s.Get(key)
		require.NoError(t, err)
		require.True(t, bytes.Equal(res, []byte("ccc")))
		require.Equal(t, 1, s.Count())
	})

	t.Run("test get after delete", func(t *testing.T) {
		res, err := s.Get(key)
		require.NoError(t, err)
		require.True(t, bytes.Equal(res, []byte("ccc")))
		require.Equal(t, 1, s.Count())

		deleted, err := s.Delete(key)
		require.NoError(t, err)
		require.True(t, deleted)
		require.Equal(t, 0, s.Count())

		// key not found error
		_, err = s.Get(key)
		require.Error(t, err)
	})

	t.Run("test file counter", func(t *testing.T) {
		counter := []byte("counter")

		cnt, err := s.Incr(counter, uint64(1))
		require.NoError(t, err)
		require.Equal(t, 1, int(cnt))

		cnt, err = s.Incr(counter, uint64(10))
		require.NoError(t, err)
		require.Equal(t, 11, int(cnt))

		cnt, err = s.Decr(counter, uint64(1))
		require.NoError(t, err)
		require.Equal(t, 10, int(cnt))

		cnt, err = s.Decr(counter, uint64(11))
		require.NoError(t, err)
		require.Equal(t, uint64(18446744073709551615), cnt)
	})
}

func Test_ReadAfterClose(t *testing.T) {
	defer os.RemoveAll(dirName)

	key := []byte("aa")
	err := os.RemoveAll(dirName)
	require.NoError(t, err)

	s, err := Open(Dir(dirName))
	require.NoError(t, err)

	err = s.Set(key, []byte("bbb"), 0)
	require.NoError(t, err)

	res, err := s.Get(key)
	require.NoError(t, err)
	require.True(t, bytes.Equal(res, []byte("bbb")))
	require.Equal(t, 1, s.Count())

	err = s.Close()
	require.NoError(t, err)

	s, err = Open(Dir(dirName))
	require.NoError(t, err)

	res, err = s.Get(key)
	require.NoError(t, err)
	require.True(t, bytes.Equal(res, []byte("bbb")))
	require.Equal(t, 1, s.Count())
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
	s, shutdown, err := mockDB()
	require.NoError(t, err)
	defer shutdown()

	n := 100_000
	keys := utils.GenKeys(n)

	lotsa.Output = os.Stdout
	lotsa.MemUsage = true
	workers := runtime.NumCPU()

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	collCnt := 0
	lotsa.Ops(n, workers, func(i, _ int) {
		b := make([]byte, 8)
		binary.BigEndian.PutUint64(b, uint64(i))
		err := s.Set(keys[i], b, 0)
		if errors.Is(err, common.ErrCollision) {
			collCnt++
			err = nil
		}
		if err != nil {
			panic(err)
		}
	})

	runtime.ReadMemStats(&ms)

	lotsa.Ops(n, workers, func(i, _ int) {
		b, err := s.Get(keys[i])
		if err != nil {
			panic(err)
		}

		v := binary.BigEndian.Uint64(b)

		if uint64(i) != v {
			panic("wrong value")
		}
	})

	runtime.ReadMemStats(&ms)

	lotsa.Ops(n, workers, func(i, _ int) {
		_, err := s.Delete(keys[i])
		// skip the ErrKeyNotFound issue check
		// In a multithreaded environment, another thread might have already removed this key
		if err != nil && !errors.Is(err, common.ErrKeyNotFound) {
			panic(err)
		}
	})
}

func Test_SingleShard(t *testing.T) {
	os.RemoveAll(dirName)
	s, err := Open(Dir(dirName), ShardsCollision(0), ShardsTotal(1))
	require.NoError(t, err)

	err = s.Set([]byte("a"), []byte("123"), 0)
	require.NoError(t, err)

	err = s.Set([]byte("b"), []byte("456"), 0)
	require.NoError(t, err)

	v, err := s.Get([]byte("b"))
	require.NoError(t, err)
	require.Equal(t, []byte("456"), v)

	v, err = s.Get([]byte("a"))
	require.NoError(t, err)
	require.Equal(t, []byte("123"), v)

	err = s.Close()
	os.RemoveAll(dirName)
	require.NoError(t, err)
}

func Test_BucketEmptyKey(t *testing.T) {
	s, shutdown, err := mockDB()
	require.NoError(t, err)
	defer shutdown()

	err = s.Set([]byte(""), []byte("abc"), 0)
	require.NoError(t, err)

	err = s.Set([]byte(""), []byte("def"), 0)
	require.NoError(t, err)

	// read empty key
	res, err := s.Get([]byte(""))
	require.NoError(t, err)
	require.True(t, bytes.Equal([]byte("def"), res))
	require.Equal(t, 1, s.Count())
}

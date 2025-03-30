package db_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"testing"
	"time"

	"github.com/DenzelPenzel/nyx/config"
	"github.com/DenzelPenzel/nyx/internal/common"
	"github.com/DenzelPenzel/nyx/internal/db"
	"github.com/DenzelPenzel/nyx/internal/utils"
	"github.com/stretchr/testify/require"
)

func openDB() (db.DB, func(), error) {
	dirName := utils.TempDir("db-interation-test-")
	os.RemoveAll(dirName)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cfg := &config.DBConfig{
		Dir:    dirName,
		Backup: "",
	}
	dbNode, err := db.NewDB(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}

	shutdown := func() {
		dbNode.Close()
		os.RemoveAll(dirName)
	}

	return dbNode, shutdown, nil
}

func Test_MultiOp(t *testing.T) {
	// open db conn
	d, shutdown, err := openDB()
	require.NoError(t, err)
	defer shutdown()

	n := 1_000_000
	keys := utils.GenKeys(n)

	for i, key := range keys {
		b := make([]byte, 8)
		binary.BigEndian.PutUint64(b, uint64(i))
		err := d.Set(common.SetRequest{
			Key:     key,
			Data:    b,
			Exptime: 0,
		})
		require.NoError(t, err, "failed to set key %d", i)
	}

	opaques := make([]uint32, len(keys))
	quiet := make([]bool, len(keys))

	resChan, errChan := d.Get(common.GetRequest{
		Keys:    keys,
		Opaques: opaques,
		Quiet:   quiet,
	})

	var resCount, errCount int

	for resChan != nil || errChan != nil {
		select {
		case _, ok := <-resChan:
			if !ok {
				resChan = nil
				continue
			}
			resCount++

		case _, ok := <-errChan:
			if !ok {
				errChan = nil
				continue
			}
			errCount++
			t.Logf("Get error: %v", err)
		}
	}

	require.Equal(t, 0, errCount, "expected no errors during get")
	require.Equal(t, len(keys), resCount, "expected to get all keys")

	for _, key := range keys {
		err := d.Delete(common.DeleteRequest{
			Key: key,
		})
		require.NoError(t, err)
	}

	resChan, errChan = d.Get(common.GetRequest{
		Keys:    keys,
		Opaques: make([]uint32, len(keys)),
		Quiet:   make([]bool, len(keys)),
	})
	var missCount int
	for resChan != nil || errChan != nil {
		select {
		case res, ok := <-resChan:
			if !ok {
				resChan = nil
				continue
			}
			if res.Miss {
				missCount++
			}
		case err, ok := <-errChan:
			if !ok {
				errChan = nil
				continue
			}
			t.Logf("Error on get after deletion: %v", err)
		}
	}
	require.Equal(t, len(keys), missCount, "all keys should be missing after deletion")
}

func Test_Add(t *testing.T) {
	d, shutdown, err := openDB()
	require.NoError(t, err)
	defer shutdown()
	key := []byte("abc")

	_ = d.Delete(common.DeleteRequest{Key: key})
	err = d.Add(common.SetRequest{
		Key:     key,
		Data:    []byte("123qwe"),
		Exptime: 0,
	})
	require.NoError(t, err)

	resChan, errChan := d.Get(common.GetRequest{
		Keys:    [][]byte{key},
		Opaques: []uint32{0},
		Quiet:   []bool{false},
	})
	res := <-resChan
	err = <-errChan
	require.True(t, bytes.Equal(res.Data, []byte("123qwe")))

}

func Test_AddExistingKey(t *testing.T) {
	d, shutdown, err := openDB()
	require.NoError(t, err)
	defer shutdown()
	key := []byte("abc")

	err = d.Add(common.SetRequest{
		Key:     key,
		Data:    []byte("123qwe"),
		Exptime: 0,
	})
	require.NoError(t, err)

	resChan, errChan := d.Get(common.GetRequest{
		Keys:    [][]byte{key},
		Opaques: []uint32{0},
		Quiet:   []bool{false},
	})

	res := <-resChan
	err = <-errChan
	require.NoError(t, err)
	require.True(t, bytes.Equal(res.Data, []byte("123qwe")))
}
func Test_Replace(t *testing.T) {
	d, shutdown, err := openDB()
	require.NoError(t, err)
	defer shutdown()
	key := []byte("abc")

	// replace fails for non-existent key
	err = d.Replace(common.SetRequest{
		Key:     key,
		Data:    []byte("123"),
		Exptime: 0,
	})
	require.Error(t, err)

	err = d.Set(common.SetRequest{
		Key:     key,
		Data:    []byte("123"),
		Exptime: 0,
	})
	require.NoError(t, err)

	err = d.Replace(common.SetRequest{
		Key:     key,
		Data:    []byte("456"),
		Exptime: 0,
	})
	require.NoError(t, err)

	resChan, errChan := d.Get(common.GetRequest{
		Keys:    [][]byte{key},
		Opaques: []uint32{0},
		Quiet:   []bool{false},
	})
	err = <-errChan
	res := <-resChan
	require.NoError(t, err)
	require.True(t, bytes.Equal(res.Data, []byte("456")))
}

func Test_Append(t *testing.T) {
	d, shutdown, err := openDB()
	require.NoError(t, err)
	defer shutdown()
	key := []byte("abc")

	err = d.Append(common.SetRequest{
		Key:     key,
		Data:    []byte("123"),
		Exptime: 0,
	})
	require.Error(t, err)

	err = d.Set(common.SetRequest{
		Key:     key,
		Data:    []byte("123"),
		Exptime: 0,
	})
	require.NoError(t, err)

	err = d.Append(common.SetRequest{
		Key:     key,
		Data:    []byte("456"),
		Exptime: 0,
	})
	require.NoError(t, err)

	resChan, errChan := d.Get(common.GetRequest{
		Keys:    [][]byte{key},
		Opaques: []uint32{0},
		Quiet:   []bool{false},
	})
	var res common.GetResponse
	err = <-errChan
	res = <-resChan
	require.NoError(t, err)
	require.True(t, bytes.Equal(res.Data, []byte("123456")), "append operation failed")

}

func Test_Prepend(t *testing.T) {
	d, shutdown, err := openDB()
	require.NoError(t, err)
	defer shutdown()
	key := []byte("abc")

	err = d.Prepend(common.SetRequest{
		Key:     key,
		Data:    []byte("123"),
		Exptime: 0,
	})
	require.Error(t, err)

	err = d.Set(common.SetRequest{
		Key:     key,
		Data:    []byte("456"),
		Exptime: 0,
	})
	require.NoError(t, err)

	err = d.Prepend(common.SetRequest{
		Key:     key,
		Data:    []byte("123"),
		Exptime: 0,
	})
	require.NoError(t, err)

	resChan, errChan := d.Get(common.GetRequest{
		Keys:    [][]byte{key},
		Opaques: []uint32{0},
		Quiet:   []bool{false},
	})
	err = <-errChan
	res := <-resChan
	require.NoError(t, err)
	require.True(t, bytes.Equal(res.Data, []byte("123456")))
}

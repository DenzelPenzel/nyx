package db_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"testing"

	"github.com/DenzelPenzel/nyx/internal/common"
	"github.com/DenzelPenzel/nyx/internal/config"
	"github.com/DenzelPenzel/nyx/internal/db"
	"github.com/DenzelPenzel/nyx/internal/utils"
	"github.com/stretchr/testify/require"
)

func openDB() (db.DB, func(), error) {
	dirName := utils.TempDir("db-interation-test-")
	os.RemoveAll(dirName)
	ctx := context.Background()
	cfg := &config.Config{
		Environment: common.Local,
		DBConfig: &config.DBConfig{
			DataDir: dirName,
			Backup:  "",
		},
	}
	dbNode, err := db.NewDB(ctx, cfg.DBConfig)
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
	defer shutdown()

	require.NoError(t, err)
	n := 1_000_000
	keys := utils.GenKeys(n)

	// setup `n` keys to the db
	for i, key := range keys {
		b := make([]byte, 8)
		binary.BigEndian.PutUint64(b, uint64(i))
		err := d.Set(common.SetRequest{
			Key:     key,
			Data:    b,
			Exptime: 0,
		})
		require.NoError(t, err)
	}

	opaques := make([]uint32, len(keys))
	quiet := make([]bool, len(keys))

	resChan, errChan := d.Get(common.GetRequest{
		Keys:    keys,
		Opaques: opaques,
		Quiet:   quiet,
	})

	errCount := 0
	resCount := 0

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
		}
	}

	require.Equal(t, 0, errCount)
	require.Equal(t, len(keys), resCount)

	for _, key := range keys {
		err := d.Delete(common.DeleteRequest{
			Key: key,
		})
		require.NoError(t, err)
	}
}

func Test_Add(t *testing.T) {
	// open db conn
	d, shutdown, err := openDB()
	defer shutdown()
	require.NoError(t, err)
	key := []byte("abc")

	t.Run("test successful add operation", func(t *testing.T) {
		err = d.Add(common.SetRequest{
			Key:     key,
			Data:    []byte("123qwe"),
			Exptime: 0,
		})
		require.NoError(t, err)

		resChan, _ := d.Get(common.GetRequest{
			Keys:    [][]byte{key},
			Opaques: []uint32{0},
			Quiet:   []bool{false},
		})
		res := <-resChan
		require.True(t, bytes.Equal(res.Data, []byte("123qwe")))
	})

	t.Run("test key miss, if call add operation for existing key", func(t *testing.T) {
		// add operation for existing key remove this key
		err = d.Add(common.SetRequest{
			Key:     key,
			Data:    []byte("123qwe"),
			Exptime: 0,
		})
		require.Error(t, err)
		// should be key miss here
		resChan, errChan := d.Get(common.GetRequest{
			Keys:    [][]byte{key},
			Opaques: []uint32{0},
			Quiet:   []bool{false},
		})
		err := <-errChan
		res := <-resChan
		require.NoError(t, err)
		require.True(t, res.Miss)
	})
}

func Test_Replace(t *testing.T) {
	// open db conn
	d, shutdown, err := openDB()
	defer shutdown()
	require.NoError(t, err)
	key := []byte("abc")

	t.Run("test failed replace operation for not existing key", func(t *testing.T) {
		err = d.Replace(common.SetRequest{
			Key:     key,
			Data:    []byte("123"),
			Exptime: 0,
		})
		require.Error(t, err)
	})

	t.Run("test successful replace operation", func(t *testing.T) {
		err = d.Set(common.SetRequest{
			Key:     key,
			Data:    []byte("123"),
			Exptime: 0,
		})
		require.NoError(t, err)

		err := d.Replace(common.SetRequest{
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
	})
}

func Test_Append(t *testing.T) {
	// open db conn
	d, shutdown, err := openDB()
	defer shutdown()
	require.NoError(t, err)
	key := []byte("abc")

	t.Run("test failed append operation for not existing key", func(t *testing.T) {
		err = d.Append(common.SetRequest{
			Key:     key,
			Data:    []byte("123"),
			Exptime: 0,
		})
		require.Error(t, err)
	})

	t.Run("test successful append operation", func(t *testing.T) {
		err = d.Set(common.SetRequest{
			Key:     key,
			Data:    []byte("123"),
			Exptime: 0,
		})
		require.NoError(t, err)

		err := d.Append(common.SetRequest{
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
		require.True(t, bytes.Equal(res.Data, []byte("123456")))
	})
}

func Test_Prepend(t *testing.T) {
	// open db conn
	d, shutdown, err := openDB()
	defer shutdown()
	require.NoError(t, err)
	key := []byte("abc")

	t.Run("test failed prepend operation for not existing key", func(t *testing.T) {
		err = d.Prepend(common.SetRequest{
			Key:     key,
			Data:    []byte("123"),
			Exptime: 0,
		})
		require.Error(t, err)
	})

	t.Run("test successful prepend operation", func(t *testing.T) {
		err = d.Set(common.SetRequest{
			Key:     key,
			Data:    []byte("456"),
			Exptime: 0,
		})
		require.NoError(t, err)

		err := d.Prepend(common.SetRequest{
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
	})
}

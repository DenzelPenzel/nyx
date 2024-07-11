package db

import (
	"context"
	"fmt"
	"net"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/DenzelPenzel/nyx/internal/common"
	"github.com/DenzelPenzel/nyx/internal/config"
	"github.com/DenzelPenzel/nyx/internal/db/store"
	"github.com/DenzelPenzel/nyx/internal/logging"
	"go.uber.org/zap"
)

type DB interface {
	Set(cmd common.SetRequest) error
	Add(cmd common.SetRequest) error
	Replace(cmd common.SetRequest) error
	Append(cmd common.SetRequest) error
	Prepend(cmd common.SetRequest) error
	Get(cmd common.GetRequest) (<-chan common.GetResponse, <-chan error)
	GetE(cmd common.GetRequest) (<-chan common.GetEResponse, <-chan error)
	GAT(cmd common.GATRequest) (common.GetResponse, error)
	Delete(cmd common.DeleteRequest) error
	Touch(cmd common.TouchRequest) error
	Close() error
	Count() uint64
	Stats() (response []byte, err error)
	Backup(name string) error
	Restore(name string) error
}

type stats struct {
	sync.RWMutex
}

type db struct {
	slaveAddr string
	getCnt    uint64
	setCnt    uint64
	updateAt  int64
	ctx       context.Context
	stats     *stats
	store     *store.Store
}

func NewDB(ctx context.Context, cfg *config.DBConfig) (DB, error) {
	d := &db{
		ctx:       ctx,
		slaveAddr: cfg.Backup,
	}

	s, err := store.Open(store.Dir(cfg.DataDir), store.ExpireInterval(cfg.ExpireInterval))
	if err != nil {
		return nil, err
	}
	// save the store pointer
	d.store = s

	debug.SetGCPercent(20)
	atomic.StoreUint64(&d.getCnt, 0)
	atomic.StoreUint64(&d.setCnt, 0)
	d.stats = &stats{}

	return d, nil
}

func (c *db) Set(cmd common.SetRequest) error {
	expire := cmd.Exptime
	if cmd.Exptime > 0 {
		expire += uint32(time.Now().Unix())
	}
	// TODO: replication
	return c.store.Set(cmd.Key, cmd.Data, expire)
}

func (c *db) Add(cmd common.SetRequest) error {
	_, err := c.store.Get(cmd.Key)
	if err == nil {
		c.store.Delete(cmd.Key)
		return common.ErrKeyExists
	}

	expire := cmd.Exptime
	if cmd.Exptime > 0 {
		expire += uint32(time.Now().Unix())
	}

	return c.store.Set(cmd.Key, cmd.Data, expire)
}

func (c *db) Replace(cmd common.SetRequest) error {
	_, err := c.store.Get(cmd.Key)
	if err != nil {
		c.store.Delete(cmd.Key)
		return common.ErrKeyNotFound
	}

	expire := cmd.Exptime
	if cmd.Exptime > 0 {
		expire += uint32(time.Now().Unix())
	}

	return c.store.Set(cmd.Key, cmd.Data, expire)
}

func (c *db) Append(cmd common.SetRequest) error {
	data, err := c.store.Get(cmd.Key)
	if err != nil {
		c.store.Delete(cmd.Key)
		return common.ErrKeyNotFound
	}
	data = append(data, cmd.Data...)
	return c.store.Set(cmd.Key, data, cmd.Exptime)
}

func (c *db) Prepend(cmd common.SetRequest) error {
	data, err := c.store.Get(cmd.Key)
	if err != nil {
		c.store.Delete(cmd.Key)
		return common.ErrKeyNotFound
	}
	data = append(cmd.Data, data...)
	return c.store.Set(cmd.Key, data, cmd.Exptime)
}

func (c *db) Get(cmd common.GetRequest) (<-chan common.GetResponse, <-chan error) {
	dataOut := make(chan common.GetResponse, len(cmd.Keys))
	errOut := make(chan error)

	for idx, key := range cmd.Keys {
		data, err := c.store.Get(key)
		if err != nil {
			c.store.Delete(key)
			dataOut <- common.GetResponse{
				Miss:   true,
				Quiet:  cmd.Quiet[idx],
				Opaque: cmd.Opaques[idx],
				Key:    key,
			}
			continue
		}

		dataOut <- common.GetResponse{
			Miss:   false,
			Quiet:  cmd.Quiet[idx],
			Opaque: cmd.Opaques[idx],
			Key:    key,
			Data:   data,
		}
	}

	close(dataOut)
	close(errOut)

	return dataOut, errOut
}

func (c *db) GetE(cmd common.GetRequest) (<-chan common.GetEResponse, <-chan error) {
	dataOut := make(chan common.GetEResponse, len(cmd.Keys))
	errorOut := make(chan error)

	for idx, key := range cmd.Keys {
		data, err := c.store.Get(key)

		if err != nil {
			c.store.Delete(key)
			dataOut <- common.GetEResponse{
				Miss:   true,
				Quiet:  cmd.Quiet[idx],
				Opaque: cmd.Opaques[idx],
				Key:    key,
			}
			continue
		}

		dataOut <- common.GetEResponse{
			Miss:   false,
			Quiet:  cmd.Quiet[idx],
			Opaque: cmd.Opaques[idx],
			// TODO add expire and flags
			Key:  key,
			Data: data,
		}
	}

	close(dataOut)
	close(errorOut)

	return dataOut, errorOut
}

func (c *db) GAT(_ common.GATRequest) (common.GetResponse, error) {
	// TODO implement me
	panic("implement me")
}

func (c *db) Delete(cmd common.DeleteRequest) error {
	logger := logging.WithContext(c.ctx)
	atomic.StoreInt64(&c.updateAt, time.Now().Unix())
	deleted, err := c.store.Delete(cmd.Key)

	if c.slaveAddr != "" && deleted && err == nil {
		slaves := strings.Split(c.slaveAddr, ",")
		for _, slave := range slaves {
			conn, err := net.Dial("tcp", slave)
			if err != nil {
				logger.Error("Error to connect", zap.String("slave", slave), zap.Error(err))
				break
			}
			n, err := fmt.Fprintf(conn, "delete %s noreply\r\n", cmd.Key)
			if err != nil {
				logger.Info("Error slave", zap.Error(err), zap.Int("bytes", n))
			}
			err = conn.Close()
			if err != nil {
				logger.Error("Error to close connection", zap.Error(err))
			}
		}
	}

	return err
}

func (c *db) Touch(cmd common.TouchRequest) error {
	atomic.StoreInt64(&c.updateAt, time.Now().Unix())
	expire := cmd.Exptime
	if expire > 0 {
		expire += uint32(time.Now().Unix())
	}
	return c.store.Touch(cmd.Key, expire)
}

func (c *db) Close() error {
	return c.store.Close()
}

func (c *db) Count() uint64 {
	return uint64(c.store.Count())
}

func (c *db) Stats() ([]byte, error) {
	// TODO implement me
	panic("implement me")
}

func (c *db) Backup(name string) error {
	if c.store != nil {
		file, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_EXCL, os.FileMode(0644))
		if err != nil {
			return err
		}
		defer file.Close()
		return c.store.Backup(file)
	}
	return nil
}

func (c *db) Restore(_ string) error {
	// TODO implement me
	panic("implement me")
}

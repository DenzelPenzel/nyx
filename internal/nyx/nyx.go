package nyx

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/DenzelPenzel/nyx/config"
	"github.com/DenzelPenzel/nyx/internal/common"
	"github.com/DenzelPenzel/nyx/internal/db"
	"github.com/DenzelPenzel/nyx/internal/logging"
	"github.com/DenzelPenzel/nyx/internal/proto"
	"github.com/DenzelPenzel/nyx/internal/utils"
	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
	"go.uber.org/zap"
)

type payload struct {
	Typ payloadType     `json:"typ,omitempty"`
	Sub json.RawMessage `json:"sub,omitempty"`
}

type metadataPayload struct {
	RaftID string            `json:"raft_id,omitempty"`
	Data   map[string]string `json:"data,omitempty"`
}

type fsmSnapshot struct {
	database []byte
	meta     []byte
}

// Persist writes the snapshot to the given sink
func (f *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	err := func() error {
		// Start by writing size of database
		b := new(bytes.Buffer)
		size := uint64(len(f.database))
		err := binary.Write(b, binary.LittleEndian, size)
		if err != nil {
			return err
		}
		if _, err := sink.Write(b.Bytes()); err != nil {
			return err
		}

		if _, err := sink.Write(f.database); err != nil {
			return err
		}

		if _, err := sink.Write(f.meta); err != nil {
			return err
		}

		return sink.Close()
	}()

	if err != nil {
		sinkErr := sink.Cancel()
		if sinkErr != nil {
			return sinkErr
		}
		return err
	}

	return nil

}

func (f *fsmSnapshot) Release() {}

type HandlerConst func() db.DB

type NConst func(ctx context.Context, cfg *config.Config, raftLayer Listener, res proto.Responder) *Nyx

const (
	retainSnapshotCount = 2
)

type DBHandler interface {
	Set(req common.SetRequest) error

	Add(req common.SetRequest) error

	Replace(req common.SetRequest) error

	Append(req common.SetRequest) error

	Prepend(req common.SetRequest) error

	Delete(req common.DeleteRequest) error

	Touch(req common.TouchRequest) error

	Get(req common.GetRequest) error

	GetE(req common.GetRequest) error

	Gat(req common.GATRequest) error

	Noop(req common.NoopRequest) error

	Quit(req common.QuitRequest) error

	Version(req common.VersionRequest) error

	Unknown(req common.Request) error

	Error(req common.Request, reqType common.RequestType, err error)

	// Join connects the node,
	// identified by the provided ID and reachable at the specified address, to the current node
	Join(id, addr string, metadata map[string]string) error

	// Remove node from the cluster
	Remove(addr string) error

	// LeaderID ... Returns the raft leader
	LeaderID() (string, error)

	// Stats ... Returns info about the storage
	Stats() (map[string]interface{}, error)

	// GetMetadata ... Returns metadata for specific node_id and for a given key
	GetMetadata(id, key string) string

	SetResponder(resp proto.Responder)
}

type Nyx struct {
	ctx context.Context

	// raft
	raft          *raft.Raft
	raftLayer     Listener
	raftTransport *raft.NetworkTransport

	// physical store
	boltStore *raftboltdb.BoltStore

	// kvs persistent store
	raftStable raft.StableStore

	// log storage
	raftLog raft.LogStore

	raftDir string
	raftID  string

	mu     sync.RWMutex
	metaMu sync.RWMutex

	meta map[string]map[string]string

	ShutdownOnRemove  bool
	SnapshotThreshold uint64
	SnapshotInterval  time.Duration
	HeartbeatTimeout  time.Duration
	ElectionTimeout   time.Duration
	ApplyTimeout      time.Duration
	OpenTimeout       time.Duration

	logger    *zap.Logger
	db        db.DB
	dbConf    *config.DBConfig
	Responder proto.Responder

	shutdown     bool
	shutdownLock sync.Mutex
}

func (n *Nyx) Remove(addr string) error {
	// TODO implement me
	panic("implement me")
}

func (n *Nyx) Stats() (map[string]interface{}, error) {
	// TODO implement me
	panic("implement me")
}

func New(ctx context.Context, cfg *config.Config, raftLayer Listener, res proto.Responder) *Nyx {
	logger := logging.WithContext(ctx)
	return &Nyx{
		ctx:               ctx,
		logger:            logger,
		dbConf:            cfg.StorageConfig.DBCfg,
		raftLayer:         raftLayer,
		meta:              make(map[string]map[string]string),
		raftID:            cfg.StorageConfig.RaftID,
		raftDir:           cfg.StorageConfig.RaftDir,
		ShutdownOnRemove:  cfg.StorageConfig.RaftShutdownOnRemove,
		SnapshotThreshold: cfg.StorageConfig.RaftSnapThreshold,
		ElectionTimeout:   cfg.StorageConfig.RaftElectionTimeout,
		HeartbeatTimeout:  cfg.StorageConfig.RaftHeartbeatTimeout,
		ApplyTimeout:      cfg.StorageConfig.RaftApplyTimeout,
		OpenTimeout:       cfg.StorageConfig.RaftOpenTimeout,
		Responder:         res,
	}
}

func (n *Nyx) Open(allowSingle bool) error {
	n.logger.Info("Running Raft", zap.String("raftDir", n.raftDir), zap.String("NODE_ID", n.raftID))

	if err := os.MkdirAll(n.raftDir, 0750); err != nil {
		n.logger.Fatal("Failed to create raft dir")
		return err
	}

	// open database
	d, err := db.NewDB(n.ctx, n.dbConf)
	if err != nil {
		n.logger.Fatal("Failed to open the database")
		return err
	}

	// init db instance
	n.db = d

	isNewNode := !utils.PathExists(filepath.Join(n.raftDir, "raft.db"))

	// Create a transport layer
	trans := raft.NewNetworkTransport(
		NewRaftLayer(n.raftLayer),
		3,
		10*time.Second,
		os.Stderr,
	)

	n.raftTransport = trans

	// get raft config for the store
	raftCfg := n.getRaftConfig()
	raftCfg.LocalID = raft.ServerID(n.raftID)

	/*
		Snapshot stores the state to recover and restore data
			Example:
				If EC2 instance failed and an autoscaling group brought up another instance for the Raft server
				Rather than streaming all the data from the Raft leader, the new server would restore
				from the snapshot (which you could store in S3 or a similar storage service)
				and then get the latest changes from the leader
	*/
	snapshots, err := raft.NewFileSnapshotStore(
		n.raftDir,
		retainSnapshotCount,
		os.Stderr,
	)
	if err != nil {
		n.logger.Fatal("Error create file snapshot store", zap.Error(err))
		return err
	}

	// create the log store and stable store
	n.boltStore, err = raftboltdb.NewBoltStore(filepath.Join(n.raftDir, "raft.db"))
	if err != nil {
		n.logger.Fatal("Error create new bolt store", zap.Error(err))
		return err
	}

	n.raftStable = n.boltStore
	n.raftLog, err = raft.NewLogCache(512, n.boltStore)
	if err != nil {
		return err
	}

	// create the Raft instance and bootstrap the cluster
	rf, err := raft.NewRaft(
		raftCfg,
		n,
		n.raftLog,
		n.raftStable,
		snapshots,
		n.raftTransport,
	)
	if err != nil {
		n.logger.Fatal("Error creating raft system", zap.Error(err))
		return err
	}

	if allowSingle && isNewNode {
		n.logger.Debug("Running bootstrapping app...")
		n.logger.Debug("Opening store for node", zap.String("NODE_ID", n.raftID))
		cfg := raft.Configuration{
			Servers: []raft.Server{
				raft.Server{
					ID:      raftCfg.LocalID,
					Address: trans.LocalAddr(),
				},
			},
		}
		rf.BootstrapCluster(cfg)
	} else {
		n.logger.Debug("Bootstrapping app not needed")
	}

	// init raft instance
	n.raft = rf

	return nil
}

func (n *Nyx) SetResponder(resp proto.Responder) {
	n.Responder = resp
}

func (n *Nyx) Shutdown() error {
	n.logger.Info("shutting down server")
	n.shutdownLock.Lock()
	defer n.shutdownLock.Unlock()

	if n.shutdown {
		return nil
	}

	n.shutdown = true

	if err := n.db.Close(); err != nil {
		return err
	}

	if n.raft != nil {
		_ = n.raftTransport.Close()
		_ = n.raftLayer.Close()
		future := n.raft.Shutdown()
		if err := future.Error(); err != nil {
			n.logger.Warn("error shutting down raft", zap.Error(err))
		}
	}

	return nil
}

func (n *Nyx) Apply(l *raft.Log) interface{} {
	var c payload
	if err := json.Unmarshal(l.Data, &c); err != nil {
		panic(fmt.Sprintf("failed to unmarshal raft command: %s", err.Error()))
	}

	switch c.Typ {
	case Set:
		var req common.SetRequest
		if err := json.Unmarshal(c.Sub, &req); err != nil {
			return &common.GenericResponse{Error: err}
		}
		err := n.db.Set(req)
		return &common.GenericResponse{Error: err}

	case Get:
		var req common.GetRequest
		if err := json.Unmarshal(c.Sub, &req); err != nil {
			return &common.GenericResponse{Error: err}
		}
		var resp []common.GetResponse
		var getErr error
		resChan, errChan := n.db.Get(req)

		for resChan != nil || errChan != nil {
			select {
			case res, ok := <-resChan:
				if !ok {
					resChan = nil
					continue
				}
				resp = append(resp, res)

			case resErr, ok := <-errChan:
				if !ok {
					errChan = nil
					continue
				}
				getErr = resErr
			}
		}

		return &common.FsmGetResponse{Err: getErr, Result: resp}

	case Peer:
		var data metadataPayload
		if err := json.Unmarshal(c.Sub, &data); err != nil {
			return &common.GenericResponse{Error: err}
		}
		func() {
			n.metaMu.Lock()
			defer n.metaMu.Unlock()
			if _, ok := n.meta[data.RaftID]; !ok {
				n.meta[data.RaftID] = make(map[string]string)
			}
			for k, v := range data.Data {
				n.meta[data.RaftID][k] = v
			}
		}()
		return &common.GenericResponse{}
	default:
		return &common.GenericResponse{Error: fmt.Errorf("unknown command: %v", c.Typ)}
	}
}

// Snapshot returns a snapshot of the database. The caller must ensure that
// no transaction is taking place during this call. Hashicorp Raft guarantees
// that this function will not be called concurrently with Apply.
func (n *Nyx) Snapshot() (raft.FSMSnapshot, error) {
	fsm := &fsmSnapshot{}
	logger := logging.WithContext(n.ctx)
	var err error
	fsm.database, err = n.Database(false)
	if err != nil {
		logger.Error("error connecting to database snapshot", zap.Error(err))
		return nil, err
	}

	fsm.meta, err = json.Marshal(n.meta)
	if err != nil {
		logger.Error("error encode metadata for snapshot", zap.Error(err))
		return nil, err
	}

	return fsm, nil
}

// Restore restores the node to a previous state
func (n *Nyx) Restore(snapshot io.ReadCloser) error {
	if err := n.db.Close(); err != nil {
		return err
	}

	// get size of the database
	var size uint64
	if err := binary.Read(snapshot, binary.LittleEndian, &size); err != nil {
		return err
	}

	// read in the database file data and restore
	database := make([]byte, size)
	if _, err := io.ReadFull(snapshot, database); err != nil {
		return err
	}

	if err := os.WriteFile(n.dbConf.Dir, database, 0600); err != nil {
		return err
	}

	d, err := db.NewDB(n.ctx, n.dbConf)
	if err != nil {
		return err
	}

	n.db = d

	// Read remaining bytes, and set to cluster meta.
	b, err := io.ReadAll(snapshot)
	if err != nil {
		return err
	}

	n.metaMu.Lock()
	defer n.metaMu.Unlock()

	err = json.Unmarshal(b, &n.meta)
	if err != nil {
		return err
	}

	return nil
}

func (n *Nyx) Database(leader bool) ([]byte, error) {
	if leader && n.raft.State() != raft.Leader {
		return nil, errors.New("leader is missing")
	}

	// Ensure only one snapshot can take place at once, and block all queries
	n.mu.Lock()
	defer n.mu.Unlock()

	f, err := os.CreateTemp("", "nyx-snap-")
	if err != nil {
		return nil, err
	}
	_ = f.Close()
	defer os.Remove(f.Name())

	if err := n.db.Backup(f.Name()); err != nil {
		return nil, err
	}

	return os.ReadFile(f.Name())
}

func (n *Nyx) WaitForLeader(timeout time.Duration) (string, error) {
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	tmr := time.NewTimer(timeout)
	defer tmr.Stop()

	for {
		select {
		case <-tick.C:
			leader, _ := n.LeaderAddr()
			if leader != "" {
				return string(leader), nil
			}
		case <-tmr.C:
			return "", fmt.Errorf("timeout expired")
		}
	}
}

func (n *Nyx) WaitForAppliedIndex(idx uint64, timeout time.Duration) error {
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	tmr := time.NewTicker(timeout)
	defer tmr.Stop()

	for {
		select {
		case <-tick.C:
			if n.raft.AppliedIndex() >= idx {
				return nil
			}
		case <-tmr.C:
			return fmt.Errorf("timeout expired")
		}
	}
}

func (n *Nyx) WaitForApplied(timeout time.Duration) error {
	if timeout == 0 {
		return nil
	}
	n.logger.Info(
		"Waiting for the app of all Raft log entries to be completed on the database",
		zap.String("timeout", strconv.FormatInt(int64(timeout), 10)),
	)
	return n.WaitForAppliedIndex(n.raft.LastIndex(), timeout)
}

func (n *Nyx) LeaderAddr() (raft.ServerAddress, raft.ServerID) {
	return n.raft.LeaderWithID()
}

// Path returns the path to the store's storage directory
func (n *Nyx) Path() string {
	return n.raftDir
}

// ID returns the raft ID
func (n *Nyx) ID() string {
	return n.raftID
}

// Addr returns the address of the store
func (n *Nyx) Addr() string {
	return string(n.raftTransport.LocalAddr())
}

func (n *Nyx) LeaderID() (string, error) {
	addr, serverID := n.LeaderAddr()
	n.logger.Info("Current leader info", zap.String("addr", string(addr)), zap.String("server ID", string(serverID)))

	cfg := n.raft.GetConfiguration()
	if err := cfg.Error(); err != nil {
		return "", err
	}
	raftServers := cfg.Configuration().Servers

	for _, srv := range raftServers {
		if srv.Address == addr {
			return string(srv.ID), nil
		}
	}

	return "", nil
}

// GetMetadata returns metadata for specific node_id and for a given key
func (n *Nyx) GetMetadata(id, key string) string {
	n.metaMu.RLock()
	defer n.metaMu.RUnlock()

	if _, ok := n.meta[id]; !ok {
		return ""
	}
	v, ok := n.meta[id][key]
	if !ok {
		return ""
	}
	return v
}

func (n *Nyx) Join(id, addr string, metadata map[string]string) error {
	n.logger.Info("Received a request to join node", zap.String("addr", addr))

	if n.raft.State() != raft.Leader {
		return ErrNotLeader
	}

	f := n.raft.AddVoter(raft.ServerID(id), raft.ServerAddress(addr), 0, 0)

	if e, ok := f.(raft.Future); ok && e.Error() != nil {
		if errors.Is(e.Error(), raft.ErrNotLeader) {
			return ErrNotLeader
		}
		return e.Error()
	}

	if err := n.setMetadata(id, metadata); err != nil {
		return err
	}

	n.logger.Info("Successfully joined the node", zap.String("addr", addr))

	return nil
}

func (n *Nyx) getRaftConfig() *raft.Config {
	cfg := raft.DefaultConfig()
	cfg.ShutdownOnRemove = n.ShutdownOnRemove
	if n.SnapshotThreshold != 0 {
		cfg.SnapshotThreshold = n.SnapshotThreshold
	}
	if n.SnapshotInterval != 0 {
		cfg.SnapshotInterval = n.SnapshotInterval
	}
	if n.HeartbeatTimeout != 0 {
		cfg.HeartbeatTimeout = n.HeartbeatTimeout
	}
	if n.ElectionTimeout != 0 {
		cfg.ElectionTimeout = n.ElectionTimeout
	}
	return cfg
}

func (n *Nyx) SetMetadata(md map[string]string) error {
	return n.setMetadata(n.raftID, md)
}

func (n *Nyx) setMetadata(id string, md map[string]string) error {
	if func() bool {
		n.metaMu.RLock()
		defer n.metaMu.RUnlock()

		if _, ok := n.meta[id]; ok {
			for k, v := range md {
				if n.meta[id][k] != v {
					return false
				}
			}
			return true
		}
		return false
	}() {
		return nil
	}

	body, err := json.Marshal(metadataPayload{
		RaftID: id,
		Data:   md,
	})
	if err != nil {
		return err
	}
	b, err := json.Marshal(payload{Typ: Peer, Sub: body})
	if err != nil {
		return err
	}

	f := n.raft.Apply(b, n.ApplyTimeout)
	if e, ok := f.(raft.Future); ok && e.Error() != nil {
		if errors.Is(e.Error(), raft.ErrNotLeader) {
			return ErrNotLeader
		}
		err := e.Error()
		if err != nil {
			return err
		}
	}

	return nil
}

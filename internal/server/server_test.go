package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/DenzelPenzel/nyx/config"
	"github.com/DenzelPenzel/nyx/internal/common"
	"github.com/DenzelPenzel/nyx/internal/proto"
	"github.com/DenzelPenzel/nyx/internal/server"
	"github.com/DenzelPenzel/nyx/internal/utils"
	"github.com/stretchr/testify/require"
)

type ioCloserWrp struct {
	closed bool
}

func (i *ioCloserWrp) Close() error {
	i.closed = true
	return nil
}

type testReqParser struct {
	req       common.Request
	reqType   common.RequestType
	startTime int64
	err       error

	called bool
}

type testNyx struct {
	setRes,
	addRes,
	replaceRes,
	appendRes,
	prependRes,
	deleteRes,
	touchRes,
	getRes,
	geteRes,
	gatRes,
	noopRes,
	quitRes,
	versionRes,
	unknownRes error

	callMap map[string]interface{}
}

func (t *testNyx) SetResponder(resp proto.Responder) {
	// TODO implement me
	panic("implement me")
}

func (t *testNyx) Join(id, addr string, metadata map[string]string) error {
	t.callMap["Join"] = nil
	return nil
}

func (t *testNyx) Remove(addr string) error {
	t.callMap["Remove"] = nil
	return nil
}

func (t *testNyx) LeaderID() (string, error) {
	t.callMap["LeaderID"] = nil
	return "", nil
}

func (t *testNyx) Stats() (map[string]interface{}, error) {
	t.callMap["Stats"] = nil
	return nil, nil
}

func (t *testNyx) GetMetadata(id, key string) string {
	t.callMap["GetMetadata"] = nil
	return ""
}

func (t *testNyx) Set(_ common.SetRequest) error {
	t.callMap["Set"] = nil
	return t.setRes
}
func (t *testNyx) Add(_ common.SetRequest) error {
	t.callMap["Add"] = nil
	return t.addRes
}
func (t *testNyx) Replace(_ common.SetRequest) error {
	t.callMap["Replace"] = nil
	return t.replaceRes
}
func (t *testNyx) Append(_ common.SetRequest) error {
	t.callMap["Append"] = nil
	return t.appendRes
}
func (t *testNyx) Prepend(_ common.SetRequest) error {
	t.callMap["Prepend"] = nil
	return t.prependRes
}
func (t *testNyx) Delete(_ common.DeleteRequest) error {
	t.callMap["Remove"] = nil
	return t.deleteRes
}
func (t *testNyx) Touch(_ common.TouchRequest) error {
	t.callMap["Touch"] = nil
	return t.touchRes
}
func (t *testNyx) Get(_ common.GetRequest) error {
	t.callMap["Get"] = nil
	return t.getRes
}
func (t *testNyx) GetE(_ common.GetRequest) error {
	t.callMap["GetE"] = nil
	return t.geteRes
}
func (t *testNyx) Gat(_ common.GATRequest) error {
	t.callMap["Gat"] = nil
	return t.gatRes
}
func (t *testNyx) Noop(_ common.NoopRequest) error {
	t.callMap["Noop"] = nil
	return t.noopRes
}
func (t *testNyx) Quit(_ common.QuitRequest) error {
	t.callMap["Quit"] = nil
	return t.quitRes
}
func (t *testNyx) Version(_ common.VersionRequest) error {
	t.callMap["Version"] = nil
	return t.versionRes
}
func (t *testNyx) Unknown(_ common.Request) error {
	t.callMap["Unknown"] = nil
	return t.unknownRes
}

func (t *testNyx) Error(_ common.Request, _ common.RequestType, _ error) {}

// On first call, returns the values in the testReqParser
// On second call, always returns io.EOF
func (tp *testReqParser) Parse() (common.Request, common.RequestType, int64, error) {
	if tp.called {
		return nil, 0, 0, io.EOF
	}
	tp.called = true
	return tp.req, tp.reqType, tp.startTime, tp.err
}

func Test_Server(t *testing.T) {
	testSuccess := func(t *testing.T, action string, reqType common.RequestType, req common.Request) {
		conn := []io.Closer{&ioCloserWrp{}, &ioCloserWrp{}}
		nyx := &testNyx{callMap: make(map[string]interface{})}
		rp := &testReqParser{
			reqType: reqType,
			req:     req,
		}

		s := server.NewServer(conn, rp, nyx)
		go s.Loop()

		for {
			closed := true

			for _, closer := range conn {
				if !closer.(*ioCloserWrp).closed {
					closed = false
				}
			}

			if closed {
				break
			}

			runtime.Gosched()
		}

		_, ok := nyx.callMap[action]
		require.True(t, ok)
	}

	t.Run("Set", func(t *testing.T) {
		testSuccess(t, "Set", common.RequestSet, common.SetRequest{
			Key:  []byte("001"),
			Data: []byte("abc"),
		})
	})

	t.Run("Replace", func(t *testing.T) {
		testSuccess(t, "Replace", common.RequestReplace, common.SetRequest{
			Key:  []byte("001"),
			Data: []byte("abc"),
		})
	})

	t.Run("Remove", func(t *testing.T) {
		testSuccess(t, "Remove", common.RequestDelete, common.DeleteRequest{
			Key: []byte("key"),
		})
	})

	t.Run("Get", func(t *testing.T) {
		testSuccess(t, "Get", common.RequestGet, common.GetRequest{
			Keys:    [][]byte{[]byte("key")},
			Opaques: []uint32{0},
			Quiet:   []bool{false},
		})
	})

	t.Run("Append", func(t *testing.T) {
		testSuccess(t, "Append", common.RequestAppend, common.SetRequest{
			Key:  []byte("key"),
			Data: []byte("data"),
		})
	})

	t.Run("Add", func(t *testing.T) {
		testSuccess(t, "Add", common.RequestAdd, common.SetRequest{
			Key:  []byte("key"),
			Data: []byte("data"),
		})
	})
}

type MockStorage struct {
}

func (s *MockStorage) SetResponder(resp proto.Responder) {
	//TODO implement me
	panic("implement me")
}

func (s *MockStorage) Add(req common.SetRequest) error {
	return nil
}

func (s *MockStorage) Replace(req common.SetRequest) error {
	return nil
}

func (s *MockStorage) Append(req common.SetRequest) error {
	return nil
}

func (s *MockStorage) Prepend(req common.SetRequest) error {
	return nil
}

func (s *MockStorage) Delete(req common.DeleteRequest) error {
	return nil
}

func (s *MockStorage) Touch(req common.TouchRequest) error {
	return nil
}

func (s *MockStorage) GetE(req common.GetRequest) error {
	return nil
}

func (s *MockStorage) Gat(req common.GATRequest) error {
	return nil
}

func (s *MockStorage) Noop(req common.NoopRequest) error {
	return nil
}

func (s *MockStorage) Quit(req common.QuitRequest) error {
	return nil
}

func (s *MockStorage) Version(req common.VersionRequest) error {
	return nil
}

func (s *MockStorage) Unknown(req common.Request) error {
	return nil
}

func (s *MockStorage) Error(req common.Request, reqType common.RequestType, err error) {
	return
}

func (s *MockStorage) Get(_ common.GetRequest) error {
	return nil
}

func (s *MockStorage) Set(_ common.SetRequest) error {
	return nil
}

func (s *MockStorage) Stats() (map[string]interface{}, error) {
	return nil, nil //nolint:nilnil //Ignore it
}

func (s *MockStorage) LeaderID() (string, error) {
	return "", nil
}

func (s *MockStorage) GetMetadata(_, _ string) string {
	return ""
}

func (s *MockStorage) Join(_, _ string, _ map[string]string) error {
	return nil
}

func (s *MockStorage) Remove(_ string) error {
	return nil
}

func (s *MockStorage) Leader() string {
	return ""
}

func tempDir() string {
	path, err := os.MkdirTemp("", "nyx-test-")
	if err != nil {
		panic("failed to create temp dir")
	}
	return path
}

func createMockConfig() *config.Config {
	dir := tempDir()
	defer os.RemoveAll(dir)

	httpAddr, _ := utils.GetTCPAddr("localhost:4001")
	raftAddr, _ := utils.GetTCPAddr("localhost:4002")
	raftHeartbeatTimeout, _ := time.ParseDuration("1s")
	raftElectionTimeout, _ := time.ParseDuration("1s")
	raftOpenTimeout, _ := time.ParseDuration("120s")
	raftApplyTimeout, _ := time.ParseDuration("10s")

	raftID := utils.RandomString(5)

	return &config.Config{
		Environment: common.Development,
		ServerConfig: &config.ServerConfig{
			HTTPAddr: httpAddr,
		},
		StorageConfig: &config.StorageConfig{
			RaftID:               raftID,
			RaftDir:              filepath.Join(dir, raftID),
			RaftAddr:             raftAddr,
			RaftHeartbeatTimeout: raftHeartbeatTimeout,
			RaftElectionTimeout:  raftElectionTimeout,
			RaftApplyTimeout:     raftApplyTimeout,
			RaftOpenTimeout:      raftOpenTimeout,
			RaftSnapThreshold:    uint64(8192),
			RaftShutdownOnRemove: false,
			DBCfg: &config.DBConfig{
				Dir:    dir,
				Backup: "",
			},
		},
	}
}

func Test_HTTPServer(t *testing.T) {
	t.Run("test create a new server server", func(t *testing.T) {
		m := &MockStorage{}
		cfg := createMockConfig()
		s := server.NewHTTPServer(context.TODO(), cfg.ServerConfig, m)
		defer s.ShutDown()
		err := s.ListenAndServeHTTP()
		require.NoError(t, err)
	})

	t.Run("test set join node", func(t *testing.T) {
		mockSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Fatalf("invalid method name: %s", r.Method)
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer mockSrv.Close()

		addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:9090")
		resp, err := server.Join([]string{mockSrv.URL}, "node1", addr, nil)
		require.NoError(t, err)
		require.Equal(t, resp, mockSrv.URL+"/api/db/join")
	})

	t.Run("test set join node and parse meta", func(t *testing.T) {
		var body map[string]interface{}
		mockSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Fatalf("invalid method name: %s", r.Method)
			}
			w.WriteHeader(http.StatusOK)

			b, err := io.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			if err := json.Unmarshal(b, &body); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		}))
		defer mockSrv.Close()

		addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:9090")
		meta := map[string]string{"test": "test"}

		resp, err := server.Join([]string{mockSrv.URL}, "node1", addr, meta)
		require.NoError(t, err)
		require.Equal(t, mockSrv.URL+"/api/db/join", resp)

		val, ok := body["id"]
		require.True(t, ok)
		require.Equal(t, "node1", val)

		val, ok = body["addr"]
		require.True(t, ok)
		require.Equal(t, addr.String(), val)

		val, ok = body["meta"]
		require.True(t, ok)

		val1, _ := json.Marshal(val)
		val2, _ := json.Marshal(meta)
		require.Equal(t, string(val1), string(val2))
	})

	t.Run("test set join node failed", func(t *testing.T) {
		mockSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer mockSrv.Close()

		addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:9090")
		_, err := server.Join([]string{mockSrv.URL}, "node1", addr, nil)
		require.Error(t, err)
	})

	t.Run("test multi set join first node", func(t *testing.T) {
		mockSrv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer mockSrv1.Close()

		mockSrv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer mockSrv2.Close()

		addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:9090")
		resp, err := server.Join([]string{mockSrv1.URL, mockSrv2.URL}, "node1", addr, nil)
		require.NoError(t, err)
		require.Equal(t, mockSrv1.URL+"/api/db/join", resp)
	})

	t.Run("test multi set join second node", func(t *testing.T) {
		mockSrv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer mockSrv1.Close()

		mockSrv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer mockSrv2.Close()

		addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:9090")
		resp, err := server.Join([]string{mockSrv1.URL, mockSrv2.URL}, "node1", addr, nil)
		require.NoError(t, err)
		require.Equal(t, mockSrv2.URL+"/api/db/join", resp)
	})

	t.Run("test multi set join second node redirect", func(t *testing.T) {
		mockSrv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer mockSrv1.Close()
		redirectAddr := fmt.Sprintf("%s%s", mockSrv1.URL, "/api/db/join")

		mockSrv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, redirectAddr, http.StatusMovedPermanently)
		}))
		defer mockSrv2.Close()

		addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
		resp, err := server.Join([]string{mockSrv2.URL}, "node2", addr, nil)
		require.NoError(t, err)
		require.Equal(t, redirectAddr, resp)
	})

}

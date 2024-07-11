package server_test

import (
	"io"
	"runtime"
	"testing"

	"github.com/DenzelPenzel/nyx/internal/common"
	"github.com/DenzelPenzel/nyx/internal/server"
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
	t.callMap["Delete"] = nil
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

	t.Run("Delete", func(t *testing.T) {
		testSuccess(t, "Delete", common.RequestDelete, common.DeleteRequest{
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

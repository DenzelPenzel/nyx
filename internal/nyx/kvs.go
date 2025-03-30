package nyx

import (
	"encoding/json"
	"errors"

	"github.com/DenzelPenzel/nyx/internal/common"
	"github.com/hashicorp/raft"
)

var (
	// ErrNotLeader is returned when a node attempts to execute a leader-only
	// operation.
	ErrNotLeader = errors.New("not leader")

	// ErrOpenTimeout is returned when the Shrek does not apply its initial
	// logs within the specified time.
	ErrOpenTimeout = errors.New("timeout waiting for initial logs application")

	// ErrInvalidBackupFormat is returned when the requested backup format
	// is not valid.
	ErrInvalidBackupFormat = errors.New("invalid backup format")
)

func (n *Nyx) Set(req common.SetRequest) error {
	if n.raft.State() != raft.Leader {
		return ErrNotLeader
	}

	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	b, err := json.Marshal(&payload{
		Typ: Set,
		Sub: body,
	})
	if err != nil {
		return err
	}

	f := n.raft.Apply(b, n.ApplyTimeout)
	if e, ok := f.(raft.Future); ok && e.Error() != nil {
		if errors.Is(e.Error(), raft.ErrNotLeader) {
			return ErrNotLeader
		}
		return e.Error()
	}

	_, ok := f.Response().(*common.GenericResponse)
	if !ok {
		return errors.New("failed response type")
	}

	return n.Responder.Set(req.Opaque, req.Quiet)
}

func (n *Nyx) Add(req common.SetRequest) error {
	err := n.db.Add(req)
	if err == nil {
		err = n.Responder.Add(req.Opaque, req.Quiet)
	}
	return err
}

func (n *Nyx) Replace(req common.SetRequest) error {
	err := n.db.Replace(req)
	if err == nil {
		err = n.Responder.Replace(req.Opaque, req.Quiet)
	}
	return err
}

func (n *Nyx) Append(req common.SetRequest) error {
	err := n.db.Append(req)
	if err == nil {
		err = n.Responder.Append(req.Opaque, req.Quiet)
	}
	return err
}

func (n *Nyx) Prepend(req common.SetRequest) error {
	err := n.db.Prepend(req)
	if err == nil {
		err = n.Responder.Prepend(req.Opaque, req.Quiet)
	}
	return err
}

func (n *Nyx) Delete(req common.DeleteRequest) error {
	err := n.db.Delete(req)
	if err == nil {
		err = n.Responder.Delete(req.Opaque)
	}
	return err
}

func (n *Nyx) Touch(req common.TouchRequest) error {
	err := n.db.Touch(req)
	if err == nil {
		n.Responder.Touch(req.Opaque)
	}
	return err
}

func (n *Nyx) Get(req common.GetRequest) error {
	n.mu.RLock()
	defer n.mu.RUnlock()

	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	b, err := json.Marshal(&payload{
		Typ: Get,
		Sub: body,
	})
	if err != nil {
		return err
	}
	f := n.raft.Apply(b, n.ApplyTimeout)
	if e, ok := f.(raft.Future); ok && e.Error() != nil {
		if errors.Is(e.Error(), raft.ErrNotLeader) {
			return ErrNotLeader
		}
		return e.Error()
	}

	res, ok := f.Response().(*common.FsmGetResponse)
	if !ok {
		return errors.New("failed response type")
	}

	for _, x := range res.Result {
		n.Responder.Get(x)
	}

	// Call GetEnd if there was no error
	if res.Err == nil {
		n.Responder.GetEnd(req.NoopOpaque, req.NoopEnd)
	}

	return res.Err
}

func (n *Nyx) GetE(req common.GetRequest) error {
	resChan, errChan := n.db.GetE(req)
	var err error

	for resChan != nil || errChan != nil {
		select {
		case res, ok := <-resChan:
			if !ok {
				resChan = nil
				continue
			}
			n.Responder.GetE(res)

		case resErr, ok := <-errChan:
			if !ok {
				errChan = nil
				continue
			}
			err = resErr
		}
	}

	if err == nil {
		n.Responder.GetEnd(req.NoopOpaque, req.NoopEnd)
	}

	return err
}

func (n *Nyx) Gat(req common.GATRequest) error {
	res, err := n.db.GAT(req)
	if err == nil {
		n.Responder.GAT(res)
	}
	return err
}

func (n *Nyx) Noop(req common.NoopRequest) error {
	return n.Responder.Noop(req.Opaque)
}

func (n *Nyx) Quit(req common.QuitRequest) error {
	return n.Responder.Quit(req.Opaque, req.Quiet)
}

func (n *Nyx) Version(req common.VersionRequest) error {
	return n.Responder.Version(req.Opaque)
}

func (n *Nyx) Unknown(_ common.Request) error {
	return common.ErrUnknownCmd
}

func (n *Nyx) Error(req common.Request, reqType common.RequestType, err error) {
	var opaque uint32
	var quiet bool

	if req != nil {
		opaque = req.GetOpaque()
		quiet = req.IsQuiet()
	}

	n.Responder.Error(opaque, reqType, err, quiet)
}

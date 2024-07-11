package nyx

import (
	"github.com/DenzelPenzel/nyx/internal/common"
	"github.com/DenzelPenzel/nyx/internal/db"
	"github.com/DenzelPenzel/nyx/internal/proto"
)

type HandlerConst func() db.DB

type NConst func(d db.DB, res proto.Responder) common.DBHandler

type Nyx struct {
	db  db.DB
	res proto.Responder
}

func NewNyx(d db.DB, res proto.Responder) common.DBHandler {
	return &Nyx{
		db:  d,
		res: res,
	}
}

func (n *Nyx) Set(req common.SetRequest) error {
	err := n.db.Set(req)
	if err == nil {
		err = n.res.Set(req.Opaque, req.Quiet)
	}
	return err
}

func (n *Nyx) Add(req common.SetRequest) error {
	err := n.db.Add(req)
	if err == nil {
		err = n.res.Add(req.Opaque, req.Quiet)
	}
	return err
}

func (n *Nyx) Replace(req common.SetRequest) error {
	err := n.db.Replace(req)
	if err == nil {
		err = n.res.Replace(req.Opaque, req.Quiet)
	}
	return err
}

func (n *Nyx) Append(req common.SetRequest) error {
	err := n.db.Append(req)
	if err == nil {
		err = n.res.Append(req.Opaque, req.Quiet)
	}
	return err
}

func (n *Nyx) Prepend(req common.SetRequest) error {
	err := n.db.Prepend(req)
	if err == nil {
		err = n.res.Prepend(req.Opaque, req.Quiet)
	}
	return err
}

func (n *Nyx) Delete(req common.DeleteRequest) error {
	err := n.db.Delete(req)
	if err == nil {
		err = n.res.Delete(req.Opaque)
	}
	return err
}

func (n *Nyx) Touch(req common.TouchRequest) error {
	err := n.db.Touch(req)
	if err == nil {
		n.res.Touch(req.Opaque)
	}
	return err
}

func (n *Nyx) Get(req common.GetRequest) error {
	resChan, errChan := n.db.Get(req)
	var err error

	for resChan != nil || errChan != nil {
		select {
		case res, ok := <-resChan:
			if !ok {
				resChan = nil
				continue
			}
			n.res.Get(res)

		case resErr, ok := <-errChan:
			if !ok {
				errChan = nil
				continue
			}
			err = resErr
		}
	}

	// Call GetEnd if there was no error
	if err == nil {
		n.res.GetEnd(req.NoopOpaque, req.NoopEnd)
	}

	return err
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
			n.res.GetE(res)

		case resErr, ok := <-errChan:
			if !ok {
				errChan = nil
				continue
			}
			err = resErr
		}
	}

	if err == nil {
		n.res.GetEnd(req.NoopOpaque, req.NoopEnd)
	}

	return err
}

func (n *Nyx) Gat(req common.GATRequest) error {
	res, err := n.db.GAT(req)
	if err == nil {
		n.res.GAT(res)
	}
	return err
}

func (n *Nyx) Noop(req common.NoopRequest) error {
	return n.res.Noop(req.Opaque)
}

func (n *Nyx) Quit(req common.QuitRequest) error {
	return n.res.Quit(req.Opaque, req.Quiet)
}

func (n *Nyx) Version(req common.VersionRequest) error {
	return n.res.Version(req.Opaque)
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

	n.res.Error(opaque, reqType, err, quiet)
}

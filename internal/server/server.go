package server

import (
	"fmt"
	"github.com/denzelpenzel/nyx/internal/common"
	"github.com/denzelpenzel/nyx/internal/logging"
	"github.com/denzelpenzel/nyx/internal/nyx"
	"github.com/denzelpenzel/nyx/internal/proto"
	"github.com/denzelpenzel/nyx/internal/utils"
	"go.uber.org/zap"
	"io"
)

type SrvConst func(conns []io.Closer, rp proto.RequestParser, n nyx.NyxDB) Server

type Server interface {
	Loop()
}

type DefaultServer struct {
	rp    proto.RequestParser
	n     nyx.NyxDB
	conns []io.Closer
}

func NewServer(conns []io.Closer, rp proto.RequestParser, n nyx.NyxDB) Server {
	return &DefaultServer{
		n:     n,
		rp:    rp,
		conns: conns,
	}
}

func (s *DefaultServer) Loop() {
	logger := logging.NoContext()

	defer func() {
		if r := recover(); r != nil {
			if r != io.EOF {
				logger.Fatal("recover from runtime panic",
					zap.Any("recover", r),
					zap.String("path", utils.IdentifyPanic()),
				)
			}
			shutdown(s.conns, fmt.Errorf("runtime panic: %v", r))
		}
	}()

	for {
		request, reqType, _, err := s.rp.Parse()
		if err != nil {
			if common.IsWrongRequest(err) {
				s.n.Error(nil, common.RequestUnknown, err)
				continue
			}
			shutdown(s.conns, err)
			return
		}

		logger.Info("Received the new request",
			zap.Any("req type", reqType),
			zap.Any("body", request),
		)

		switch reqType {
		case common.RequestGet:
			err = s.n.Get(request.(common.GetRequest))

		case common.RequestSet:
			err = s.n.Set(request.(common.SetRequest))

		case common.RequestReplace:
			err = s.n.Replace(request.(common.SetRequest))

		case common.RequestDelete:
			err = s.n.Delete(request.(common.DeleteRequest))

		case common.RequestAppend:
			err = s.n.Append(request.(common.SetRequest))

		case common.RequestAdd:
			err = s.n.Add(request.(common.SetRequest))

		default:
			s.n.Error(nil, common.RequestUnknown, fmt.Errorf("invalid req type"))
		}

		if err != nil {
			if common.IsAppError(err) {
				s.n.Error(request, reqType, err)
			} else {
				shutdown(s.conns, err)
				return
			}
		}
	}
}

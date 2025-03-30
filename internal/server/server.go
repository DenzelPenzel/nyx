package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/DenzelPenzel/nyx/internal/common"
	"github.com/DenzelPenzel/nyx/internal/logging"
	"github.com/DenzelPenzel/nyx/internal/nyx"
	"github.com/DenzelPenzel/nyx/internal/proto"
	"github.com/DenzelPenzel/nyx/internal/utils"
	"go.uber.org/zap"
)

const numAttempts = 3

type SrvConst func(conns []io.Closer, rp proto.RequestParser, n nyx.DBHandler) Server

type Server interface {
	Loop()
}

type DefaultServer struct {
	rp      proto.RequestParser
	storage nyx.DBHandler
	conns   []io.Closer
}

func NewServer(conns []io.Closer, rp proto.RequestParser, storage nyx.DBHandler) Server {
	return &DefaultServer{
		storage: storage,
		rp:      rp,
		conns:   conns,
	}
}

func (s *DefaultServer) Loop() {
	logger := logging.NoContext()

	defer func() {
		if r := recover(); r != nil {
			if err, ok := r.(error); ok && !errors.Is(err, io.EOF) {
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
				s.storage.Error(nil, common.RequestUnknown, err)
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
			err = s.storage.Get(request.(common.GetRequest))

		case common.RequestSet:
			err = s.storage.Set(request.(common.SetRequest))

		case common.RequestReplace:
			err = s.storage.Replace(request.(common.SetRequest))

		case common.RequestDelete:
			err = s.storage.Delete(request.(common.DeleteRequest))

		case common.RequestAppend:
			err = s.storage.Append(request.(common.SetRequest))

		case common.RequestAdd:
			err = s.storage.Add(request.(common.SetRequest))

		default:
			s.storage.Error(nil, common.RequestUnknown, fmt.Errorf("invalid req type"))
		}

		if err != nil {
			if common.IsAppError(err) {
				s.storage.Error(request, reqType, err)
			} else {
				shutdown(s.conns, err)
				return
			}
		}
	}
}

func Join(joinAddrs []string, id string, addr *net.TCPAddr, meta map[string]string) (string, error) {
	var err error
	var fullURL string
	logger := logging.NoContext()
	joinAddr := make([]*url.URL, 0)

	for _, ja := range joinAddrs {
		u, err := url.Parse(fmt.Sprintf("%s/api/db/join", ja))
		if err != nil {
			return "", err
		}
		joinAddr = append(joinAddr, u)
	}

	for i := 0; i < numAttempts; i++ {
		for _, joinAddr := range joinAddr {
			fullURL, err = join(joinAddr, id, addr, meta)
			if err == nil {
				return fullURL, nil
			}
		}
		time.Sleep(2 * time.Second)
	}

	logger.Error("failed to join raft cluster",
		zap.String("attempts", strconv.Itoa(numAttempts)),
		zap.Error(err),
	)

	return "", err
}

func join(joinAddr *url.URL, id string, addr *net.TCPAddr, meta map[string]string) (string, error) {
	logger := logging.NoContext()
	if id == "" {
		return "", fmt.Errorf("node ID is empty")
	}

	tr := &http.Transport{}
	client := &http.Client{Transport: tr}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	for {
		b, err := json.Marshal(map[string]interface{}{
			"id":   id,
			"addr": addr.String(),
			"meta": meta,
		})
		if err != nil {
			return "", err
		}

		resp, err := client.Post( //nolint:noctx // no need ctx
			joinAddr.String(),
			"application-type/json",
			bytes.NewReader(b),
		)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		logger.Debug("Join request, method: POST", zap.String("url", joinAddr.String()))

		b, err = io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}

		switch resp.StatusCode {
		case http.StatusOK:
			return joinAddr.String(), nil

		case http.StatusMovedPermanently:
			redirectURL := resp.Header.Get("location")
			if redirectURL == "" {
				return "", fmt.Errorf("failed to join, invalid redirect received")
			}
			joinAddr, err = url.Parse(redirectURL)
			if err != nil {
				return "", fmt.Errorf("failed to join, invalid redirect received")
			}
			continue
		case http.StatusBadRequest:
			if joinAddr.Scheme == "https" {
				return "", fmt.Errorf("failed to join, node returned: %s: (%s)", resp.Status, string(b))
			}
			logger.Info("join via HTTP failed, trying via HTTPS")
			joinAddr.Scheme = "https"
			continue
		default:
			return "", fmt.Errorf("failed to join, node returned: %s: (%s)", resp.Status, string(b))
		}
	}
}

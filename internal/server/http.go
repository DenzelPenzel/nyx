package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/DenzelPenzel/nyx/config"
	"github.com/DenzelPenzel/nyx/internal/logging"
	"github.com/DenzelPenzel/nyx/internal/nyx"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/contrib/cors"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type HTTPServer struct {
	start   time.Time
	addr    net.Addr
	ln      net.Listener
	cfg     *config.ServerConfig
	storage nyx.DBHandler
	srv     *http.Server
	logger  *zap.Logger
}

func NewHTTPServer(ctx context.Context, cfg *config.ServerConfig, storage nyx.DBHandler) *HTTPServer {
	logger := logging.WithContext(ctx)
	return &HTTPServer{
		cfg:     cfg,
		addr:    cfg.HTTPAddr,
		logger:  logger,
		storage: storage,
		start:   time.Now(),
	}
}

func (s *HTTPServer) ListenAndServeHTTP() error {
	router := gin.Default()

	router.Use(ginzap.Ginzap(s.logger, time.RFC3339, true))
	router.Use(ginzap.RecoveryWithZap(s.logger, true))

	if s.cfg.AllowedOrigins != nil && s.cfg.AllowedMethods != nil {
		allowAllOrigins := len(s.cfg.AllowedOrigins) == 1 && s.cfg.AllowedOrigins[0] == "*"
		allowedOrigins := s.cfg.AllowedOrigins
		if allowAllOrigins {
			allowedOrigins = nil
		}
		router.Use(cors.New(cors.Config{
			AllowAllOrigins: allowAllOrigins,
			AllowedOrigins:  allowedOrigins,
			AllowedMethods:  s.cfg.AllowedMethods,
			AllowedHeaders:  s.cfg.AllowedHeaders,
		}))
	}

	router.POST("/api/db/join", s.handleJoin())

	srv := &http.Server{
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ln, err := net.Listen("tcp", s.addr.String())
	if err != nil {
		return err
	}

	s.ln = ln
	s.srv = srv

	go func() {
		err := srv.Serve(s.ln)
		if err != nil {
			s.logger.Info("failed to execute HTTP Server() call", zap.Error(err))
		}
	}()

	s.logger.Info("service running on", zap.String("addr", s.addr.String()))

	return nil
}

func (s *HTTPServer) ShutDown() {
	_ = s.ln.Close()
	_ = s.srv.Close()
}

func (s *HTTPServer) Addr() net.Addr {
	return s.ln.Addr()
}

func (s *HTTPServer) handleJoin() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req map[string]interface{}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("Bad request: %v", err),
			})
		}
		id, ok := req["id"]
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": errors.New("missing id param"),
			})
			return
		}

		var meta map[string]string
		if _, ok := req["meta"].(map[string]interface{}); ok {
			meta = make(map[string]string)
			for key, value := range req["meta"].(map[string]interface{}) {
				if stringValue, ok := value.(string); ok {
					meta[key] = stringValue
				}
			}
		}

		addr, ok := req["addr"]
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": errors.New("missing addr param"),
			})
			return
		}

		if err := s.storage.Join(id.(string), addr.(string), meta); err != nil {
			if errors.Is(err, nyx.ErrNotLeader) {
				leader := s.leaderAPIAddr()
				if leader == "" {
					c.JSON(http.StatusServiceUnavailable, gin.H{
						"error": err.Error(),
					})
					return
				}
			}
			b := bytes.NewBufferString(err.Error())
			c.JSON(http.StatusInternalServerError, b)
		}
	}
}

func (s *HTTPServer) leaderAPIAddr() string {
	id, err := s.storage.LeaderID()
	if err != nil {
		return ""
	}
	return s.storage.GetMetadata(id, "api_addr")
}

func (s *HTTPServer) leaderID() string {
	id, err := s.storage.LeaderID()
	if err != nil {
		return ""
	}
	return id
}

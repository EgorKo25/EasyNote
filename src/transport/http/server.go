package apphttp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/EgorKo25/EasyNote/src/config"
)

type Server struct {
	cfg    config.HTTP
	server *http.Server
}

func NewServer(cfg config.HTTP, handler http.Handler) *Server {
	address := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))

	return &Server{
		cfg: cfg,
		server: &http.Server{
			Addr:         address,
			Handler:      handler,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			IdleTimeout:  cfg.IdleTimeout,
		},
	}
}

func (s *Server) Info() string {
	return fmt.Sprintf("run http server on %s", s.server.Addr)
}

func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.server.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}

		return fmt.Errorf("run http server: %w", err)

	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.cfg.ShutdownTimeout)
		defer cancel()

		if err := s.server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown http server: %w", err)
		}

		err := <-errCh
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("run http server: %w", err)
		}

		return nil
	}
}

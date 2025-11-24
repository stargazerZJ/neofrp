package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"neofrp/internal/config"

	"github.com/charmbracelet/log"
	"github.com/gorilla/websocket"
)

type Server struct {
	cfg        *config.ServerConfig
	upgrader   websocket.Upgrader
	pools      map[int]*ConnectionPool
	mu         sync.RWMutex
	httpServer *http.Server
	shutdown   chan struct{}
}

type ConnectionPool struct {
	conns   chan *websocket.Conn
	port    int
	authKey string
}

func NewServer(cfg *config.ServerConfig) *Server {
	return &Server{
		cfg: cfg,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		pools:    make(map[int]*ConnectionPool),
		shutdown: make(chan struct{}),
	}
}

func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.BindAddr, s.cfg.BindPort)
	log.Info("Starting FRP server", "addr", addr)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWebSocket)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/shutdown", s.handleShutdown)

	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Start server in a goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Wait for shutdown signal or error
	select {
	case err := <-errChan:
		return err
	case <-s.shutdown:
		log.Info("Shutdown signal received, stopping server...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.httpServer.Shutdown(ctx)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (s *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	log.Info("Shutdown requested via HTTP endpoint")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Shutting down..."))
	
	// Trigger shutdown in a goroutine to allow response to be sent
	go func() {
		time.Sleep(100 * time.Millisecond)
		close(s.shutdown)
		// Give the server time to shutdown gracefully, then force exit
		time.Sleep(6 * time.Second)
		log.Warn("Forcing exit after shutdown timeout")
		os.Exit(0)
	}()
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Check Authorization header
	authHeader := r.Header.Get("Authorization")
	expectedAuth := fmt.Sprintf("Bearer %s", s.cfg.AuthKey)
	if authHeader != expectedAuth {
		log.Warn("Unauthorized connection attempt", "remote", r.RemoteAddr)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get remote port from query parameter
	remotePortStr := r.URL.Query().Get("remote_port")
	if remotePortStr == "" {
		http.Error(w, "remote_port parameter required", http.StatusBadRequest)
		return
	}

	var remotePort int
	fmt.Sscanf(remotePortStr, "%d", &remotePort)

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error("Failed to upgrade connection", "error", err)
		return
	}

	log.Info("New WebSocket connection", "remote", r.RemoteAddr, "port", remotePort)

	// Get or create connection pool for this port
	pool := s.getOrCreatePool(remotePort)
	
	// Add connection to pool
	select {
	case pool.conns <- conn:
		log.Debug("Connection added to pool", "port", remotePort)
	default:
		log.Warn("Connection pool full, closing connection", "port", remotePort)
		conn.Close()
	}
}

func (s *Server) getOrCreatePool(port int) *ConnectionPool {
	s.mu.Lock()
	defer s.mu.Unlock()

	pool, exists := s.pools[port]
	if !exists {
		pool = &ConnectionPool{
			conns:   make(chan *websocket.Conn, 10),
			port:    port,
			authKey: s.cfg.AuthKey,
		}
		s.pools[port] = pool

		// Start TCP listener for this port
		go s.startTCPListener(pool)
	}

	return pool
}

func (s *Server) startTCPListener(pool *ConnectionPool) {
	addr := fmt.Sprintf("0.0.0.0:%d", pool.port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Error("Failed to start TCP listener", "port", pool.port, "error", err)
		return
	}
	defer listener.Close()

	log.Info("TCP listener started", "port", pool.port)

	for {
		tcpConn, err := listener.Accept()
		if err != nil {
			log.Error("Failed to accept TCP connection", "error", err)
			continue
		}

		log.Debug("New TCP connection", "remote", tcpConn.RemoteAddr(), "port", pool.port)

		// Get a WebSocket connection from the pool
		select {
		case wsConn := <-pool.conns:
			go s.handleTCPConnection(tcpConn, wsConn)
		case <-time.After(5 * time.Second):
			log.Warn("No WebSocket connection available in pool", "port", pool.port)
			tcpConn.Close()
		}
	}
}

func (s *Server) handleTCPConnection(tcpConn net.Conn, wsConn *websocket.Conn) {
	defer tcpConn.Close()
	defer wsConn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(2)

	// TCP -> WebSocket
	go func() {
		defer wg.Done()
		defer cancel()

		buf := make([]byte, 32*1024)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			tcpConn.SetReadDeadline(time.Now().Add(30 * time.Second))
			n, err := tcpConn.Read(buf)
			if err != nil {
				if err != io.EOF {
					log.Debug("TCP read error", "error", err)
				}
				return
			}

			if err := wsConn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
				log.Debug("WebSocket write error", "error", err)
				return
			}
		}
	}()

	// WebSocket -> TCP
	go func() {
		defer wg.Done()
		defer cancel()

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			wsConn.SetReadDeadline(time.Now().Add(30 * time.Second))
			_, data, err := wsConn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Debug("WebSocket read error", "error", err)
				}
				return
			}

			if _, err := tcpConn.Write(data); err != nil {
				log.Debug("TCP write error", "error", err)
				return
			}
		}
	}()

	wg.Wait()
	log.Debug("Connection closed")
}
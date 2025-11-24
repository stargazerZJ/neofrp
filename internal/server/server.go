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
)

type Server struct {
	cfg        *config.ServerConfig
	pools      map[int]*ConnectionPool
	tunnels    map[string]*Tunnel
	tunnelsMu  sync.RWMutex
	mu         sync.RWMutex
	httpServer *http.Server
	shutdown   chan struct{}
}

type ConnectionPool struct {
	tunnels chan *Tunnel
	port    int
	authKey string
}

type Tunnel struct {
	id           string
	uploadChan   chan []byte
	downloadChan chan []byte
	ctx          context.Context
	cancel       context.CancelFunc
}

func NewServer(cfg *config.ServerConfig) *Server {
	return &Server{
		cfg:      cfg,
		pools:    make(map[int]*ConnectionPool),
		tunnels:  make(map[string]*Tunnel),
		shutdown: make(chan struct{}),
	}
}

func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.BindAddr, s.cfg.BindPort)
	log.Info("Starting FRP server", "addr", addr)

	mux := http.NewServeMux()
	mux.HandleFunc("/tunnel/download", s.handleDownload)
	mux.HandleFunc("/tunnel/upload", s.handleUpload)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/shutdown", s.handleShutdown)
	
	log.Info("HTTP endpoints registered",
		"download", "/tunnel/download",
		"upload", "/tunnel/upload",
		"health", "/health",
		"shutdown", "/shutdown")

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

// handleDownload streams data from TCP connection to client via HTTP GET
func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get parameters
	remotePortStr := r.URL.Query().Get("remote_port")
	tunnelID := r.URL.Query().Get("tunnel_id")
	
	if remotePortStr == "" || tunnelID == "" {
		http.Error(w, "remote_port and tunnel_id parameters required", http.StatusBadRequest)
		return
	}

	var remotePort int
	fmt.Sscanf(remotePortStr, "%d", &remotePort)

	log.Info("New download stream", "remote", r.RemoteAddr, "port", remotePort, "tunnel_id", tunnelID)

	// Get or create connection pool for this port
	pool := s.getOrCreatePool(remotePort)
	
	// Create tunnel
	ctx, cancel := context.WithCancel(r.Context())
	tunnel := &Tunnel{
		id:           tunnelID,
		uploadChan:   make(chan []byte, 100),
		downloadChan: make(chan []byte, 100),
		ctx:          ctx,
		cancel:       cancel,
	}
	
	// Register tunnel globally
	s.tunnelsMu.Lock()
	s.tunnels[tunnelID] = tunnel
	s.tunnelsMu.Unlock()
	
	// Clean up tunnel on exit
	defer func() {
		s.tunnelsMu.Lock()
		delete(s.tunnels, tunnelID)
		s.tunnelsMu.Unlock()
	}()
	
	// Add tunnel to pool
	select {
	case pool.tunnels <- tunnel:
		log.Debug("Tunnel added to pool", "port", remotePort, "tunnel_id", tunnelID)
	default:
		log.Warn("Tunnel pool full, rejecting connection", "port", remotePort)
		http.Error(w, "Pool full", http.StatusServiceUnavailable)
		cancel()
		return
	}

	// Set headers for streaming
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Error("Streaming not supported")
		cancel()
		return
	}

	// Stream data from upload channel to HTTP response
	for {
		select {
		case <-ctx.Done():
			log.Debug("Download stream closed", "tunnel_id", tunnelID)
			return
		case data := <-tunnel.uploadChan:
			if len(data) == 0 {
				continue
			}
			
			_, err := w.Write(data)
			if err != nil {
				log.Debug("Failed to write to download stream", "error", err, "tunnel_id", tunnelID)
				cancel()
				return
			}
			flusher.Flush()
		}
	}
}

// handleUpload receives data from client via HTTP POST and forwards to TCP
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tunnelID := r.URL.Query().Get("tunnel_id")
	if tunnelID == "" {
		http.Error(w, "tunnel_id parameter required", http.StatusBadRequest)
		return
	}

	// Read data from request body
	data, err := io.ReadAll(r.Body)
	if err != nil {
		log.Debug("Failed to read upload data", "error", err, "tunnel_id", tunnelID)
		http.Error(w, "Failed to read data", http.StatusBadRequest)
		return
	}

	if len(data) == 0 {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
		return
	}

	log.Debug("Received upload data", "tunnel_id", tunnelID, "size", len(data))

	// Find the tunnel and send data to it
	s.tunnelsMu.RLock()
	tunnel, exists := s.tunnels[tunnelID]
	s.tunnelsMu.RUnlock()

	if !exists {
		log.Warn("Tunnel not found", "tunnel_id", tunnelID)
		http.Error(w, "Tunnel not found", http.StatusNotFound)
		return
	}

	// Send data to download channel (will be written to TCP)
	select {
	case tunnel.downloadChan <- data:
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	case <-tunnel.ctx.Done():
		http.Error(w, "Tunnel closed", http.StatusGone)
	case <-time.After(5 * time.Second):
		log.Warn("Download channel blocked", "tunnel_id", tunnelID)
		http.Error(w, "Timeout", http.StatusRequestTimeout)
	}
}

func (s *Server) getOrCreatePool(port int) *ConnectionPool {
	s.mu.Lock()
	defer s.mu.Unlock()

	pool, exists := s.pools[port]
	if !exists {
		pool = &ConnectionPool{
			tunnels: make(chan *Tunnel, 10000), // Large buffer to handle many concurrent connections
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

		// Get a tunnel from the pool
		select {
		case tunnel := <-pool.tunnels:
			go s.handleTCPConnection(tcpConn, tunnel)
		case <-time.After(5 * time.Second):
			log.Warn("No tunnel available in pool", "port", pool.port)
			tcpConn.Close()
		}
	}
}

func (s *Server) handleTCPConnection(tcpConn net.Conn, tunnel *Tunnel) {
	defer tcpConn.Close()
	defer tunnel.cancel()

	log.Debug("Handling TCP connection with tunnel", "tunnel_id", tunnel.id)

	ctx, cancel := context.WithCancel(tunnel.ctx)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(2)

	// TCP -> Upload channel (which streams to client via GET)
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

			// Send data to upload channel (will be streamed via GET)
			data := make([]byte, n)
			copy(data, buf[:n])
			
			select {
			case tunnel.uploadChan <- data:
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
				log.Warn("Upload channel blocked, closing connection")
				return
			}
		}
	}()

	// Download channel -> TCP (data from client POST requests)
	go func() {
		defer wg.Done()
		defer cancel()

		for {
			select {
			case <-ctx.Done():
				return
			case data := <-tunnel.downloadChan:
				if len(data) == 0 {
					continue
				}

				tcpConn.SetWriteDeadline(time.Now().Add(30 * time.Second))
				_, err := tcpConn.Write(data)
				if err != nil {
					log.Debug("TCP write error", "error", err)
					return
				}
			}
		}
	}()

	wg.Wait()
	log.Debug("TCP connection handler finished", "tunnel_id", tunnel.id)
}
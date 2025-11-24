package client

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"neofrp/internal/config"

	"github.com/charmbracelet/log"
	"github.com/gorilla/websocket"
)

type Client struct {
	cfg  *config.ClientConfig
	pool *ConnectionPool
}

type ConnectionPool struct {
	conns    []*websocket.Conn
	mu       sync.Mutex
	cfg      *config.ClientConfig
	ctx      context.Context
	cancel   context.CancelFunc
}

func NewClient(cfg *config.ClientConfig) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &Client{
		cfg: cfg,
		pool: &ConnectionPool{
			conns:  make([]*websocket.Conn, 0, cfg.PoolSize),
			cfg:    cfg,
			ctx:    ctx,
			cancel: cancel,
		},
	}
}

func (c *Client) Start() error {
	log.Info("Starting FRP client",
		"server", fmt.Sprintf("%s:%d", c.cfg.ServerAddr, c.cfg.ServerPort),
		"local", fmt.Sprintf("%s:%d", c.cfg.LocalAddr, c.cfg.LocalPort),
		"remote_port", c.cfg.RemotePort,
		"pool_size", c.cfg.PoolSize)

	// Initialize connection pool
	for i := 0; i < c.cfg.PoolSize; i++ {
		if err := c.pool.addConnection(); err != nil {
			log.Error("Failed to create initial connection", "index", i, "error", err)
		}
	}

	// Monitor and maintain pool
	go c.pool.maintain()

	// Block forever
	select {}
}

func (p *ConnectionPool) addConnection() error {
	// Build WebSocket URL
	scheme := "ws"
	if p.cfg.ServerPort == 443 {
		scheme = "wss"
	}

	u := url.URL{
		Scheme:   scheme,
		Host:     fmt.Sprintf("%s:%d", p.cfg.ServerAddr, p.cfg.ServerPort),
		Path:     "/ws",
		RawQuery: fmt.Sprintf("remote_port=%d", p.cfg.RemotePort),
	}

	// Create HTTP header with Authorization
	header := http.Header{}
	header.Set("Authorization", fmt.Sprintf("Bearer %s", p.cfg.AuthKey))

	// Connect to server
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.Dial(u.String(), header)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}

	log.Info("WebSocket connection established", "server", u.Host)

	p.mu.Lock()
	p.conns = append(p.conns, conn)
	p.mu.Unlock()

	// Handle this connection
	go p.handleConnection(conn)

	return nil
}

func (p *ConnectionPool) handleConnection(wsConn *websocket.Conn) {
	defer func() {
		wsConn.Close()
		p.removeConnection(wsConn)
	}()

	// Wait for server to send us data (indicating a new TCP connection)
	wsConn.SetReadDeadline(time.Time{}) // No deadline for first message
	_, firstData, err := wsConn.ReadMessage()
	if err != nil {
		log.Debug("WebSocket closed before use", "error", err)
		return
	}

	// Connect to local service
	localAddr := fmt.Sprintf("%s:%d", p.cfg.LocalAddr, p.cfg.LocalPort)
	tcpConn, err := net.DialTimeout("tcp", localAddr, 5*time.Second)
	if err != nil {
		log.Error("Failed to connect to local service", "addr", localAddr, "error", err)
		return
	}
	defer tcpConn.Close()

	log.Debug("Connected to local service", "addr", localAddr)

	// Write the first message to local service
	if len(firstData) > 0 {
		if _, err := tcpConn.Write(firstData); err != nil {
			log.Debug("Failed to write first message to TCP", "error", err)
			return
		}
	}

	ctx, cancel := context.WithCancel(p.ctx)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(2)

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

	wg.Wait()
	log.Debug("Connection handler finished")
}

func (p *ConnectionPool) removeConnection(conn *websocket.Conn) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i, c := range p.conns {
		if c == conn {
			p.conns = append(p.conns[:i], p.conns[i+1:]...)
			break
		}
	}
}

func (p *ConnectionPool) maintain() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.mu.Lock()
			current := len(p.conns)
			p.mu.Unlock()

			needed := p.cfg.PoolSize - current
			if needed > 0 {
				log.Debug("Replenishing connection pool", "current", current, "needed", needed)
				for i := 0; i < needed; i++ {
					if err := p.addConnection(); err != nil {
						log.Warn("Failed to add connection to pool", "error", err)
						time.Sleep(1 * time.Second)
					}
				}
			}
		}
	}
}
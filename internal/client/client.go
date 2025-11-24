package client

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"neofrp/internal/config"

	"github.com/charmbracelet/log"
)

type Client struct {
	cfg  *config.ClientConfig
	pool *ConnectionPool
}

type ConnectionPool struct {
	tunnels       []*Tunnel
	mu            sync.Mutex
	cfg           *config.ClientConfig
	ctx           context.Context
	cancel        context.CancelFunc
	replenishChan chan struct{}
}

type Tunnel struct {
	id         string
	httpClient *http.Client
	cfg        *config.ClientConfig
	ctx        context.Context
	cancel     context.CancelFunc
}
func generateTunnelID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}


func NewClient(cfg *config.ClientConfig) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &Client{
		cfg: cfg,
		pool: &ConnectionPool{
			tunnels:       make([]*Tunnel, 0, cfg.PoolSize),
			cfg:           cfg,
			ctx:           ctx,
			cancel:        cancel,
			replenishChan: make(chan struct{}, cfg.PoolSize),
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
		if err := c.pool.addTunnel(); err != nil {
			log.Error("Failed to create initial tunnel", "index", i, "error", err)
		}
	}

	// Monitor and maintain pool
	go c.pool.maintain()

	// Block forever
	select {}
}

func (p *ConnectionPool) addTunnel() error {
	// Generate unique tunnel ID
	tunnelID := generateTunnelID()

	ctx, cancel := context.WithCancel(p.ctx)
	
	tunnel := &Tunnel{
		id: tunnelID,
		httpClient: &http.Client{
			Timeout: 0, // No timeout for streaming
		},
		cfg:    p.cfg,
		ctx:    ctx,
		cancel: cancel,
	}

	p.mu.Lock()
	p.tunnels = append(p.tunnels, tunnel)
	p.mu.Unlock()

	log.Info("Tunnel created", "tunnel_id", tunnelID)

	// Start handling this tunnel
	go p.handleTunnel(tunnel)

	return nil
}

func (p *ConnectionPool) handleTunnel(tunnel *Tunnel) {
	defer func() {
		tunnel.cancel()
		p.removeTunnel(tunnel)
	}()

	// Build server URL
	scheme := "http"
	if tunnel.cfg.ServerPort == 443 {
		scheme = "https"
	}

	baseURL := fmt.Sprintf("%s://%s:%d", scheme, tunnel.cfg.ServerAddr, tunnel.cfg.ServerPort)
	
	// Start download stream (GET request)
	downloadURL := fmt.Sprintf("%s/tunnel/download?remote_port=%d&tunnel_id=%s",
		baseURL, tunnel.cfg.RemotePort, tunnel.id)

	req, err := http.NewRequestWithContext(tunnel.ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		log.Error("Failed to create download request", "error", err, "tunnel_id", tunnel.id)
		return
	}

	// Add authorization header
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tunnel.cfg.AuthKey))

	resp, err := tunnel.httpClient.Do(req)
	if err != nil {
		log.Error("Failed to start download stream", "error", err, "tunnel_id", tunnel.id)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Try to read error message from response body
		errMsg := ""
		if resp.Body != nil {
			// Read up to 256B of error message
			buf := make([]byte, 256)
			if n, err := resp.Body.Read(buf); err == nil && n > 0 {
				errMsg = string(buf[:n])
			}
		}
		log.Error("Download stream failed", "status", resp.StatusCode, "tunnel_id", tunnel.id, "resp_body", errMsg)
		return
	}

	log.Info("Download stream established", "tunnel_id", tunnel.id)

	// Connect to local service
	localAddr := fmt.Sprintf("%s:%d", tunnel.cfg.LocalAddr, tunnel.cfg.LocalPort)
	tcpConn, err := net.DialTimeout("tcp", localAddr, 5*time.Second)
	if err != nil {
		log.Error("Failed to connect to local service", "addr", localAddr, "error", err, "tunnel_id", tunnel.id)
		return
	}
	defer tcpConn.Close()

	log.Debug("Connected to local service", "addr", localAddr, "tunnel_id", tunnel.id)

	ctx, cancel := context.WithCancel(tunnel.ctx)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(2)

	// Download stream -> TCP (data from server via GET)
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

			n, err := resp.Body.Read(buf)
			if err != nil {
				if err != io.EOF {
					log.Debug("Download stream read error", "error", err, "tunnel_id", tunnel.id)
				}
				return
			}

			if n > 0 {
				tcpConn.SetWriteDeadline(time.Now().Add(30 * time.Second))
				_, err := tcpConn.Write(buf[:n])
				if err != nil {
					log.Debug("TCP write error", "error", err, "tunnel_id", tunnel.id)
					return
				}
			}
		}
	}()

	// TCP -> Upload (data to server via POST)
	go func() {
		defer wg.Done()
		defer cancel()

		buf := make([]byte, 32*1024)
		uploadURL := fmt.Sprintf("%s/tunnel/upload?tunnel_id=%s", baseURL, tunnel.id)

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
					log.Debug("TCP read error", "error", err, "tunnel_id", tunnel.id)
				}
				return
			}

			if n > 0 {
				// Send data via POST request
				data := make([]byte, n)
				copy(data, buf[:n])

				postReq, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, bytes.NewReader(data))
				if err != nil {
					log.Debug("Failed to create upload request", "error", err, "tunnel_id", tunnel.id)
					return
				}

				postReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tunnel.cfg.AuthKey))
				postReq.Header.Set("Content-Type", "application/octet-stream")

				postResp, err := tunnel.httpClient.Do(postReq)
				if err != nil {
					log.Debug("Upload request failed", "error", err, "tunnel_id", tunnel.id)
					return
				}
				postResp.Body.Close()

				if postResp.StatusCode != http.StatusOK {
					// Try to read error message from response body
					errMsg := ""
					if postResp.Body != nil {
						// Read up to 256B of error message
						buf := make([]byte, 256)
						if n, err := postResp.Body.Read(buf); err == nil && n > 0 {
							errMsg = string(buf[:n])
						}
					}
					log.Debug("Upload failed", "status", postResp.StatusCode, "tunnel_id", tunnel.id, "error_message", errMsg)
					return
				}
			}
		}
	}()

	wg.Wait()
	log.Debug("Tunnel handler finished", "tunnel_id", tunnel.id)
}

func (p *ConnectionPool) removeTunnel(tunnel *Tunnel) {
	p.mu.Lock()
	for i, t := range p.tunnels {
		if t.id == tunnel.id {
			p.tunnels = append(p.tunnels[:i], p.tunnels[i+1:]...)
			break
		}
	}
	p.mu.Unlock()
	
	log.Debug("Tunnel removed from pool", "tunnel_id", tunnel.id)
	
	// Signal to replenish immediately
	select {
	case p.replenishChan <- struct{}{}:
	default:
	}
}

func (p *ConnectionPool) maintain() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-p.replenishChan:
			// Immediate replenishment when a tunnel is removed
			p.replenishPool()
		case <-ticker.C:
			// Periodic check to ensure pool is healthy
			p.replenishPool()
		}
	}
}

func (p *ConnectionPool) replenishPool() {
	p.mu.Lock()
	current := len(p.tunnels)
	p.mu.Unlock()

	needed := p.cfg.PoolSize - current
	if needed > 0 {
		log.Debug("Replenishing tunnel pool", "current", current, "needed", needed)
		for i := 0; i < needed; i++ {
			if err := p.addTunnel(); err != nil {
				log.Warn("Failed to add tunnel to pool", "error", err)
				time.Sleep(1 * time.Second)
			}
		}
	}
}
package main

import (
	"flag"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/charmbracelet/log"
)

func main() {
	listenAddr := flag.String("listen", ":8080", "proxy listen address")
	backendAddr := flag.String("backend", "http://localhost:7000", "backend server address")
	authKey := flag.String("key", "test-key", "required auth key")
	flag.Parse()

	backend, err := url.Parse(*backendAddr)
	if err != nil {
		log.Fatal("Invalid backend URL", "error", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(backend)
	
	// Customize the director to preserve WebSocket upgrade headers
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		// Ensure WebSocket headers are preserved
		if req.Header.Get("Upgrade") == "websocket" {
			req.Header.Set("Connection", "Upgrade")
		}
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Check Authorization header
		authHeader := r.Header.Get("Authorization")
		expectedAuth := fmt.Sprintf("Bearer %s", *authKey)
		
		if authHeader != expectedAuth {
			log.Warn("Unauthorized request", 
				"remote", r.RemoteAddr, 
				"path", r.URL.Path,
				"auth", authHeader)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		log.Info("Proxying request", 
			"remote", r.RemoteAddr, 
			"path", r.URL.Path,
			"upgrade", r.Header.Get("Upgrade"))

		proxy.ServeHTTP(w, r)
	})

	log.Info("Starting test reverse proxy", 
		"listen", *listenAddr, 
		"backend", *backendAddr,
		"auth_key", *authKey)
	
	if err := http.ListenAndServe(*listenAddr, nil); err != nil {
		log.Fatal("Proxy failed", "error", err)
	}
}
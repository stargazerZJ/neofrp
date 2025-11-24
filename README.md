# NeoFRP - Minimal HTTP-based Fast Reverse Proxy

A minimal FRP (Fast Reverse Proxy) implementation that uses HTTP streaming transport for both server and client, with connection pooling for efficient TCP port forwarding. Compatible with reverse proxies that don't support WebSocket.

## Features

- **HTTP Streaming Transport**: Uses HTTP GET for downloading data streams and POST for uploading data packets
- **Reverse Proxy Compatible**: Works with any HTTP reverse proxy, no WebSocket support required
- **Authorization Header**: Built-in support for `Authorization: Bearer` header required by reverse proxies
- **Connection Pooling**: Maintains a pool of HTTP tunnels for better performance
- **Single Port Forwarding**: Forwards one TCP port from client to server
- **Minimal Dependencies**: Uses only standard library and `charmbracelet/log`

## Architecture

```
[Local Service] <--TCP--> [frpc] <--HTTP GET/POST--> [Reverse Proxy] <--HTTP--> [frps] <--TCP--> [Remote Client]
    :22                    Pool of HTTP tunnels      (with Auth header)           :6000
```

**Data Flow:**
- **Download (Server → Client)**: HTTP GET request with streaming response
- **Upload (Client → Server)**: HTTP POST requests with data packets

## Installation

### Build from source

```bash
# Build server
go build -o frps cmd/frps/main.go

# Build client
go build -o frpc cmd/frpc/main.go
```

## Configuration

### Server Configuration (frps.toml)

```toml
# Server bind address
bind_addr = "0.0.0.0"

# Server bind port (HTTP port)
bind_port = 7000

# Authorization key (must match client)
auth_key = "your-secret-key-here"

# Log level: debug, info, warn, error
log_level = "info"
```

### Client Configuration (frpc.toml)

```toml
# Server address (can be domain name or IP)
server_addr = "your-server.com"

# Server port (HTTP port, use 443 for HTTPS)
server_port = 443

# Authorization key (must match server)
auth_key = "your-secret-key-here"

# Local service address to forward
local_addr = "127.0.0.1"

# Local service port to forward
local_port = 22

# Remote port on server to expose
remote_port = 6000

# Connection pool size
pool_size = 5

# Log level: debug, info, warn, error
log_level = "info"
```

## Usage

### 1. Start the Server

On your server machine:

```bash
./frps -c frps.toml
```

The server will:
- Listen for HTTP connections on the configured port
- Handle GET requests for streaming data downloads
- Handle POST requests for uploading data packets
- Automatically create TCP listeners for requested remote ports
- Validate the `Authorization: Bearer` header on all connections

### 2. Start the Client

On your client machine:

```bash
./frpc -c frpc.toml
```

The client will:
- Establish a pool of HTTP tunnels to the server
- Use GET requests to receive streaming data from the server
- Use POST requests to send data packets to the server
- Include the `Authorization: Bearer $KEY` header in all requests
- Automatically reconnect if tunnels are lost
- Forward traffic between the local service and remote port

### 3. Connect to the Service

From any machine that can reach your server:

```bash
# Example: SSH through the tunnel
ssh -p 6000 user@your-server.com
```

## How It Works

### Server (frps)

1. Listens for HTTP connections on the configured port
2. Handles two types of requests:
   - **GET `/tunnel/download`**: Streams data from TCP to client (long-lived connection)
   - **POST `/tunnel/upload`**: Receives data packets from client and forwards to TCP
3. Validates the `Authorization: Bearer` header on each request
4. Maintains a tunnel pool for each remote port
5. When a TCP connection arrives on a remote port:
   - Takes a tunnel from the pool
   - Bridges the TCP connection with the HTTP tunnel
   - Forwards data bidirectionally using GET (download) and POST (upload)

### Client (frpc)

1. Establishes multiple HTTP tunnels to the server (connection pool)
2. For each tunnel:
   - Opens a GET request to `/tunnel/download` for receiving data (streaming response)
   - Sends POST requests to `/tunnel/upload` for sending data packets
3. Includes `Authorization: Bearer $KEY` header in all HTTP requests
4. Maintains the pool size by reconnecting when tunnels are lost
5. When the server uses a tunnel:
   - Connects to the local service
   - Bridges the HTTP tunnel with the local TCP connection
   - Forwards data bidirectionally

### Tunnel Pool

The tunnel pool ensures:
- Multiple concurrent connections can be handled
- Low latency for new connections (tunnels are pre-established)
- Automatic recovery from connection failures
- Efficient resource usage
- Compatible with any HTTP reverse proxy

## Reverse Proxy Configuration

If you have a reverse proxy (like Nginx or Caddy) in front of the server, configure it to:

1. Forward HTTP requests to the frps server
2. Pass through the `Authorization` header
3. Support long-lived connections for streaming

Example Nginx configuration:

```nginx
location /tunnel/ {
    proxy_pass http://localhost:7000;
    proxy_http_version 1.1;
    proxy_set_header Authorization $http_authorization;
    proxy_set_header Host $host;
    proxy_read_timeout 3600s;
    proxy_send_timeout 3600s;
    proxy_buffering off;  # Important for streaming
}
```

Example Caddy configuration:

```
your-domain.com {
    reverse_proxy /tunnel/* localhost:7000 {
        header_up Authorization {http.request.header.Authorization}
        flush_interval -1  # Disable buffering for streaming
    }
}
```

## Security Considerations

1. **Authorization Key**: Keep your `auth_key` secret and use a strong random value
2. **TLS/SSL**: Use HTTPS (port 443) in production with proper TLS certificates
3. **Firewall**: Only expose necessary ports on your server
4. **Key Rotation**: Regularly rotate your authorization keys

## Troubleshooting

### Connection Refused

- Check that frps is running and listening on the correct port
- Verify firewall rules allow connections
- Ensure the reverse proxy is properly configured

### Unauthorized Errors

- Verify the `auth_key` matches in both frps.toml and frpc.toml
- Check that the reverse proxy is passing through the Authorization header

### Connection Pool Empty

- Increase `pool_size` in frpc.toml
- Check client logs for connection errors
- Verify network connectivity between client and server

### High Latency

- Increase `pool_size` for more concurrent connections
- Check network latency between client and server
- Consider deploying the server closer to your clients

## License

See LICENSE file for details.

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.
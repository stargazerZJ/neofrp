# NeoFRP - Minimal WebSocket-based Fast Reverse Proxy

A minimal FRP (Fast Reverse Proxy) implementation that uses WebSocket transport for both server and client, with connection pooling for efficient TCP port forwarding.

## Features

- **WebSocket Transport**: Server uses WS, client supports both WS and WSS
- **Authorization Header**: Built-in support for `Authorization: Bearer` header required by reverse proxies
- **Connection Pooling**: Maintains a pool of WebSocket connections for better performance
- **Single Port Forwarding**: Forwards one TCP port from client to server
- **Minimal Dependencies**: Uses only `gorilla/websocket` and `charmbracelet/log`

## Architecture

```
[Local Service] <--TCP--> [frpc] <--WebSocket/WSS--> [Reverse Proxy] <--WebSocket--> [frps] <--TCP--> [Remote Client]
    :22                    Pool of WS connections      (with Auth header)              :6000
```

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

# Server bind port (WebSocket port)
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

# Server port (WebSocket port, use 443 for WSS)
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
- Listen for WebSocket connections on the configured port
- Automatically create TCP listeners for requested remote ports
- Validate the `Authorization: Bearer` header on all connections

### 2. Start the Client

On your client machine:

```bash
./frpc -c frpc.toml
```

The client will:
- Establish a pool of WebSocket connections to the server
- Include the `Authorization: Bearer $KEY` header in all requests
- Automatically reconnect if connections are lost
- Forward traffic between the local service and remote port

### 3. Connect to the Service

From any machine that can reach your server:

```bash
# Example: SSH through the tunnel
ssh -p 6000 user@your-server.com
```

## How It Works

### Server (frps)

1. Listens for WebSocket connections on the configured port
2. Validates the `Authorization: Bearer` header on each connection
3. Maintains a connection pool for each remote port
4. When a TCP connection arrives on a remote port:
   - Takes a WebSocket connection from the pool
   - Bridges the TCP connection with the WebSocket connection
   - Forwards data bidirectionally

### Client (frpc)

1. Establishes multiple WebSocket connections to the server (connection pool)
2. Includes `Authorization: Bearer $KEY` header in all WebSocket upgrade requests
3. Maintains the pool size by reconnecting when connections are lost
4. When the server uses a connection:
   - Connects to the local service
   - Bridges the WebSocket connection with the local TCP connection
   - Forwards data bidirectionally

### Connection Pool

The connection pool ensures:
- Multiple concurrent connections can be handled
- Low latency for new connections (no need to establish WebSocket connection on-demand)
- Automatic recovery from connection failures
- Efficient resource usage

## Reverse Proxy Configuration

If you have a reverse proxy (like Nginx or Caddy) in front of the server, configure it to:

1. Forward WebSocket connections to the frps server
2. Pass through the `Authorization` header
3. Support WebSocket upgrade

Example Nginx configuration:

```nginx
location /ws {
    proxy_pass http://localhost:7000;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Authorization $http_authorization;
    proxy_set_header Host $host;
    proxy_read_timeout 3600s;
    proxy_send_timeout 3600s;
}
```

## Security Considerations

1. **Authorization Key**: Keep your `auth_key` secret and use a strong random value
2. **TLS/SSL**: Use WSS (port 443) in production with proper TLS certificates
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
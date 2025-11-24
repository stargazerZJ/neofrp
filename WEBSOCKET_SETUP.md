# WebSocket Transport Setup Guide

This guide explains how to use WebSocket (ws) and secure WebSocket (wss) transport in neofrp, which is particularly useful when you have an HTTP/HTTPS reverse proxy in between that you cannot control.

## Overview

The WebSocket transport allows neofrp to work through HTTP/HTTPS reverse proxies by encapsulating the multiplexed stream protocol over WebSocket connections. This is useful in scenarios where:

- You have a reverse proxy (like nginx, Apache, or Cloudflare) between the client and server
- Direct TCP/QUIC connections are blocked but HTTP/HTTPS is allowed
- You need to tunnel through corporate firewalls that only allow HTTP/HTTPS traffic

## Architecture

```
Client (wss) <---> HTTPS Reverse Proxy <---> Server (ws)
     |                                              |
  frpc with                                    frps with
  wss protocol                                 ws protocol
```

The client uses `wss` (WebSocket Secure) to connect through the HTTPS reverse proxy, while the server uses `ws` (plain WebSocket) since the TLS termination happens at the reverse proxy.

## Configuration

### Server Configuration (ws)

Create `server_ws.json`:

```json
{
    "log": {
        "log_level": "debug"
    },
    "recognized_tokens": [
        "user_token"
    ],
    "transport": {
        "protocol": "ws",
        "port": 3400,
        "cert_file": "",
        "key_file": ""
    },
    "connections": {
        "tcp_ports": [35560],
        "udp_ports": [35561]
    }
}
```

**Note**: The server uses `ws` (not `wss`) because TLS termination is handled by your reverse proxy.

### Client Configuration (wss)

Create `client_wss.json`:

```json
{
    "log": {
        "log_level": "debug"
    },
    "token": "user_token",
    "transport": {
        "protocol": "wss",
        "server_ip": "your-domain.com",
        "server_port": 443,
        "ca_file": "",
        "server_name": "your-domain.com"
    },
    "connections": [
        {
            "type": "tcp",
            "local_port": 25560,
            "server_port": 35560
        },
        {
            "type": "udp",
            "local_port": 25561,
            "server_port": 35561
        }
    ]
}
```

**Note**: The client uses `wss` to connect through HTTPS (port 443) to your reverse proxy.

## Reverse Proxy Configuration

### Nginx Example

```nginx
server {
    listen 443 ssl http2;
    server_name your-domain.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://127.0.0.1:3400;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # WebSocket specific settings
        proxy_read_timeout 86400;
        proxy_send_timeout 86400;
    }
}
```

### Apache Example

```apache
<VirtualHost *:443>
    ServerName your-domain.com
    
    SSLEngine on
    SSLCertificateFile /path/to/cert.pem
    SSLCertificateKeyFile /path/to/key.pem
    
    ProxyPreserveHost On
    ProxyPass / ws://127.0.0.1:3400/
    ProxyPassReverse / ws://127.0.0.1:3400/
    
    # Enable WebSocket
    RewriteEngine On
    RewriteCond %{HTTP:Upgrade} =websocket [NC]
    RewriteRule /(.*)           ws://127.0.0.1:3400/$1 [P,L]
</VirtualHost>
```

## Running the Setup

1. **Start the server** (on your server machine):
   ```bash
   ./bin/frps -c testcases/server_ws.json
   ```

2. **Configure your reverse proxy** (nginx/Apache) to forward WebSocket connections to the frps server

3. **Start the client** (on your client machine):
   ```bash
   ./bin/frpc -c testcases/client_wss.json
   ```

## Testing

You can test the setup by forwarding a TCP port:

1. Start a test TCP server on the client machine:
   ```bash
   python3 testcases/tcp_server.py
   ```

2. Connect to the forwarded port through the server:
   ```bash
   python3 testcases/tcp_client.py
   ```

## Protocol Comparison

| Protocol | Use Case | TLS | Performance | Firewall Friendly |
|----------|----------|-----|-------------|-------------------|
| `quic` | Direct connection, best performance | Yes | Highest | Medium |
| `tcp` | Direct connection, good performance | Yes | High | Medium |
| `ws` | Server-side, behind reverse proxy | No* | Medium | High |
| `wss` | Client-side, through HTTPS proxy | Yes | Medium | Highest |

*TLS is handled by the reverse proxy

## Troubleshooting

### Connection Refused
- Ensure the reverse proxy is properly configured and running
- Check that the WebSocket upgrade headers are being forwarded
- Verify the server is listening on the correct port

### Timeout Issues
- Increase proxy timeout settings (proxy_read_timeout, proxy_send_timeout)
- Check firewall rules on both client and server

### Certificate Errors
- Ensure the client's `server_name` matches the certificate CN/SAN
- If using self-signed certificates, set `ca_file` appropriately or use `InsecureSkipVerify` (not recommended for production)

## Security Considerations

1. **Always use wss (not ws) for client connections** to ensure end-to-end encryption
2. **Use strong authentication tokens** in the configuration
3. **Keep your reverse proxy updated** with the latest security patches
4. **Use valid TLS certificates** from a trusted CA for production deployments
5. **Implement rate limiting** on your reverse proxy to prevent abuse

## Performance Notes

WebSocket transport adds a small overhead compared to direct QUIC/TCP connections due to:
- HTTP framing overhead
- Additional proxy hop
- WebSocket message framing

However, it provides excellent compatibility with existing HTTP infrastructure and can traverse most firewalls and proxies.
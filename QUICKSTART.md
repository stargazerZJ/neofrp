# Quick Start Guide

This guide will help you get NeoFRP up and running quickly.

## Prerequisites

- Go 1.23 or later (for building from source)
- A server with a public IP or domain name
- A reverse proxy (optional, but recommended for production)

## Step 1: Build the Binaries

```bash
make build
```

This creates two binaries in the `bin/` directory:
- `frps` - the server
- `frpc` - the client

## Step 2: Configure the Server

Edit `frps.toml`:

```toml
bind_addr = "0.0.0.0"
bind_port = 7000
auth_key = "change-this-to-a-secure-random-key"
log_level = "info"
```

**Important**: Change `auth_key` to a secure random string!

## Step 3: Configure the Client

Edit `frpc.toml`:

```toml
server_addr = "your-server.com"
server_port = 443
auth_key = "change-this-to-a-secure-random-key"
local_addr = "127.0.0.1"
local_port = 22
remote_port = 6000
pool_size = 5
log_level = "info"
```

**Important**: 
- Use the same `auth_key` as the server
- Set `server_addr` to your server's domain or IP
- Set `local_port` to the service you want to forward (e.g., 22 for SSH)
- Set `remote_port` to the port you want to expose on the server

## Step 4: Start the Server

On your server:

```bash
./bin/frps -c frps.toml
```

You should see:
```
INFO Starting FRP server addr=0.0.0.0:7000
```

## Step 5: Start the Client

On your client machine:

```bash
./bin/frpc -c frpc.toml
```

You should see:
```
INFO Starting FRP client server=your-server.com:443 local=127.0.0.1:22 remote_port=6000 pool_size=5
INFO WebSocket connection established server=your-server.com:443
```

## Step 6: Test the Connection

From any machine that can reach your server:

```bash
# Example: SSH through the tunnel
ssh -p 6000 user@your-server.com
```

## Example: Forwarding SSH

### Server Configuration (frps.toml)
```toml
bind_addr = "0.0.0.0"
bind_port = 7000
auth_key = "my-super-secret-key-12345"
log_level = "info"
```

### Client Configuration (frpc.toml)
```toml
server_addr = "example.com"
server_port = 443
auth_key = "my-super-secret-key-12345"
local_addr = "127.0.0.1"
local_port = 22
remote_port = 6000
pool_size = 5
log_level = "info"
```

### Connect via SSH
```bash
ssh -p 6000 user@example.com
```

## Example: Forwarding a Web Server

### Client Configuration (frpc.toml)
```toml
server_addr = "example.com"
server_port = 443
auth_key = "my-super-secret-key-12345"
local_addr = "127.0.0.1"
local_port = 8080
remote_port = 8080
pool_size = 10
log_level = "info"
```

### Access the Web Server
```bash
curl http://example.com:8080
```

## Running as a Service

### systemd (Linux)

Create `/etc/systemd/system/frps.service`:

```ini
[Unit]
Description=NeoFRP Server
After=network.target

[Service]
Type=simple
User=frp
WorkingDirectory=/opt/neofrp
ExecStart=/opt/neofrp/bin/frps -c /opt/neofrp/frps.toml
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

Enable and start:
```bash
sudo systemctl enable frps
sudo systemctl start frps
sudo systemctl status frps
```

Create `/etc/systemd/system/frpc.service` for the client similarly.

## Troubleshooting

### "Unauthorized" Error
- Verify `auth_key` matches in both configs
- Check that the reverse proxy passes the Authorization header

### "Connection refused"
- Ensure frps is running
- Check firewall rules
- Verify the port is correct

### "No WebSocket connection available in pool"
- Increase `pool_size` in frpc.toml
- Check if frpc is running and connected
- Review frpc logs for connection errors

### High CPU Usage
- Reduce `pool_size` if you don't need many concurrent connections
- Check for network issues causing frequent reconnections

## Security Best Practices

1. **Use a strong auth_key**: Generate with `openssl rand -base64 32`
2. **Use WSS in production**: Set `server_port = 443` and configure TLS
3. **Limit exposed ports**: Only expose necessary ports on your server
4. **Use a reverse proxy**: Add an extra layer of security and TLS termination
5. **Monitor logs**: Watch for unauthorized connection attempts
6. **Rotate keys regularly**: Change `auth_key` periodically

## Next Steps

- Read the full [README.md](README.md) for detailed information
- Configure a reverse proxy for production use
- Set up monitoring and logging
- Consider using a process manager like systemd or supervisor
# SSH Forwarding Example

This example demonstrates how to forward SSH connections through NeoFRP.

## Setup

1. Build the binaries:
```bash
cd ../..
make build
```

2. Start the server:
```bash
./bin/frps -c examples/ssh-forward/frps.toml
```

3. In another terminal, start the client:
```bash
./bin/frpc -c examples/ssh-forward/frpc.toml
```

4. Connect via SSH:
```bash
ssh -p 6000 user@localhost
```

## What's Happening

1. The server (`frps`) listens on port 7000 for WebSocket connections
2. The client (`frpc`) establishes 5 WebSocket connections to the server
3. When you connect to port 6000 on the server, it:
   - Takes a WebSocket connection from the pool
   - The client receives the connection and connects to local SSH (port 22)
   - Data flows: Your SSH client → Server:6000 → WebSocket → Client → Local SSH:22

## Testing with a Local Service

If you don't have SSH running, you can test with any TCP service:

1. Start a simple HTTP server:
```bash
python3 -m http.server 8080
```

2. Update `frpc.toml`:
```toml
local_port = 8080
remote_port = 8080
```

3. Restart the client and test:
```bash
curl http://localhost:8080
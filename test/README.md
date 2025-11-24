# NeoFRP Test Suite

This directory contains automated tests for NeoFRP.

## Test Coverage

The test suite validates:

1. **Direct Connection**: Client connects directly to server via WebSocket
2. **Proxy Connection**: Client connects through a reverse proxy that enforces Authorization header
3. **Auth Rejection**: Reverse proxy correctly rejects connections with wrong Authorization key
4. **TCP Forwarding**: Actual data flows correctly through the tunnel

## Running Tests

```bash
./test/test.sh
```

## Test Results

All tests passed successfully:

```
=== Test Summary ===
Tests passed: 3
Tests failed: 0

All tests passed!
```

### Test 1: Direct Connection ✓
- Server listens on port 7000 (WebSocket)
- Client connects directly to server
- TCP port 9999 forwards to local port 8888
- Data flows correctly through the tunnel

### Test 2: Connection Through Reverse Proxy ✓
- Server listens on port 7000 (WebSocket)
- Reverse proxy listens on port 8081
- Proxy enforces `Authorization: Bearer` header
- Client connects through proxy to server
- TCP port 9998 forwards to local port 8888
- Data flows correctly through proxy → server → local service

### Test 3: Auth Header Rejection ✓
- Reverse proxy configured with different auth key
- Client attempts to connect with correct key
- Proxy correctly rejects the connection as unauthorized
- Validates that Authorization header enforcement works

## Test Components

### Test Configurations

- [`test/direct/`](direct/) - Direct connection test configs
- [`test/proxy/`](proxy/) - Proxy connection test configs

### Test Proxy

[`cmd/test-proxy/main.go`](../cmd/test-proxy/main.go) - A simple reverse proxy that:
- Enforces `Authorization: Bearer` header
- Forwards WebSocket connections
- Logs all requests for debugging

### Test Script

[`test/test.sh`](test.sh) - Automated test runner that:
- Builds all binaries
- Starts a test HTTP server
- Runs all test scenarios
- Reports results with color-coded output
- Cleans up all processes

## Manual Testing

You can also test manually:

### 1. Start Test HTTP Server
```bash
cd /tmp && python3 -m http.server 8888
```

### 2. Test Direct Connection

Terminal 1 (Server):
```bash
./bin/frps -c test/direct/frps.toml
```

Terminal 2 (Client):
```bash
./bin/frpc -c test/direct/frpc.toml
```

Terminal 3 (Test):
```bash
curl http://localhost:9999/
```

### 3. Test With Proxy

Terminal 1 (Server):
```bash
./bin/frps -c test/proxy/frps.toml
```

Terminal 2 (Proxy):
```bash
./bin/test-proxy -listen :8081 -backend http://localhost:7000 -key test-secret-key-12345
```

Terminal 3 (Client):
```bash
./bin/frpc -c test/proxy/frpc.toml
```

Terminal 4 (Test):
```bash
curl http://localhost:9998/
```

## Debugging

Test logs are written to `/tmp/`:
- `/tmp/frps.log` - Server logs (direct test)
- `/tmp/frpc.log` - Client logs (direct test)
- `/tmp/frps-proxy.log` - Server logs (proxy test)
- `/tmp/frpc-proxy.log` - Client logs (proxy test)
- `/tmp/proxy.log` - Proxy logs
- `/tmp/proxy-auth.log` - Proxy logs (auth test)

View logs in real-time:
```bash
tail -f /tmp/frps.log
tail -f /tmp/frpc.log
tail -f /tmp/proxy.log
```

## Requirements

- Go 1.23+
- Python 3 (for test HTTP server)
- curl (for testing)
- bash (for test script)

## Troubleshooting

### Port Already in Use

If you see "address already in use" errors, kill existing processes:
```bash
pkill -f frps
pkill -f frpc
pkill -f test-proxy
```

### Tests Hang

Check if processes are still running:
```bash
ps aux | grep -E 'frps|frpc|test-proxy'
```

Kill them if needed:
```bash
pkill -f frps
pkill -f frpc
pkill -f test-proxy
```

### Connection Refused

Ensure the test HTTP server is running:
```bash
curl http://localhost:8888/
```

If not, start it:
```bash
cd /tmp && python3 -m http.server 8888 &
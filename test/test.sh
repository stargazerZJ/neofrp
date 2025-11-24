#!/bin/bash

set -e

echo "=== NeoFRP Test Suite ==="
echo ""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test results
TESTS_PASSED=0
TESTS_FAILED=0

# Cleanup function
cleanup() {
    echo ""
    echo "Cleaning up..."
    pkill -f "bin/frps" || true
    pkill -f "bin/frpc" || true
    pkill -f "bin/test-proxy" || true
    pkill -f "python3 -m http.server" || true
    sleep 1
}

trap cleanup EXIT

# Start a simple HTTP server on port 8888 for testing
start_test_server() {
    echo "Starting test HTTP server on port 8888..."
    cd /tmp
    python3 -m http.server 8888 > /dev/null 2>&1 &
    TEST_SERVER_PID=$!
    cd - > /dev/null
    sleep 2
    echo "Test server started (PID: $TEST_SERVER_PID)"
}

# Test 1: Direct connection (client -> server)
test_direct() {
    echo ""
    echo "=== Test 1: Direct Connection (Client -> Server) ==="
    
    # Start server
    echo "Starting frps..."
    ./bin/frps -c test/direct/frps.toml > /tmp/frps.log 2>&1 &
    FRPS_PID=$!
    sleep 2
    
    if ! ps -p $FRPS_PID > /dev/null; then
        echo -e "${RED}✗ FAILED: frps failed to start${NC}"
        cat /tmp/frps.log
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi
    echo "frps started (PID: $FRPS_PID)"
    
    # Start client
    echo "Starting frpc..."
    ./bin/frpc -c test/direct/frpc.toml > /tmp/frpc.log 2>&1 &
    FRPC_PID=$!
    sleep 3
    
    if ! ps -p $FRPC_PID > /dev/null; then
        echo -e "${RED}✗ FAILED: frpc failed to start${NC}"
        cat /tmp/frpc.log
        TESTS_FAILED=$((TESTS_FAILED + 1))
        kill $FRPS_PID 2>/dev/null || true
        return 1
    fi
    echo "frpc started (PID: $FRPC_PID)"
    
    # Test connection
    echo "Testing TCP forwarding (localhost:9999 -> localhost:8888)..."
    sleep 2
    
    if curl -s --max-time 5 http://localhost:9999/ > /dev/null 2>&1; then
        echo -e "${GREEN}✓ PASSED: Direct connection works!${NC}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo -e "${RED}✗ FAILED: Could not connect through tunnel${NC}"
        echo "Server logs:"
        tail -20 /tmp/frps.log
        echo "Client logs:"
        tail -20 /tmp/frpc.log
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
    
    # Cleanup
    kill $FRPC_PID 2>/dev/null || true
    kill $FRPS_PID 2>/dev/null || true
    sleep 1
}

# Test 2: Connection through reverse proxy
test_proxy() {
    echo ""
    echo "=== Test 2: Connection Through Reverse Proxy ==="
    
    # Start server
    echo "Starting frps..."
    ./bin/frps -c test/proxy/frps.toml > /tmp/frps-proxy.log 2>&1 &
    FRPS_PID=$!
    sleep 2
    
    if ! ps -p $FRPS_PID > /dev/null; then
        echo -e "${RED}✗ FAILED: frps failed to start${NC}"
        cat /tmp/frps-proxy.log
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi
    echo "frps started (PID: $FRPS_PID)"
    
    # Start reverse proxy
    echo "Starting test reverse proxy on port 8081..."
    ./bin/test-proxy -listen :8081 -backend http://localhost:7000 -key test-secret-key-12345 > /tmp/proxy.log 2>&1 &
    PROXY_PID=$!
    sleep 2
    
    if ! ps -p $PROXY_PID > /dev/null; then
        echo -e "${RED}✗ FAILED: test-proxy failed to start${NC}"
        cat /tmp/proxy.log
        TESTS_FAILED=$((TESTS_FAILED + 1))
        kill $FRPS_PID 2>/dev/null || true
        return 1
    fi
    echo "test-proxy started (PID: $PROXY_PID)"
    
    # Start client (connecting through proxy)
    echo "Starting frpc (connecting through proxy)..."
    ./bin/frpc -c test/proxy/frpc.toml > /tmp/frpc-proxy.log 2>&1 &
    FRPC_PID=$!
    sleep 3
    
    if ! ps -p $FRPC_PID > /dev/null; then
        echo -e "${RED}✗ FAILED: frpc failed to start${NC}"
        cat /tmp/frpc-proxy.log
        TESTS_FAILED=$((TESTS_FAILED + 1))
        kill $PROXY_PID 2>/dev/null || true
        kill $FRPS_PID 2>/dev/null || true
        return 1
    fi
    echo "frpc started (PID: $FRPC_PID)"
    
    # Test connection
    echo "Testing TCP forwarding through proxy (localhost:9998 -> proxy:8081 -> server:7000 -> localhost:8888)..."
    sleep 2
    
    if curl -s --max-time 5 http://localhost:9998/ > /dev/null 2>&1; then
        echo -e "${GREEN}✓ PASSED: Proxy connection works!${NC}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo -e "${RED}✗ FAILED: Could not connect through proxy tunnel${NC}"
        echo "Proxy logs:"
        tail -20 /tmp/proxy.log
        echo "Server logs:"
        tail -20 /tmp/frps-proxy.log
        echo "Client logs:"
        tail -20 /tmp/frpc-proxy.log
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
    
    # Cleanup
    kill $FRPC_PID 2>/dev/null || true
    kill $PROXY_PID 2>/dev/null || true
    kill $FRPS_PID 2>/dev/null || true
    sleep 1
}

# Test 3: Test auth rejection
test_auth_rejection() {
    echo ""
    echo "=== Test 3: Auth Header Rejection ==="
    
    # Start server
    echo "Starting frps..."
    ./bin/frps -c test/proxy/frps.toml > /tmp/frps-auth.log 2>&1 &
    FRPS_PID=$!
    sleep 2
    
    # Start reverse proxy with different key
    echo "Starting test reverse proxy with WRONG key..."
    ./bin/test-proxy -listen :8081 -backend http://localhost:7000 -key wrong-key > /tmp/proxy-auth.log 2>&1 &
    PROXY_PID=$!
    sleep 2
    
    # Start client (should fail to connect)
    echo "Starting frpc (should be rejected by proxy)..."
    ./bin/frpc -c test/proxy/frpc.toml > /tmp/frpc-auth.log 2>&1 &
    FRPC_PID=$!
    sleep 3
    
    # Check if connection was rejected
    if grep -q "Unauthorized" /tmp/proxy-auth.log; then
        echo -e "${GREEN}✓ PASSED: Proxy correctly rejected unauthorized connection${NC}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo -e "${RED}✗ FAILED: Proxy did not reject unauthorized connection${NC}"
        echo "Proxy logs:"
        cat /tmp/proxy-auth.log
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
    
    # Cleanup
    kill $FRPC_PID 2>/dev/null || true
    kill $PROXY_PID 2>/dev/null || true
    kill $FRPS_PID 2>/dev/null || true
    sleep 1
}

# Main test execution
main() {
    echo "Building binaries..."
    make build
    
    echo ""
    echo "Starting test HTTP server..."
    start_test_server
    
    # Run tests
    test_direct
    test_proxy
    test_auth_rejection
    
    # Summary
    echo ""
    echo "=== Test Summary ==="
    echo -e "Tests passed: ${GREEN}${TESTS_PASSED}${NC}"
    echo -e "Tests failed: ${RED}${TESTS_FAILED}${NC}"
    echo ""
    
    if [ $TESTS_FAILED -eq 0 ]; then
        echo -e "${GREEN}All tests passed!${NC}"
        exit 0
    else
        echo -e "${RED}Some tests failed!${NC}"
        exit 1
    fi
}

main
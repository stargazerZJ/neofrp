#!/bin/bash

# Test script for WebSocket (ws) protocol in neofrp
# This script tests both TCP and UDP forwarding through WebSocket transport

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
BIN_DIR="$PROJECT_ROOT/bin"
LOG_DIR="$SCRIPT_DIR/logs"

# Create log directory
mkdir -p "$LOG_DIR"

# Cleanup function
cleanup() {
    echo -e "\n${YELLOW}Cleaning up processes...${NC}"
    
    # Kill all background processes
    if [ ! -z "$SERVER_PID" ]; then
        echo "Stopping FRP server (PID: $SERVER_PID)"
        kill $SERVER_PID 2>/dev/null || true
    fi
    
    if [ ! -z "$CLIENT_PID" ]; then
        echo "Stopping FRP client (PID: $CLIENT_PID)"
        kill $CLIENT_PID 2>/dev/null || true
    fi
    
    if [ ! -z "$TCP_SERVER_PID" ]; then
        echo "Stopping TCP test server (PID: $TCP_SERVER_PID)"
        kill $TCP_SERVER_PID 2>/dev/null || true
    fi
    
    if [ ! -z "$UDP_SERVER_PID" ]; then
        echo "Stopping UDP test server (PID: $UDP_SERVER_PID)"
        kill $UDP_SERVER_PID 2>/dev/null || true
    fi
    
    # Wait a bit for processes to terminate
    sleep 1
    
    echo -e "${GREEN}Cleanup complete${NC}"
}

# Set trap to cleanup on exit
trap cleanup EXIT INT TERM

# Print header
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  WebSocket (ws) Protocol Test${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Check if binaries exist
if [ ! -f "$BIN_DIR/frps" ] || [ ! -f "$BIN_DIR/frpc" ]; then
    echo -e "${RED}Error: Binaries not found. Building...${NC}"
    cd "$PROJECT_ROOT"
    make
    if [ $? -ne 0 ]; then
        echo -e "${RED}Build failed!${NC}"
        exit 1
    fi
fi

# Step 1: Start TCP test server
echo -e "${YELLOW}[1/6] Starting TCP test server on port 25560...${NC}"
python3 "$SCRIPT_DIR/tcp_server.py" > "$LOG_DIR/tcp_server.log" 2>&1 &
TCP_SERVER_PID=$!
sleep 2

if ! ps -p $TCP_SERVER_PID > /dev/null; then
    echo -e "${RED}Failed to start TCP test server${NC}"
    cat "$LOG_DIR/tcp_server.log"
    exit 1
fi
echo -e "${GREEN}✓ TCP test server started (PID: $TCP_SERVER_PID)${NC}"

# Step 2: Start UDP test server
echo -e "${YELLOW}[2/6] Starting UDP test server on port 25561...${NC}"
python3 "$SCRIPT_DIR/udp_server.py" > "$LOG_DIR/udp_server.log" 2>&1 &
UDP_SERVER_PID=$!
sleep 2

if ! ps -p $UDP_SERVER_PID > /dev/null; then
    echo -e "${RED}Failed to start UDP test server${NC}"
    cat "$LOG_DIR/udp_server.log"
    exit 1
fi
echo -e "${GREEN}✓ UDP test server started (PID: $UDP_SERVER_PID)${NC}"

# Step 3: Start FRP server with WebSocket
echo -e "${YELLOW}[3/6] Starting FRP server with WebSocket protocol...${NC}"
"$BIN_DIR/frps" -c "$SCRIPT_DIR/server_ws.json" > "$LOG_DIR/frps.log" 2>&1 &
SERVER_PID=$!
sleep 3

if ! ps -p $SERVER_PID > /dev/null; then
    echo -e "${RED}Failed to start FRP server${NC}"
    cat "$LOG_DIR/frps.log"
    exit 1
fi
echo -e "${GREEN}✓ FRP server started (PID: $SERVER_PID)${NC}"

# Step 4: Start FRP client with WebSocket
echo -e "${YELLOW}[4/6] Starting FRP client with WebSocket protocol...${NC}"
"$BIN_DIR/frpc" -c "$SCRIPT_DIR/client_ws.json" > "$LOG_DIR/frpc.log" 2>&1 &
CLIENT_PID=$!
sleep 3

if ! ps -p $CLIENT_PID > /dev/null; then
    echo -e "${RED}Failed to start FRP client${NC}"
    cat "$LOG_DIR/frpc.log"
    exit 1
fi
echo -e "${GREEN}✓ FRP client started (PID: $CLIENT_PID)${NC}"

# Step 5: Test TCP forwarding
echo -e "${YELLOW}[5/6] Testing TCP forwarding through WebSocket...${NC}"
python3 "$SCRIPT_DIR/tcp_client.py" --host 127.0.0.1 --port 35560 --count 3 --interval 1 > "$LOG_DIR/tcp_client.log" 2>&1
TCP_TEST_RESULT=$?

if [ $TCP_TEST_RESULT -eq 0 ]; then
    echo -e "${GREEN}✓ TCP test passed${NC}"
    echo -e "${BLUE}TCP test output:${NC}"
    tail -n 10 "$LOG_DIR/tcp_client.log"
else
    echo -e "${RED}✗ TCP test failed${NC}"
    echo -e "${RED}TCP client log:${NC}"
    cat "$LOG_DIR/tcp_client.log"
fi

echo ""

# Step 6: Test UDP forwarding
echo -e "${YELLOW}[6/6] Testing UDP forwarding through WebSocket...${NC}"
python3 "$SCRIPT_DIR/udp_client.py" --host 127.0.0.1 --port 35561 --count 3 --interval 1 > "$LOG_DIR/udp_client.log" 2>&1
UDP_TEST_RESULT=$?

if [ $UDP_TEST_RESULT -eq 0 ]; then
    echo -e "${GREEN}✓ UDP test passed${NC}"
    echo -e "${BLUE}UDP test output:${NC}"
    tail -n 10 "$LOG_DIR/udp_client.log"
else
    echo -e "${RED}✗ UDP test failed${NC}"
    echo -e "${RED}UDP client log:${NC}"
    cat "$LOG_DIR/udp_client.log"
fi

echo ""
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  Test Summary${NC}"
echo -e "${BLUE}========================================${NC}"

# Print summary
if [ $TCP_TEST_RESULT -eq 0 ] && [ $UDP_TEST_RESULT -eq 0 ]; then
    echo -e "${GREEN}✓ All tests passed!${NC}"
    echo -e "${GREEN}WebSocket (ws) protocol is working correctly${NC}"
    EXIT_CODE=0
else
    echo -e "${RED}✗ Some tests failed${NC}"
    [ $TCP_TEST_RESULT -ne 0 ] && echo -e "${RED}  - TCP test failed${NC}"
    [ $UDP_TEST_RESULT -ne 0 ] && echo -e "${RED}  - UDP test failed${NC}"
    EXIT_CODE=1
fi

echo ""
echo -e "${BLUE}Log files location: $LOG_DIR${NC}"
echo -e "  - FRP Server: $LOG_DIR/frps.log"
echo -e "  - FRP Client: $LOG_DIR/frpc.log"
echo -e "  - TCP Server: $LOG_DIR/tcp_server.log"
echo -e "  - UDP Server: $LOG_DIR/udp_server.log"
echo -e "  - TCP Client: $LOG_DIR/tcp_client.log"
echo -e "  - UDP Client: $LOG_DIR/udp_client.log"

echo ""
echo -e "${YELLOW}Press Enter to view detailed logs or Ctrl+C to exit...${NC}"
read -t 5 || true

# Show some logs if tests failed
if [ $EXIT_CODE -ne 0 ]; then
    echo ""
    echo -e "${YELLOW}=== FRP Server Log (last 20 lines) ===${NC}"
    tail -n 20 "$LOG_DIR/frps.log"
    echo ""
    echo -e "${YELLOW}=== FRP Client Log (last 20 lines) ===${NC}"
    tail -n 20 "$LOG_DIR/frpc.log"
fi

exit $EXIT_CODE
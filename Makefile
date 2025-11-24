.PHONY: all build clean server client test test-proxy

all: build

build: server client test-proxy

server:
	@echo "Building frps..."
	@go build -o bin/frps cmd/frps/main.go

client:
	@echo "Building frpc..."
	@go build -o bin/frpc cmd/frpc/main.go

test-proxy:
	@echo "Building test-proxy..."
	@go build -o bin/test-proxy cmd/test-proxy/main.go

clean:
	@echo "Cleaning..."
	@rm -rf bin/

test:
	@echo "Running tests..."
	@go test -v ./...

run-server:
	@./bin/frps -c frps.toml

run-client:
	@./bin/frpc -c frpc.toml

install:
	@echo "Installing..."
	@go install ./cmd/frps
	@go install ./cmd/frpc
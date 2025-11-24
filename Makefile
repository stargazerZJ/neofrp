.PHONY: all build clean server client test

all: build

build: server client

server:
	@echo "Building frps..."
	@go build -o bin/frps cmd/frps/main.go

client:
	@echo "Building frpc..."
	@go build -o bin/frpc cmd/frpc/main.go

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
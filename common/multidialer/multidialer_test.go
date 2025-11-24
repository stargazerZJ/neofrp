package multidialer

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/charmbracelet/log"
)

func TestWSTransport(t *testing.T) {
	// Setup logger
	log.SetLevel(log.DebugLevel)

	// Start a WS listener
	port := 12345
	address := fmt.Sprintf("127.0.0.1:%d", port)
	listener, err := Listen(context.Background(), "ws", address, nil)
	if err != nil {
		t.Fatalf("Failed to start listener: %v", err)
	}
	defer listener.Close()

	// Accept session in a goroutine
	go func() {
		session, err := listener.AcceptSession()
		if err != nil {
			t.Errorf("Failed to accept session: %v", err)
			return
		}
		defer session.Close(nil)

		// Accept stream
		stream, err := session.AcceptStream(context.Background())
		if err != nil {
			t.Errorf("Failed to accept stream: %v", err)
			return
		}
		defer stream.Close()

		// Echo data back
		_, err = io.Copy(stream, stream)
		if err != nil && err != io.EOF {
			t.Errorf("Failed to echo data: %v", err)
		}
	}()

	// Wait for listener to be ready
	time.Sleep(100 * time.Millisecond)

	// Dial the listener
	session, err := Dial(context.Background(), "ws", address, nil)
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer session.Close(nil)

	// Open a stream
	stream, err := session.OpenStream(context.Background())
	if err != nil {
		t.Fatalf("Failed to open stream: %v", err)
	}
	defer stream.Close()

	// Send data
	message := "Hello, WebSocket!"
	_, err = stream.Write([]byte(message))
	if err != nil {
		t.Fatalf("Failed to write data: %v", err)
	}

	// Read response
	buffer := make([]byte, len(message))
	_, err = io.ReadFull(stream, buffer)
	if err != nil {
		t.Fatalf("Failed to read data: %v", err)
	}

	if string(buffer) != message {
		t.Errorf("Expected %s, got %s", message, string(buffer))
	}
}

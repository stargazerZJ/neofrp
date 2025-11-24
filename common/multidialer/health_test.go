package multidialer

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWSHealthEndpoint(t *testing.T) {
	// Start a WS listener
	l, err := Listen(context.Background(), "ws", "127.0.0.1:0", nil)
	assert.NoError(t, err)
	defer l.Close()

	// Get the address
	addr := l.Addr().String()
	url := fmt.Sprintf("http://%s/health", addr)

	// Make a request to the health endpoint
	resp, err := http.Get(url)
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Equal(t, "OK", string(body))
}

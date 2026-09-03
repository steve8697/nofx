package market

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gorilla/websocket"
)

func TestCombinedStreamsConcurrentSubscribe(t *testing.T) {
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		for {
			_, _, err := c.ReadMessage()
			if err != nil {
				break
			}
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	dialer := websocket.Dialer{}
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to dial test websocket: %v", err)
	}

	client := NewCombinedStreamsClient(10)
	client.conn = conn

	// Launch 50 concurrent goroutines calling subscribeStreams simultaneously
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			err := client.subscribeStreams([]string{"btcusdt@kline_3m", "ethusdt@kline_15m"})
			if err != nil {
				t.Errorf("subscribeStreams error: %v", err)
			}
		}(i)
	}
	wg.Wait()

	client.Close()
}

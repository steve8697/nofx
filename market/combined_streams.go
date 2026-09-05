package market

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type CombinedStreamsClient struct {
	conn        *websocket.Conn
	mu          sync.RWMutex
	writeMu     sync.Mutex
	subscribers map[string]chan []byte
	reconnect   bool
	done        chan struct{}
	batchSize   int // 每批订阅的流数量
}

func NewCombinedStreamsClient(batchSize int) *CombinedStreamsClient {
	return &CombinedStreamsClient{
		subscribers: make(map[string]chan []byte),
		reconnect:   true,
		done:        make(chan struct{}),
		batchSize:   batchSize,
	}
}

func (c *CombinedStreamsClient) Connect() error {
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	// 组合流使用不同的端点
	conn, _, err := dialer.Dial("wss://fstream.binance.com/stream", nil)
	if err != nil {
		return fmt.Errorf("组合流WebSocket连接失败: %v", err)
	}

	c.mu.Lock()
	if c.conn != nil {
		c.conn.Close()
	}
	c.conn = conn
	c.mu.Unlock()

	// 🛡️ 心跳保活：設置 ReadDeadline 與 PongHandler，徹底消除半開連線靜默假死
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	log.Println("组合流WebSocket连接成功")
	go c.readMessages()

	return nil
}

// BatchSubscribeKlines 批量订阅K线
func (c *CombinedStreamsClient) BatchSubscribeKlines(symbols []string, interval string) error {
	// 将symbols分批处理
	batches := c.splitIntoBatches(symbols, c.batchSize)

	for i, batch := range batches {
		log.Printf("订阅第 %d 批, 数量: %d", i+1, len(batch))

		streams := make([]string, len(batch))
		for j, symbol := range batch {
			streams[j] = fmt.Sprintf("%s@kline_%s", strings.ToLower(symbol), interval)
		}

		if err := c.subscribeStreams(streams); err != nil {
			return fmt.Errorf("第 %d 批订阅失败: %v", i+1, err)
		}

		// 批次间延迟，避免被限制
		if i < len(batches)-1 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	return nil
}

// splitIntoBatches 将切片分成指定大小的批次
func (c *CombinedStreamsClient) splitIntoBatches(symbols []string, batchSize int) [][]string {
	var batches [][]string

	for i := 0; i < len(symbols); i += batchSize {
		end := i + batchSize
		if end > len(symbols) {
			end = len(symbols)
		}
		batches = append(batches, symbols[i:end])
	}

	return batches
}

// subscribeStreams 订阅多个流
func (c *CombinedStreamsClient) subscribeStreams(streams []string) error {
	subscribeMsg := map[string]interface{}{
		"method": "SUBSCRIBE",
		"params": streams,
		"id":     time.Now().UnixNano(),
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("WebSocket未连接")
	}

	log.Printf("订阅流: %v", streams)
	return conn.WriteJSON(subscribeMsg)
}

// UnsubscribeStreams 取消订阅多个流
func (c *CombinedStreamsClient) UnsubscribeStreams(streams []string) error {
	unsubscribeMsg := map[string]interface{}{
		"method": "UNSUBSCRIBE",
		"params": streams,
		"id":     time.Now().UnixNano(),
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("WebSocket未连接")
	}

	log.Printf("取消订阅流: %v", streams)
	return conn.WriteJSON(unsubscribeMsg)
}

func (c *CombinedStreamsClient) readMessages() {
	for {
		select {
		case <-c.done:
			return
		default:
			c.mu.RLock()
			conn := c.conn
			c.mu.RUnlock()

			if conn == nil {
				time.Sleep(1 * time.Second)
				continue
			}

			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Printf("读取组合流消息失败: %v", err)
				c.handleReconnect()
				return
			}

			c.handleCombinedMessage(message)
		}
	}
}

func (c *CombinedStreamsClient) handleCombinedMessage(message []byte) {
	var combinedMsg struct {
		Stream string          `json:"stream"`
		Data   json.RawMessage `json:"data"`
	}

	if err := json.Unmarshal(message, &combinedMsg); err != nil {
		log.Printf("解析组合消息失败: %v", err)
		return
	}

	c.mu.RLock()
	ch, exists := c.subscribers[combinedMsg.Stream]
	if exists && ch != nil {
		func() {
			defer func() {
				if r := recover(); r != nil {
					// 订阅通道在并发移除时被关闭，静默捕获防止程序崩溃
				}
			}()
			select {
			case ch <- combinedMsg.Data:
			default:
				log.Printf("订阅者通道已满: %s", combinedMsg.Stream)
			}
		}()
	}
	c.mu.RUnlock()
}

func (c *CombinedStreamsClient) AddSubscriber(stream string, bufferSize int) <-chan []byte {
	ch := make(chan []byte, bufferSize)
	c.mu.Lock()
	if oldCh, exists := c.subscribers[stream]; exists {
		close(oldCh) // 关闭旧通道以允许旧的接收协程优雅退出
	}
	c.subscribers[stream] = ch
	c.mu.Unlock()
	return ch
}

// RemoveSubscriber 移除订阅者并关闭通道
func (c *CombinedStreamsClient) RemoveSubscriber(stream string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ch, exists := c.subscribers[stream]; exists {
		close(ch)
		delete(c.subscribers, stream)
	}
}

func (c *CombinedStreamsClient) handleReconnect() {
	backoff := 3 * time.Second
	maxBackoff := 60 * time.Second

	for {
		c.mu.RLock()
		reconn := c.reconnect
		c.mu.RUnlock()
		if !reconn {
			return
		}

		select {
		case <-c.done:
			return
		case <-time.After(backoff):
		}

		c.mu.RLock()
		reconn = c.reconnect
		c.mu.RUnlock()
		if !reconn {
			return
		}

		log.Printf("组合流尝试重新连接 (退避: %v)...", backoff)
		if err := c.Connect(); err != nil {
			log.Printf("组合流重新连接失败: %v", err)
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		// 重新连接成功后，自动重新订阅所有现有活跃流
		c.resubscribeAll()
		return
	}
}

func (c *CombinedStreamsClient) resubscribeAll() {
	c.mu.RLock()
	streams := make([]string, 0, len(c.subscribers))
	for s := range c.subscribers {
		streams = append(streams, s)
	}
	c.mu.RUnlock()

	if len(streams) == 0 {
		return
	}

	log.Printf("🔄 组合流重连成功，正在重新订阅 %d 个活跃流...", len(streams))
	batches := c.splitIntoBatches(streams, c.batchSize)
	for i, batch := range batches {
		if err := c.subscribeStreams(batch); err != nil {
			log.Printf("⚠️ 重新订阅第 %d 批失败: %v", i+1, err)
		}
		if i < len(batches)-1 {
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func (c *CombinedStreamsClient) Close() {
	c.reconnect = false
	close(c.done)

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}

	for stream, ch := range c.subscribers {
		close(ch)
		delete(c.subscribers, stream)
	}
}

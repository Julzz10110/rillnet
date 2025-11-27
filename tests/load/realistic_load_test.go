package main

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"rillnet/internal/core/domain"

	"github.com/gorilla/websocket"
)

type LoadTestClient struct {
	peerID      domain.PeerID
	streamID    domain.StreamID
	wsConn      *websocket.Conn
	isPublisher bool
}

func NewLoadTestClient(peerID domain.PeerID, streamID domain.StreamID, isPublisher bool) *LoadTestClient {
	return &LoadTestClient{
		peerID:      peerID,
		streamID:    streamID,
		isPublisher: isPublisher,
	}
}

func (c *LoadTestClient) Connect(signalURL string) error {
	conn, _, err := websocket.DefaultDialer.Dial(signalURL, nil)
	if err != nil {
		return err
	}
	c.wsConn = conn
	return nil
}

func (c *LoadTestClient) JoinStream() error {
	joinMsg := map[string]interface{}{
		"type": "join_stream",
		"payload": map[string]interface{}{
			"stream_id":    c.streamID,
			"is_publisher": c.isPublisher,
			"capabilities": map[string]interface{}{
				"max_bitrate": 1000 + rand.Intn(2000),
				"codecs":      []string{"VP8", "H264"},
			},
		},
	}
	return c.wsConn.WriteJSON(joinMsg)
}

func RunRealisticLoadTest() {
	const (
		numPublishers  = 3
		numSubscribers = 20
		signalURL      = "ws://localhost:8081/ws"
		streamID       = "load-test-stream"
		testDuration   = 5 * time.Minute
	)

	var wg sync.WaitGroup
	clients := make([]*LoadTestClient, 0)

	// Create publishers
	for i := 0; i < numPublishers; i++ {
		client := NewLoadTestClient(
			domain.PeerID(fmt.Sprintf("publisher-%d", i)),
			domain.StreamID(streamID),
			true,
		)
		clients = append(clients, client)
	}

	// Create subscribers
	for i := 0; i < numSubscribers; i++ {
		client := NewLoadTestClient(
			domain.PeerID(fmt.Sprintf("subscriber-%d", i)),
			domain.StreamID(streamID),
			false,
		)
		clients = append(clients, client)
	}

	// Connect all clients
	for _, client := range clients {
		wg.Add(1)
		go func(c *LoadTestClient) {
			defer wg.Done()

			if err := c.Connect(signalURL); err != nil {
				log.Printf("Failed to connect client %s: %v", c.peerID, err)
				return
			}

			if err := c.JoinStream(); err != nil {
				log.Printf("Failed to join stream for client %s: %v", c.peerID, err)
				return
			}

			log.Printf("Client %s connected and joined stream", c.peerID)
		}(client)
	}

	wg.Wait()
	log.Printf("All clients connected. Running test for %v", testDuration)

	// Wait for test completion
	time.Sleep(testDuration)

	log.Println("Load test completed")
}

func main() {
	RunRealisticLoadTest()
}

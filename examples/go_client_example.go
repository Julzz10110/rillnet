package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// RillNetClient is a simple client for RillNet API
type RillNetClient struct {
	baseURL    string
	httpClient *http.Client
	token      string
}

// NewRillNetClient creates a new RillNet client
func NewRillNetClient(baseURL string) *RillNetClient {
	return &RillNetClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetToken sets the authentication token
func (c *RillNetClient) SetToken(token string) {
	c.token = token
}

// Register registers a new user
func (c *RillNetClient) Register(username, email, password string) (map[string]interface{}, error) {
	reqBody := map[string]string{
		"username": username,
		"email":    email,
		"password": password,
	}

	return c.post("/api/v1/auth/register", reqBody)
}

// Login logs in a user
func (c *RillNetClient) Login(email, password string) (map[string]interface{}, error) {
	reqBody := map[string]string{
		"email":    email,
		"password": password,
	}

	return c.post("/api/v1/auth/login", reqBody)
}

// CreateStream creates a new stream
func (c *RillNetClient) CreateStream(name string, maxPeers int) (map[string]interface{}, error) {
	reqBody := map[string]interface{}{
		"name":      name,
		"max_peers": maxPeers,
	}

	return c.post("/api/v1/streams", reqBody)
}

// GetStream gets stream details
func (c *RillNetClient) GetStream(streamID string) (map[string]interface{}, error) {
	return c.get(fmt.Sprintf("/api/v1/streams/%s", streamID))
}

// ListStreams lists all streams
func (c *RillNetClient) ListStreams() (map[string]interface{}, error) {
	return c.get("/api/v1/streams")
}

// JoinStream joins a stream
func (c *RillNetClient) JoinStream(streamID, peerID string, isPublisher bool) (map[string]interface{}, error) {
	reqBody := map[string]interface{}{
		"peer_id":      peerID,
		"is_publisher": isPublisher,
		"capabilities": map[string]interface{}{
			"codecs": []string{"VP8", "Opus"},
		},
	}

	return c.post(fmt.Sprintf("/api/v1/streams/%s/join", streamID), reqBody)
}

// GetStreamStats gets stream statistics
func (c *RillNetClient) GetStreamStats(streamID string) (map[string]interface{}, error) {
	return c.get(fmt.Sprintf("/api/v1/streams/%s/stats", streamID))
}

// Helper methods
func (c *RillNetClient) post(path string, body interface{}) (map[string]interface{}, error) {
	jsonData, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", c.baseURL+path, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return c.parseResponse(resp)
}

func (c *RillNetClient) get(path string) (map[string]interface{}, error) {
	req, err := http.NewRequest("GET", c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return c.parseResponse(resp)
}

func (c *RillNetClient) parseResponse(resp *http.Response) (map[string]interface{}, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		var errorResp map[string]interface{}
		if err := json.Unmarshal(body, &errorResp); err == nil {
			return nil, fmt.Errorf("API error: %v", errorResp)
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// Example usage
func main() {
	client := NewRillNetClient("http://localhost:8080")

	// Register
	resp, err := client.Register("user1", "user1@example.com", "password123")
	if err != nil {
		fmt.Printf("Registration error: %v\n", err)
		return
	}

	// Set token
	if token, ok := resp["access_token"].(string); ok {
		client.SetToken(token)
		fmt.Println("Registered and logged in")
	}

	// Create stream
	streamResp, err := client.CreateStream("My Stream", 50)
	if err != nil {
		fmt.Printf("Create stream error: %v\n", err)
		return
	}

	fmt.Printf("Stream created: %v\n", streamResp)

	// Get stream stats
	if stream, ok := streamResp["stream"].(map[string]interface{}); ok {
		if streamID, ok := stream["id"].(string); ok {
			stats, err := client.GetStreamStats(streamID)
			if err != nil {
				fmt.Printf("Get stats error: %v\n", err)
				return
			}
			fmt.Printf("Stream stats: %v\n", stats)
		}
	}
}


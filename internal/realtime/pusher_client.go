// Package realtime implements a minimal client for Pusher Channels'
// WebSocket protocol (protocol=7), enough to subscribe to a private shop
// channel and react to "new-print-job" events. We hand-roll this instead of
// depending on the server-side pusher-http-go SDK (which is for *triggering*
// events, not subscribing) or an unofficial client library.
package realtime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// OnPrintJob is invoked with the raw JSON payload of a "new-print-job"
// event's "data" field whenever one arrives.
type OnPrintJob func(dataJSON string)

// ConnectionState represents the Pusher connection status.
type ConnectionState string

const (
	StateDisconnected ConnectionState = "Disconnected"
	StateConnecting   ConnectionState = "Connecting"
	StateConnected    ConnectionState = "Connected"
)

// OnStateChange is invoked whenever the Pusher connection status changes.
type OnStateChange func(state ConnectionState)

// Client maintains a reconnecting websocket connection to Pusher and
// resubscribes to the configured private channel on every (re)connect.
type Client struct {
	AppKey      string
	Cluster     string
	ChannelName string // e.g. "private-shop-<shop-id>"
	AuthURL     string // shop backend endpoint that signs channel auth
	AuthToken   string // bearer token identifying this shop to AuthURL

	OnJob         OnPrintJob
	OnStateChange OnStateChange

	stop chan struct{}
}

type pusherEnvelope struct {
	Event   string `json:"event"`
	Data    string `json:"data"`
	Channel string `json:"channel,omitempty"`
}

type connectionEstablishedData struct {
	SocketID string `json:"socket_id"`
}

type authResponse struct {
	Auth string `json:"auth"`
}

// Start begins the connect/listen/reconnect loop in a background goroutine
// and returns immediately. Call Stop to shut it down.
func (c *Client) Start() {
	c.stop = make(chan struct{})
	go c.runForever()
}

// Stop terminates the reconnect loop and closes any active connection.
func (c *Client) Stop() {
	if c.stop != nil {
		close(c.stop)
	}
	c.setState(StateDisconnected)
}

func (c *Client) setState(state ConnectionState) {
	if c.OnStateChange != nil {
		c.OnStateChange(state)
	}
}

func (c *Client) runForever() {
	backoff := time.Second
	const maxBackoff = 60 * time.Second

	for {
		select {
		case <-c.stop:
			c.setState(StateDisconnected)
			return
		default:
		}

		c.setState(StateConnecting)
		if err := c.connectAndListen(); err != nil {
			c.setState(StateDisconnected)
			log.Printf("[pusher] connection error: %v (retrying in %s)", err, backoff)
		}

		select {
		case <-c.stop:
			c.setState(StateDisconnected)
			return
		case <-time.After(backoff):
		}

		backoff = time.Duration(math.Min(float64(backoff)*2, float64(maxBackoff)))
	}
}

func (c *Client) connectAndListen() error {
	url := fmt.Sprintf("wss://ws-%s.pusher.com/app/%s?protocol=7&client=smartprint-agent&version=1.0", c.Cluster, c.AppKey)

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	// First frame from Pusher is always pusher:connection_established.
	var established pusherEnvelope
	if err := conn.ReadJSON(&established); err != nil {
		return fmt.Errorf("read handshake: %w", err)
	}
	var connData connectionEstablishedData
	if err := json.Unmarshal([]byte(established.Data), &connData); err != nil {
		return fmt.Errorf("parse handshake data: %w", err)
	}

	auth, err := c.authenticate(connData.SocketID)
	if err != nil {
		return fmt.Errorf("authenticate channel: %w", err)
	}

	subscribePayload := map[string]interface{}{
		"event": "pusher:subscribe",
		"data": map[string]string{
			"channel": c.ChannelName,
			"auth":    auth,
		},
	}
	if err := conn.WriteJSON(subscribePayload); err != nil {
		return fmt.Errorf("send subscribe: %w", err)
	}

	c.setState(StateConnected)

	// Reset backoff for the caller by returning nil only on clean shutdown;
	// on any read error below we return an error and runForever backs off.
	for {
		select {
		case <-c.stop:
			return nil
		default:
		}

		var msg pusherEnvelope
		if err := conn.ReadJSON(&msg); err != nil {
			return fmt.Errorf("read: %w", err)
		}

		switch msg.Event {
		case "pusher:ping":
			_ = conn.WriteJSON(map[string]string{"event": "pusher:pong", "data": "{}"})
		case "pusher:error":
			return fmt.Errorf("pusher error frame: %s", msg.Data)
		case "new-print-job":
			if c.OnJob != nil {
				c.OnJob(msg.Data)
			}
		}
	}
}

// authenticate calls the shop backend's Pusher auth endpoint, the same way
// pusher-js does for private channels, so the app secret never has to live
// on the shopkeeper's PC.
func (c *Client) authenticate(socketID string) (string, error) {
	body, err := json.Marshal(map[string]string{
		"socket_id":    socketID,
		"channel_name": c.ChannelName,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, c.AuthURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.AuthToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("auth endpoint returned status %d", resp.StatusCode)
	}

	var parsed authResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", err
	}
	if parsed.Auth == "" {
		return "", fmt.Errorf("auth endpoint returned empty auth string")
	}
	return parsed.Auth, nil
}

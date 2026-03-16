// Package client provides HTTP and SSE clients for the ratchet-monitor API.
package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient sets a custom http.Client for REST and SSE requests.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		c.httpClient = hc
	}
}

// WithBaseBackoff sets the initial backoff duration for SSE reconnection.
func WithBaseBackoff(d time.Duration) Option {
	return func(c *Client) {
		c.baseBackoff = d
	}
}

// WithStateCallback registers a callback invoked on connection state changes.
func WithStateCallback(fn func(ConnectionState)) Option {
	return func(c *Client) {
		c.stateCallback = fn
	}
}

// Client is an HTTP+SSE client for the ratchet-monitor API.
type Client struct {
	baseURL       string
	httpClient    *http.Client
	baseBackoff   time.Duration
	stateCallback func(ConnectionState)

	connState   atomic.Int32
	mu          sync.Mutex
	lastEventID string
}

// NewClient creates a new Client targeting the given base URL.
func NewClient(baseURL string, opts ...Option) *Client {
	c := &Client{
		baseURL:     strings.TrimRight(baseURL, "/"),
		httpClient:  http.DefaultClient,
		baseBackoff: 100 * time.Millisecond,
	}
	c.connState.Store(int32(Disconnected))
	for _, o := range opts {
		o(c)
	}
	return c
}

// ConnectionState returns the current SSE connection state.
func (c *Client) ConnectionState() ConnectionState {
	return ConnectionState(c.connState.Load())
}

func (c *Client) setState(s ConnectionState) {
	c.connState.Store(int32(s))
	if c.stateCallback != nil {
		c.stateCallback(s)
	}
}

// ── REST methods ──────────────────────────────────────────────────────

func (c *Client) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("create request for %s: %w", path, err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %d for %s", resp.StatusCode, path)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response for %s: %w", path, err)
	}
	return nil
}

func (c *Client) Pairs(ctx context.Context) ([]PairStatus, error) {
	var out []PairStatus
	err := c.get(ctx, "/api/pairs", &out)
	return out, err
}

func (c *Client) Debates(ctx context.Context) ([]DebateMeta, error) {
	var out []DebateMeta
	err := c.get(ctx, "/api/debates", &out)
	return out, err
}

func (c *Client) Debate(ctx context.Context, id string) (*DebateWithRounds, error) {
	var out DebateWithRounds
	err := c.get(ctx, "/api/debates/"+id, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) Plan(ctx context.Context) (*Plan, error) {
	var out Plan
	err := c.get(ctx, "/api/plan", &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) Status(ctx context.Context) (*StatusInfo, error) {
	var out StatusInfo
	err := c.get(ctx, "/api/status", &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) Scores(ctx context.Context, pair string) ([]ScoreEntry, error) {
	path := "/api/scores"
	if pair != "" {
		path += "?pair=" + url.QueryEscape(pair)
	}
	var out []ScoreEntry
	err := c.get(ctx, path, &out)
	return out, err
}

func (c *Client) Health(ctx context.Context) (*HealthStatus, error) {
	var out HealthStatus
	err := c.get(ctx, "/health", &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// ── SSE ───────────────────────────────────────────────────────────────

// Subscribe connects to the /events SSE endpoint and returns a channel of
// parsed events. The channel is closed when ctx is cancelled. On server
// disconnect the client reconnects with exponential backoff and sends the
// Last-Event-ID header.
func (c *Client) Subscribe(ctx context.Context) (<-chan SSEEvent, error) {
	ch := make(chan SSEEvent, 64)
	go c.sseLoop(ctx, ch)
	return ch, nil
}

func (c *Client) sseLoop(ctx context.Context, ch chan<- SSEEvent) {
	defer func() {
		c.setState(Disconnected)
		close(ch)
	}()

	attempt := 0
	for {
		if ctx.Err() != nil {
			return
		}

		if attempt > 0 {
			c.setState(Reconnecting)
			backoff := c.baseBackoff * (1 << (attempt - 1))
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
		}

		receivedEvents, err := c.sseConnect(ctx, ch)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			slog.Debug("SSE connection error", "attempt", attempt, "error", err)
		}
		if receivedEvents {
			attempt = 0 // reset backoff after a successful connection
		}
		attempt++
	}
}

// sseConnect opens a single SSE connection, parses events, and returns when
// the connection drops. Returns true if at least one event was received.
func (c *Client) sseConnect(ctx context.Context, ch chan<- SSEEvent) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/events", nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Accept", "text/event-stream")

	c.mu.Lock()
	lastID := c.lastEventID
	c.mu.Unlock()
	if lastID != "" {
		req.Header.Set("Last-Event-ID", lastID)
	}

	// Build an HTTP client that sets Connected state when the TCP dial
	// completes. This ensures the state transition happens before the server
	// handler can signal any channel (since the handler doesn't run until
	// after the connection is accepted and the request is read).
	transport := c.resolveTransport()
	dialConnected := c.wrapTransportDial(transport)

	sseHTTP := &http.Client{
		Transport: dialConnected,
		Timeout:   c.httpClient.Timeout,
	}

	resp, err := sseHTTP.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	receivedEvents := false
	scanner := bufio.NewScanner(resp.Body)
	var (
		eventType string
		eventID   string
		dataParts []string
	)

	for scanner.Scan() {
		if ctx.Err() != nil {
			return receivedEvents, ctx.Err()
		}
		line := scanner.Text()

		if line == "" {
			if len(dataParts) > 0 {
				ev := SSEEvent{
					ID:   eventID,
					Type: eventType,
					Data: []byte(strings.Join(dataParts, "\n")),
				}
				if eventID != "" {
					c.mu.Lock()
					c.lastEventID = eventID
					c.mu.Unlock()
				}
				select {
				case ch <- ev:
					receivedEvents = true
				case <-ctx.Done():
					return receivedEvents, ctx.Err()
				}
			}
			eventType = ""
			eventID = ""
			dataParts = nil
			continue
		}

		if strings.HasPrefix(line, ":") {
			continue
		}

		field, value, _ := strings.Cut(line, ":")
		value = strings.TrimPrefix(value, " ")

		switch field {
		case "event":
			eventType = value
		case "id":
			eventID = value
		case "data":
			dataParts = append(dataParts, value)
		}
	}

	return receivedEvents, scanner.Err()
}

// resolveTransport returns the underlying http.RoundTripper from the client.
func (c *Client) resolveTransport() *http.Transport {
	if c.httpClient.Transport != nil {
		if t, ok := c.httpClient.Transport.(*http.Transport); ok {
			return t.Clone()
		}
	}
	return http.DefaultTransport.(*http.Transport).Clone()
}

// wrapTransportDial returns a transport whose DialContext sets the client to
// Connected as soon as the TCP connection is established. The server's handler
// only starts running after the connection is accepted and the request is read,
// so by the time the handler can signal anything (like closing a channel), the
// Connected state is already set.
func (c *Client) wrapTransportDial(t *http.Transport) *http.Transport {
	origDial := t.DialContext
	if origDial == nil {
		var d net.Dialer
		origDial = d.DialContext
	}
	t.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		conn, err := origDial(ctx, network, addr)
		if err != nil {
			return nil, err
		}
		c.setState(Connected)
		return conn, nil
	}
	return t
}

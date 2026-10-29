// Package chat connects the chat window (kairo chat) to the running daemon
// over a unix socket. The daemon runs the agent, so background jobs keep
// running when the window is closed.
package chat

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"

	"kairo/internal/config"
)

// Event kinds: "step" (agent progress), "ask" (approval waiting, ID set),
// "asked" (no approval waiting), "notice" (reminder, job result, alert).
type Event struct {
	Kind string `json:"kind"`
	ID   string `json:"id,omitempty"`
	Text string `json:"text,omitempty"`
}

func SocketPath() string {
	return config.Path("kairo.sock")
}

type Hub struct {
	mu   sync.Mutex
	subs map[chan Event]struct{}
}

func (h *Hub) Subscribe() (chan Event, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.subs == nil {
		h.subs = map[chan Event]struct{}{}
	}
	ch := make(chan Event, 64)
	h.subs[ch] = struct{}{}
	return ch, func() {
		h.mu.Lock()
		delete(h.subs, ch)
		h.mu.Unlock()
	}
}

// Publish never blocks: a window that falls behind misses events.
func (h *Hub) Publish(e Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- e:
		default:
		}
	}
}

type Client struct {
	http *http.Client
}

func NewClient() *Client {
	return &Client{http: &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", SocketPath())
		},
	}}}
}

var ErrNoDaemon = errors.New("Kairo is not running. Start it with: systemctl --user start kairo")

func (c *Client) post(ctx context.Context, path string, body, out any) error {
	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", "http://kairo"+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() == nil {
			return ErrNoDaemon
		}
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		msg, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("kairo: %s", bytes.TrimSpace(msg))
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

func (c *Client) Send(ctx context.Context, text string) (string, error) {
	var out struct {
		Reply string `json:"reply"`
	}
	err := c.post(ctx, "/chat", map[string]string{"text": text}, &out)
	return out.Reply, err
}

func (c *Client) Answer(ctx context.Context, id, answer string) error {
	return c.post(ctx, "/answer", map[string]string{"id": id, "answer": answer}, nil)
}

func (c *Client) Stop(ctx context.Context) error {
	return c.post(ctx, "/stop", struct{}{}, nil)
}

// Events streams daemon events until ctx ends or the daemon goes away.
func (c *Client) Events(ctx context.Context, on func(Event)) error {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://kairo/events", nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return ErrNoDaemon
	}
	defer resp.Body.Close()
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	for scanner.Scan() {
		var e Event
		if json.Unmarshal(scanner.Bytes(), &e) == nil {
			on(e)
		}
	}
	return scanner.Err()
}

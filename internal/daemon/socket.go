package daemon

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"

	"kairo/internal/agent"
	"kairo/internal/app"
	"kairo/internal/chat"
)

// serveChat answers the chat window on ~/.kairo/kairo.sock (owner-only).
func serveChat(ctx context.Context, a *app.App) {
	path := chat.SocketPath()
	os.Remove(path)
	ln, err := net.Listen("unix", path)
	if err != nil {
		a.Store.Log("error", "chat socket: "+err.Error())
		return
	}
	os.Chmod(path, 0o600)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /chat", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Text string `json:"text"`
		}
		if json.NewDecoder(r.Body).Decode(&in) != nil || in.Text == "" {
			http.Error(w, "empty message", 400)
			return
		}
		// Closing the window must not cancel the request; the reply is kept
		// in the chat history either way.
		reply := a.Agent.Handle(ctx, agent.Input{Text: in.Text, Origin: "chat"})
		json.NewEncoder(w).Encode(map[string]string{"reply": reply})
	})
	mux.HandleFunc("POST /answer", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			ID     string `json:"id"`
			Answer string `json:"answer"`
		}
		json.NewDecoder(r.Body).Decode(&in)
		if !a.Approvals.Resolve(in.ID, in.Answer) {
			http.Error(w, "that question is no longer waiting", 410)
		}
	})
	mux.HandleFunc("POST /stop", func(w http.ResponseWriter, r *http.Request) {
		a.StopAll()
	})
	mux.HandleFunc("GET /events", func(w http.ResponseWriter, r *http.Request) {
		events, unsubscribe := a.Events.Subscribe()
		defer unsubscribe()
		flusher, _ := w.(http.Flusher)
		enc := json.NewEncoder(w)
		enc.Encode(chat.Event{Kind: "hello"})
		if card := a.CurrentCard(); card != nil {
			enc.Encode(chat.Event{Kind: "ask", ID: card.ID, Text: card.Question})
		}
		if flusher != nil {
			flusher.Flush()
		}
		for {
			select {
			case e := <-events:
				if enc.Encode(e) != nil {
					return
				}
				if flusher != nil {
					flusher.Flush()
				}
			case <-r.Context().Done():
				return
			case <-ctx.Done():
				return
			}
		}
	})
	server := &http.Server{Handler: mux}
	go func() {
		<-ctx.Done()
		server.Close()
		os.Remove(path)
	}()
	server.Serve(ln)
}

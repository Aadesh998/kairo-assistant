package google

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/endpoints"

	"kairo/internal/config"
)

func oauthConfig(redirect string) (*oauth2.Config, error) {
	id, secret := os.Getenv("GOOGLE_CLIENT_ID"), os.Getenv("GOOGLE_CLIENT_SECRET")
	if id == "" || secret == "" {
		return nil, errors.New("set GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET in ~/.kairo/.env")
	}
	return &oauth2.Config{
		ClientID:     id,
		ClientSecret: secret,
		Endpoint:     endpoints.Google,
		Scopes:       []string{"https://mail.google.com/", "https://www.googleapis.com/auth/calendar.events", "https://www.googleapis.com/auth/drive.file"},
		RedirectURL:  redirect,
	}, nil
}

func Login(ctx context.Context, browserBin, profileDir string) error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	redirect := fmt.Sprintf("http://%s/callback", listener.Addr())
	cfg, err := oauthConfig(redirect)
	if err != nil {
		return err
	}
	stateBytes := make([]byte, 16)
	rand.Read(stateBytes)
	state := hex.EncodeToString(stateBytes)
	url := cfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	fmt.Println("Opening Google sign-in in " + browserBin + ". If it does not open, paste this into Chrome:\n" + url)
	if err := exec.Command(browserBin, "--user-data-dir="+profileDir, "--new-window", url).Start(); err != nil {
		exec.Command("xdg-open", url).Start()
	}

	result := make(chan error, 1)
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			return
		}
		token, err := cfg.Exchange(ctx, r.URL.Query().Get("code"))
		if err == nil {
			err = saveToken(token)
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			fmt.Fprintln(w, "Kairo is connected to your Gmail and Google Calendar. You can close this tab.")
		}
		result <- err
	})}
	go server.Serve(listener)
	defer server.Close()
	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

type savingSource struct {
	src  oauth2.TokenSource
	last string
}

func (s *savingSource) Token() (*oauth2.Token, error) {
	t, err := s.src.Token()
	if err == nil && t.AccessToken != s.last {
		s.last = t.AccessToken
		saveToken(t)
	}
	return t, err
}

func TokenSource(ctx context.Context) (oauth2.TokenSource, error) {
	data, err := os.ReadFile(config.Path("google-token.json"))
	if err != nil {
		return nil, errors.New("not signed in to Google, run: kairo google login")
	}
	var token oauth2.Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, err
	}
	cfg, err := oauthConfig("")
	if err != nil {
		return nil, err
	}
	return &savingSource{src: cfg.TokenSource(ctx, &token), last: token.AccessToken}, nil
}

func saveToken(t *oauth2.Token) error {
	data, err := json.Marshal(t)
	if err != nil {
		return err
	}
	return os.WriteFile(config.Path("google-token.json"), data, 0o600)
}

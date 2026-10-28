package tools

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"

	"kairo/internal/config"
)

var (
	browserMu sync.Mutex
	browser   *rod.Browser
)

func browserTools(cfg config.Browser) []Tool {
	return []Tool{
		{
			Name:        "browser_read",
			Description: "Open a web page in Kairo's own browser and return its visible text.",
			Params:      map[string]any{"url": prop("string", "Page URL")},
			Required:    []string{"url"},
			Run: func(ctx context.Context, in map[string]any) (Result, error) {
				page, err := OpenPage(cfg, str(in, "url"))
				if err != nil {
					return Result{}, err
				}
				page = page.Timeout(30 * time.Second)
				if err := page.WaitLoad(); err != nil {
					return Result{}, err
				}
				body, err := page.Element("body")
				if err != nil {
					return Result{}, err
				}
				text, err := body.Text()
				if len(text) > 15000 {
					text = text[:15000]
				}
				return Result{Text: text}, err
			},
		},
		{
			Name:        "join_meeting",
			Description: "Join a Google Meet meeting with microphone and camera turned off.",
			Params:      map[string]any{"url": prop("string", "Meeting URL")},
			Required:    []string{"url"},
			Run: func(ctx context.Context, in map[string]any) (Result, error) {
				joined, err := JoinMeeting(ctx, cfg, str(in, "url"))
				if err != nil {
					return Result{}, err
				}
				if !joined {
					return Result{Text: "opened the meeting in the owner's browser profile with mic and camera off; the owner clicks Join now"}, nil
				}
				return Result{Text: "joined meeting"}, nil
			},
		},
	}
}

func OpenPage(cfg config.Browser, url string) (*rod.Page, error) {
	browserMu.Lock()
	defer browserMu.Unlock()
	page, err := openPage(cfg, url)
	if err != nil && browser != nil {
		browser = nil
		page, err = openPage(cfg, url)
	}
	return page, err
}

func openPage(cfg config.Browser, url string) (*rod.Page, error) {
	if browser == nil {
		l := launcher.New().
			Bin(cfg.Bin).
			UserDataDir(cfg.ProfileDir).
			Headless(false).
			Leakless(false).
			Set("use-fake-ui-for-media-stream")
		if cfg.PasswordStore != "" {
			l = l.Set("password-store", cfg.PasswordStore)
		}
		u, err := l.Launch()
		if err != nil {
			return nil, err
		}
		browser = rod.New().ControlURL(u)
		if err := browser.Connect(); err != nil {
			browser = nil
			return nil, err
		}
	}
	return browser.Page(proto.TargetCreateTarget{URL: url})
}

func JoinMeeting(ctx context.Context, cfg config.Browser, url string) (bool, error) {
	if cfg.Mode == "existing" {
		return false, openInProfile(ctx, cfg, url)
	}
	return true, autoJoin(ctx, cfg, url)
}

func openInProfile(ctx context.Context, cfg config.Browser, url string) error {
	if err := OpenInBrowser(cfg, url); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(8 * time.Second):
	}
	hyprDispatch(ctx, `hl.dsp.focus({ window = "title:^Meet" })`)
	time.Sleep(500 * time.Millisecond)
	if !meetFocused(ctx) {
		return nil
	}
	run(ctx, 5*time.Second, "wtype", keyArgs("ctrl+d")...)
	run(ctx, 5*time.Second, "wtype", keyArgs("ctrl+e")...)
	return nil
}

func meetFocused(ctx context.Context) bool {
	out, err := run(ctx, 5*time.Second, "hyprctl", "activewindow", "-j")
	if err != nil {
		return false
	}
	var active struct {
		Title string `json:"title"`
	}
	json.Unmarshal([]byte(out), &active)
	return strings.HasPrefix(active.Title, "Meet")
}

func autoJoin(ctx context.Context, cfg config.Browser, url string) error {
	page, err := OpenPage(cfg, url)
	if err != nil {
		return err
	}
	page = page.Context(ctx).Timeout(60 * time.Second)
	if err := page.WaitLoad(); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(4 * time.Second):
	}
	page.Keyboard.Press(input.ControlLeft)
	page.Keyboard.Type(input.KeyD)
	page.Keyboard.Type(input.KeyE)
	page.Keyboard.Release(input.ControlLeft)
	button, err := page.ElementR("button", "Join now|Ask to join|Join")
	if err != nil {
		return err
	}
	return button.Click("left", 1)
}

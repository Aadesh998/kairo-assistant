package tools

import (
	"context"
	"os/exec"
	"strconv"
	"time"

	"kairo/internal/config"
)

func appTools(browser config.Browser) []Tool {
	return []Tool{
		{
			Name:        "open_app",
			Description: "Launch a desktop application by its command, for example slack, google-chrome-stable, nautilus, spotify.",
			Params:      map[string]any{"command": prop("string", "Application command")},
			Required:    []string{"command"},
			Run: func(ctx context.Context, in map[string]any) (Result, error) {
				out, err := hyprDispatch(ctx, "hl.dsp.exec_cmd("+luaString(str(in, "command"))+")")
				return Result{Text: "launched: " + out}, err
			},
		},
		{
			Name:        "open_url",
			Description: "Open a URL in the owner's work browser profile (Gmail, Calendar, Meet, any website).",
			Params:      map[string]any{"url": prop("string", "URL to open")},
			Required:    []string{"url"},
			Run: func(ctx context.Context, in map[string]any) (Result, error) {
				return Result{Text: "opened " + str(in, "url")}, OpenInBrowser(browser, str(in, "url"))
			},
		},
		{
			Name:        "desktop_notify",
			Description: "Show a desktop notification on the owner's screen.",
			Params:      map[string]any{"text": prop("string", "Notification text")},
			Required:    []string{"text"},
			Run: func(ctx context.Context, in map[string]any) (Result, error) {
				_, err := run(ctx, 5*time.Second, "notify-send", "Kairo", str(in, "text"))
				return Result{Text: "notified"}, err
			},
		},
	}
}

func OpenInBrowser(browser config.Browser, url string) error {
	if browser.Mode != "existing" {
		return exec.Command("xdg-open", url).Start()
	}
	cmd := exec.Command(browser.Bin, "--profile-directory="+browser.ProfileDirectory, url)
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}

func hyprDispatch(ctx context.Context, lua string) (string, error) {
	return run(ctx, 10*time.Second, "hyprctl", "dispatch", lua)
}

func luaString(s string) string {
	return strconv.Quote(s)
}

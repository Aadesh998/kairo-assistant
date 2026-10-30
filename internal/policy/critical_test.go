package policy

import "testing"

func TestCritical(t *testing.T) {
	Protect(nil)
	cases := []struct {
		tool string
		in   map[string]any
		want bool
	}{
		{"type_text", map[string]any{"text": "Hey Rahul, running 10 min late"}, false},
		{"press_keys", map[string]any{"keys": "Return"}, false},
		{"create_event", map[string]any{"title": "Sync with team"}, false},
		{"send_file", map[string]any{"path": "~/Documents/report.pdf"}, false},
		{"type_text", map[string]any{"text": "the wifi password is hunter2"}, true},
		{"type_text", map[string]any{"text": "here is the api key sk-abcdefghijklmnop"}, true},
		{"send_file", map[string]any{"path": "~/project/.env"}, true},
		{"run_command", map[string]any{"command": "make build"}, true},
	}
	for _, c := range cases {
		if got := Critical(c.tool, c.in); got != c.want {
			t.Errorf("Critical(%s, %v) = %v, want %v", c.tool, c.in, got, c.want)
		}
	}
}

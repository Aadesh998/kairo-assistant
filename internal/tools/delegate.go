package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"kairo/internal/config"
	"kairo/internal/policy"
	"kairo/internal/store"
)

func delegateTools(cfg *config.Config, s *store.Store, notify func(string)) []Tool {
	var names []string
	for _, d := range cfg.Delegates {
		names = append(names, d.Name)
	}
	return []Tool{
		{
			Name: "delegate_task",
			Description: "Hand a coding or technical task to a worker agent. Use this for ANY code writing, fixing, refactoring, scripting, debugging, tests or repository work. " +
				"Kairo never does that work itself. The worker runs in the background and the owner is messaged when it finishes. Available workers: " + strings.Join(names, ", "),
			Params: map[string]any{
				"task":  prop("string", "Complete, self-contained instructions for the worker, including the goal and how to check the result"),
				"dir":   prop("string", "Project directory to work in, ~ allowed"),
				"agent": prop("string", "Worker name, empty for the default"),
			},
			Required: []string{"task", "dir"},
			Run: func(ctx context.Context, in map[string]any) (Result, error) {
				d, ok := findDelegate(cfg, str(in, "agent"))
				if !ok {
					return Result{}, fmt.Errorf("unknown worker %q, use one of: %s", str(in, "agent"), strings.Join(names, ", "))
				}
				dir := config.Expand(str(in, "dir"))
				if info, err := os.Stat(dir); err != nil || !info.IsDir() {
					return Result{}, fmt.Errorf("directory %s does not exist", dir)
				}
				id := time.Now().Format("0102-150405")
				logPath := config.Path("delegations/" + id + ".log")
				os.MkdirAll(config.Path("delegations"), 0o700)
				s.Log("delegate", fmt.Sprintf("%s started %s in %s: %s", d.Name, id, dir, firstLine(str(in, "task"))))
				go runDelegate(d, str(in, "task"), dir, logPath, s, notify)
				return Result{Text: fmt.Sprintf("Job %s started with %s in %s. The owner will be messaged when it finishes. Log: %s", id, d.Name, dir, logPath)}, nil
			},
		},
	}
}

func WorkerRules() string {
	return "\n\nRules you must follow while doing this task (set by the owner, not negotiable):\n" +
		"- Work only inside the given directory.\n" +
		"- Never open, print, copy or send secrets: .env files, *.pem, *.key, credentials, tokens, SSH keys, or these paths: " + strings.Join(policy.ProtectedList(), ", ") + "\n" +
		"- Never deploy, push to a remote, publish packages, run database migrations on shared databases, or touch production servers.\n" +
		"- Never run destructive commands (rm -rf, git reset --hard, force push, dropping data).\n" +
		"- Never send emails or messages to anyone.\n" +
		"- If the task needs any of the above, stop and explain what the owner must do instead.\n"
}

func findDelegate(cfg *config.Config, name string) (config.Delegate, bool) {
	if name == "" {
		name = cfg.DefaultDelegate
	}
	for _, d := range cfg.Delegates {
		if d.Name == name {
			return d, true
		}
	}
	return config.Delegate{}, false
}

func runDelegate(d config.Delegate, task, dir, logPath string, s *store.Store, notify func(string)) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()
	task += WorkerRules()
	var args []string
	for _, a := range d.Command {
		args = append(args, strings.ReplaceAll(a, "{task}", task))
	}
	if len(args) == 0 {
		return
	}
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	os.WriteFile(logPath, out, 0o600)
	status := "✅ finished"
	if err != nil {
		status = "❌ failed (" + err.Error() + ")"
	}
	result := workerResult(out, err)
	s.Log("delegate", fmt.Sprintf("%s %s: %s", d.Name, status, firstLine(result)))
	text := fmt.Sprintf("🛠 %s %s\nTask: %s\nResult: %s\nLog: %s", d.Name, status, firstLine(task), result, logPath)
	if notify != nil {
		notify(text)
	}
	exec.Command("notify-send", "-a", "Kairo", "🛠 "+d.Name+" "+status, firstLine(result)).Run()
}

func workerResult(out []byte, err error) string {
	var parsed struct {
		Result string `json:"result"`
	}
	text := strings.TrimSpace(string(out))
	if start := strings.Index(text, "{"); start >= 0 && json.Unmarshal([]byte(text[start:]), &parsed) == nil && parsed.Result != "" {
		text = parsed.Result
	}
	if text == "" && err != nil {
		text = errors.Unwrap(err).Error()
	}
	if len(text) > 1200 {
		text = "…" + text[len(text)-1200:]
	}
	return text
}

func firstLine(text string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(text), "\n")
	if len(line) > 120 {
		line = line[:120] + "…"
	}
	return line
}

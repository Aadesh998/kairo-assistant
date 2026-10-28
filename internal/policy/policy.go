package policy

import (
	"regexp"
	"strings"
)

type Level string

const (
	Safe  Level = "safe"
	Ask   Level = "ask"
	Never Level = "never"
)

var toolLevels = map[string]Level{
	"read_file":            Safe,
	"list_dir":             Safe,
	"list_windows":         Safe,
	"focus_window":         Safe,
	"switch_workspace":     Safe,
	"screenshot":           Safe,
	"open_url":             Safe,
	"open_app":             Safe,
	"desktop_notify":       Safe,
	"browser_read":         Safe,
	"add_task":             Safe,
	"list_tasks":           Safe,
	"complete_task":        Safe,
	"remember":             Safe,
	"recall":               Safe,
	"recent_activity":      Safe,
	"recent_mail":          Safe,
	"calendar_events":      Safe,
	"create_event":         Ask,
	"delegate_task":        Ask,
	"schedule_join":        Ask,
	"schedule_instruction": Safe,
	"list_scheduled":       Safe,
	"cancel_scheduled":     Safe,
	"stop_running":         Safe,
	"use_skill":            Safe,
	"send_file":            Ask,
	"send_update":          Safe,
	"run_command":          Ask,
	"type_text":            Ask,
	"press_keys":           Ask,
	"join_meeting":         Ask,
}

var neverPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(^|\s|;|&|\|)sudo\s`),
	regexp.MustCompile(`rm\s+-[a-zA-Z]*r[a-zA-Z]*f?\s+(/|~|\$HOME)\s*($|;|&)`),
	regexp.MustCompile(`rm\s+-[a-zA-Z]*f[a-zA-Z]*r?\s+(/|~|\$HOME)\s*($|;|&)`),
	regexp.MustCompile(`\bmkfs\b`),
	regexp.MustCompile(`\bdd\s+.*of=/dev/`),
	regexp.MustCompile(`\b(shutdown|reboot|poweroff)\b`),
	regexp.MustCompile(`:\(\)\s*\{`),
	regexp.MustCompile(`\.ssh/`),
	regexp.MustCompile(`curl[^|]*\|\s*(ba)?sh`),
	regexp.MustCompile(`wget[^|]*\|\s*(ba)?sh`),
	regexp.MustCompile(`\bchmod\s+-R\s+777\s+/`),
	regexp.MustCompile(`\brm\s+-[a-zA-Z]*r`),
	regexp.MustCompile(`\bgit\s+push\b.*(--force|\s-f\b)`),
	regexp.MustCompile(`\bgit\s+(reset\s+--hard|clean\s+-[a-z]*f)`),
	regexp.MustCompile(`\b(kubectl|terraform|pulumi|helm|eksctl)\b`),
	regexp.MustCompile(`\b(aws|gcloud|az|doctl|flyctl|vercel|netlify|wrangler)\s`),
	regexp.MustCompile(`\b(ssh|scp|sftp|rsync)\s.*@`),
	regexp.MustCompile(`\b(npm|pnpm|yarn)\s+publish\b|\bdocker\s+push\b`),
	regexp.MustCompile(`(?i)\b(drop\s+(table|database|schema)|truncate\s+table|delete\s+from)\b`),
	regexp.MustCompile(`\b(mail|sendmail|mutt|msmtp)\s`),
	regexp.MustCompile(`\b(env|printenv|export)\s*($|\|)`),
}

var safeCommands = map[string]bool{
	"ls": true, "cat": true, "pwd": true, "date": true, "whoami": true, "uptime": true,
	"df": true, "du": true, "free": true, "echo": true, "head": true, "tail": true,
	"wc": true, "grep": true, "find": true, "which": true, "ps": true, "uname": true,
}

var safeGit = map[string]bool{"status": true, "log": true, "diff": true, "branch": true, "show": true}

func Check(tool string, in map[string]any) (Level, string) {
	text := func(key string) string {
		v, _ := in[key].(string)
		return v
	}
	switch tool {
	case "run_command":
		if Protected(text("command")) {
			return Never, "run_command"
		}
		return checkCommand(text("command"))
	case "delegate_task":
		if Protected(text("dir")) || Protected(text("task")) {
			return Never, tool
		}
		return Ask, "delegate_task:" + text("agent") + ":" + text("dir")
	case "send_file":
		if Protected(text("path")) {
			return Never, tool
		}
		return Ask, tool
	case "read_file", "list_dir":
		path := text("path")
		if Protected(path) || strings.HasPrefix(path, "/etc") || strings.HasPrefix(path, "/boot") {
			return Never, tool
		}
	case "browser_read", "open_url":
		if strings.HasPrefix(strings.ToLower(text("url")), "file:") {
			return Never, tool
		}
	}
	level, ok := toolLevels[tool]
	if !ok {
		return Ask, tool
	}
	return level, tool
}

func checkCommand(command string) (Level, string) {
	for _, p := range neverPatterns {
		if p.MatchString(command) {
			return Never, "run_command"
		}
	}
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return Never, "run_command"
	}
	action := "run_command:" + fields[0]
	if len(fields) > 1 && !strings.HasPrefix(fields[1], "-") && !strings.ContainsAny(fields[1], "/.~") {
		action += " " + fields[1]
	}
	if strings.ContainsAny(command, ";&`>") || strings.Contains(command, "$(") {
		return Ask, action
	}
	if safeCommands[fields[0]] {
		return Safe, action
	}
	if fields[0] == "git" && len(fields) > 1 && safeGit[fields[1]] {
		return Safe, action
	}
	return Ask, action
}

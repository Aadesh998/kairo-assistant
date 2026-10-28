package policy

import (
	"regexp"
	"strings"

	"kairo/internal/config"
)

var corePaths = []string{
	"~/.ssh",
	"~/.gnupg",
	"~/.aws",
	"~/.kube",
	"~/.docker/config.json",
	"~/.kairo/.env",
	"~/.kairo/google-token.json",
	"~/.kairo/whatsapp.db",
	"~/.config/BraveSoftware",
	"~/.config/google-chrome",
	"/etc/shadow",
}

var secretFile = regexp.MustCompile(`(?i)(\.pem\b|\.key\b|\.p12\b|\.pfx\b|id_rsa|id_ed25519|(^|[/\s])\.env(\.[a-z]+)?\b|credentials?\.json|token\.json|service[-_]account)`)

var protectedPaths []string

func Protect(paths []string) {
	protectedPaths = nil
	for _, p := range append(append([]string{}, corePaths...), paths...) {
		protectedPaths = append(protectedPaths, strings.TrimRight(config.Expand(p), "/"))
	}
}

func Protected(text string) bool {
	expanded := strings.ReplaceAll(text, "~/", config.Expand("~/")+"/")
	for _, p := range protectedPaths {
		if p != "" && strings.Contains(expanded, p) {
			return true
		}
	}
	return secretFile.MatchString(text)
}

func ProtectedList() []string {
	return protectedPaths
}

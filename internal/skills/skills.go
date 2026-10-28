package skills

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"kairo/internal/config"
)

type Skill struct {
	Name        string
	Description string
	Path        string
}

type Library struct {
	list []Skill
}

func Load(cfg config.Skills) *Library {
	lib := &Library{}
	seen := map[string]bool{}
	for _, dir := range cfg.Paths {
		dir = config.Expand(dir)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			name := e.Name()
			if seen[name] || (len(cfg.Allow) > 0 && !slices.Contains(cfg.Allow, name)) {
				continue
			}
			path := filepath.Join(dir, name, "SKILL.md")
			description, ok := readDescription(path)
			if !ok {
				continue
			}
			seen[name] = true
			lib.list = append(lib.list, Skill{Name: name, Description: description, Path: path})
		}
	}
	return lib
}

func (l *Library) Index() string {
	var b strings.Builder
	for _, s := range l.list {
		description := s.Description
		if len(description) > 220 {
			description = description[:220] + "…"
		}
		b.WriteString("- " + s.Name + ": " + description + "\n")
	}
	return b.String()
}

func (l *Library) Body(name string) (string, error) {
	for _, s := range l.list {
		if s.Name == name {
			data, err := os.ReadFile(s.Path)
			return string(data), err
		}
	}
	return "", errors.New("no skill named " + name)
}

func readDescription(path string) (string, bool) {
	f, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	inFront := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			if inFront {
				break
			}
			inFront = true
			continue
		}
		if inFront && strings.HasPrefix(line, "description:") {
			value := strings.TrimSpace(strings.TrimPrefix(line, "description:"))
			value = strings.Trim(value, `"'`)
			if value == ">-" || value == "|" || value == "" {
				if scanner.Scan() {
					value = strings.TrimSpace(scanner.Text())
				}
			}
			return value, true
		}
	}
	return "", false
}

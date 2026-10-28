package persona

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"kairo/internal/config"
)

type Persona struct {
	Owner      string    `yaml:"owner"`
	Role       string    `yaml:"role"`
	Company    string    `yaml:"company"`
	About      string    `yaml:"about"`
	Greeting   string    `yaml:"greeting"`
	WorkStyle  []string  `yaml:"work_style"`
	Projects   []Project `yaml:"projects"`
	Boundaries []string  `yaml:"boundaries"`
	Routine    []Routine `yaml:"routine"`
}

type Project struct {
	Name  string `yaml:"name"`
	Path  string `yaml:"path"`
	Notes string `yaml:"notes"`
}

type Routine struct {
	At               string   `yaml:"at"`
	Days             []string `yaml:"days"`
	Do               string   `yaml:"do"`
	Title            string   `yaml:"title"`
	MeetingURL       string   `yaml:"meeting_url"`
	Prompt           string   `yaml:"prompt"`
	AskBefore        bool     `yaml:"ask_before_join"`
	AskMinutesBefore int      `yaml:"ask_minutes_before"`
	WaitSeconds      int      `yaml:"wait_for_answer_seconds"`
	IfNoAnswer       string   `yaml:"if_no_answer"`
}

func Load() (*Persona, error) {
	p := &Persona{}
	data, err := os.ReadFile(config.Path("persona.yaml"))
	if err != nil {
		data = []byte(config.DefaultPersona)
	}
	if err := yaml.Unmarshal(data, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (r Routine) TriggerTime() string {
	at, err := time.Parse("15:04", r.At)
	if err != nil {
		return r.At
	}
	if r.Do == "join_meeting" {
		at = at.Add(-time.Duration(r.AskMinutesBefore) * time.Minute)
	}
	return at.Format("15:04")
}

func (r Routine) Due(now time.Time) bool {
	day := strings.ToLower(now.Weekday().String()[:3])
	if len(r.Days) > 0 && !slices.Contains(r.Days, day) {
		return false
	}
	return now.Format("15:04") == r.TriggerTime()
}

func (r Routine) Label() string {
	if r.Title != "" {
		return r.Title
	}
	return r.Do
}

func (p *Persona) Summary() string {
	var b strings.Builder
	if p.Role != "" || p.Company != "" {
		b.WriteString(fmt.Sprintf("Role: %s at %s\n", p.Role, p.Company))
	}
	if p.About != "" {
		b.WriteString(strings.TrimSpace(p.About) + "\n")
	}
	if len(p.Projects) > 0 {
		b.WriteString("Projects:\n")
		for _, pr := range p.Projects {
			b.WriteString(fmt.Sprintf("- %s (%s): %s\n", pr.Name, pr.Path, pr.Notes))
		}
	}
	b.WriteString("Work style:\n")
	for _, s := range p.WorkStyle {
		b.WriteString("- " + s + "\n")
	}
	b.WriteString("Daily routine:\n")
	for _, r := range p.Routine {
		b.WriteString(fmt.Sprintf("- %s on %s: %s %s\n", r.At, strings.Join(r.Days, ","), r.Do, r.Title))
	}
	return b.String()
}

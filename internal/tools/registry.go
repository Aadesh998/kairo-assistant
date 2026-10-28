package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"

	"kairo/internal/calendar"
	"kairo/internal/config"
	"kairo/internal/skills"
	"kairo/internal/store"
)

type Result struct {
	Text  string
	Image []byte
}

type Tool struct {
	Name        string
	Description string
	Params      map[string]any
	Required    []string
	Run         func(ctx context.Context, in map[string]any) (Result, error)
}

type Deps struct {
	Config   *config.Config
	Store    *store.Store
	Calendar *calendar.Client
	Notify   func(text string)
	StopJobs func() int
	Skills   *skills.Library
	SendFile func(ctx context.Context, path, caption string) (string, error)
}

type Registry struct {
	tools map[string]Tool
	order []string
}

func New(d Deps) *Registry {
	r := &Registry{tools: map[string]Tool{}}
	for _, t := range shellTools() {
		r.Add(t)
	}
	for _, t := range fileTools() {
		r.Add(t)
	}
	for _, t := range appTools(d.Config.Browser) {
		r.Add(t)
	}
	for _, t := range windowTools() {
		r.Add(t)
	}
	for _, t := range keyboardTools() {
		r.Add(t)
	}
	for _, t := range screenTools() {
		r.Add(t)
	}
	for _, t := range browserTools(d.Config.Browser) {
		r.Add(t)
	}
	for _, t := range taskTools(d.Store) {
		r.Add(t)
	}
	for _, t := range fileSendTools(d.SendFile) {
		r.Add(t)
	}
	for _, t := range skillTools(d.Skills) {
		r.Add(t)
	}
	for _, t := range scheduleTools(d.Store, d.StopJobs) {
		r.Add(t)
	}
	for _, t := range delegateTools(d.Config, d.Store, d.Notify) {
		r.Add(t)
	}
	for _, t := range calendarTools(d.Store, d.Calendar) {
		r.Add(t)
	}
	for _, t := range mailTools(d.Store) {
		r.Add(t)
	}
	for _, t := range memoryTools(d.Store, d.Notify) {
		r.Add(t)
	}
	return r
}

func (r *Registry) Add(t Tool) {
	r.tools[t.Name] = t
	r.order = append(r.order, t.Name)
}

func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

func (r *Registry) Specs() []anthropic.BetaToolUnionParam {
	var specs []anthropic.BetaToolUnionParam
	for _, name := range r.order {
		t := r.tools[name]
		tool := anthropic.BetaToolParam{
			Name:        t.Name,
			Description: anthropic.String(t.Description),
			InputSchema: anthropic.BetaToolInputSchemaParam{Properties: t.Params, Required: t.Required},
		}
		specs = append(specs, anthropic.BetaToolUnionParam{OfTool: &tool})
	}
	return specs
}

func prop(kind, description string) map[string]any {
	return map[string]any{"type": kind, "description": description}
}

func str(in map[string]any, key string) string {
	v, _ := in[key].(string)
	return v
}

func num(in map[string]any, key string) int64 {
	v, _ := in[key].(float64)
	return int64(v)
}

func run(ctx context.Context, timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	text := strings.TrimSpace(string(out))
	if len(text) > 8000 {
		text = text[:8000] + "\n...(truncated)"
	}
	if err != nil {
		return text, fmt.Errorf("%v: %s", err, text)
	}
	return text, nil
}

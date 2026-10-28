package approval

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	Yes     = "yes"
	No      = "no"
	Later   = "later"
	Timeout = "timeout"
)

type Request struct {
	ID       string
	Action   string
	Question string
	Reason   string
	Wait     time.Duration
}

type Asker interface {
	AskYesNo(ctx context.Context, question string, wait time.Duration) string
}

type Broker struct {
	mu      sync.Mutex
	next    int
	pending map[string]chan string
	Voice   Asker
	Send    func(r Request)
	Wait    time.Duration
}

func NewBroker(wait time.Duration) *Broker {
	return &Broker{pending: map[string]chan string{}, Wait: wait}
}

func (b *Broker) Ask(ctx context.Context, r Request) string {
	b.mu.Lock()
	b.next++
	r.ID = fmt.Sprintf("a%d", b.next)
	answer := make(chan string, 3)
	b.pending[r.ID] = answer
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		delete(b.pending, r.ID)
		b.mu.Unlock()
	}()
	if r.Wait == 0 {
		r.Wait = b.Wait
	}
	if b.Send != nil {
		b.Send(r)
	}
	if b.Voice != nil {
		go func() {
			if a := b.Voice.AskYesNo(ctx, r.Question, r.Wait); a != Timeout {
				answer <- a
			}
		}()
	}
	select {
	case a := <-answer:
		return a
	case <-time.After(r.Wait):
		return Timeout
	case <-ctx.Done():
		return Timeout
	}
}

func (b *Broker) Resolve(id, answer string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch, ok := b.pending[id]
	if ok {
		ch <- answer
	}
	return ok
}

func (b *Broker) ResolveOnly(answer string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.pending) != 1 {
		return false
	}
	for _, ch := range b.pending {
		ch <- answer
	}
	return true
}

func (b *Broker) Pending() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.pending)
}

func ParseAnswer(text string) string {
	t := strings.ToLower(text)
	switch {
	case containsAny(t, "later", "wait", "not now", "snooze"):
		return Later
	case containsAny(t, "no", "don't", "dont", "skip", "deny", "cancel", "nope"):
		return No
	case containsAny(t, "yes", "yeah", "yep", "sure", "ok", "okay", "go ahead", "join", "do it", "approve"):
		return Yes
	}
	return ""
}

func containsAny(text string, words ...string) bool {
	for _, w := range strings.Fields(strings.NewReplacer(",", " ", ".", " ", "!", " ", "?", " ").Replace(text)) {
		for _, x := range words {
			if w == x {
				return true
			}
		}
	}
	for _, x := range words {
		if strings.Contains(x, " ") && strings.Contains(text, x) {
			return true
		}
	}
	return false
}

type Terminal struct{}

func (Terminal) AskYesNo(ctx context.Context, question string, wait time.Duration) string {
	fmt.Printf("\n%s [y/n] ", question)
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return Yes
	case "l", "later":
		return Later
	}
	return No
}

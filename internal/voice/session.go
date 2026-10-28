package voice

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"kairo/internal/approval"
	"kairo/internal/config"
)

type Handler func(ctx context.Context, text string) string

type Session struct {
	cfg      config.Voice
	ui       config.UI
	owner    string
	handle   Handler
	log      func(kind, text string)
	micMu    sync.Mutex
	busy     atomic.Bool
	awake    atomic.Bool
	wakeFlag atomic.Bool
	cancelMu sync.Mutex
	cancel   context.CancelFunc
	convMu   sync.Mutex
	stopConv context.CancelFunc
	noticeMu sync.Mutex
}

func NewSession(cfg *config.Config, handle Handler, log func(kind, text string)) *Session {
	if _, err := os.Stat(config.Path("chime.wav")); err != nil {
		WriteChime(config.Path("chime.wav"))
	}
	return &Session{
		cfg:    cfg.Voice,
		ui:     cfg.UI,
		owner:  cfg.Owner,
		handle: handle,
		log:    log,
	}
}

func (s *Session) Wake() {
	s.wakeFlag.Store(true)
	s.cancelMu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	s.cancelMu.Unlock()
}

func (s *Session) listen(ctx context.Context, o RecordOptions) []byte {
	ctx, cancel := context.WithCancel(ctx)
	s.cancelMu.Lock()
	s.cancel = cancel
	s.cancelMu.Unlock()
	defer func() {
		s.cancelMu.Lock()
		s.cancel = nil
		s.cancelMu.Unlock()
		cancel()
	}()
	return s.record(ctx, o)
}

func (s *Session) Awake() bool {
	return s.awake.Load()
}

func (s *Session) Run(ctx context.Context) {
	SetMode("sleeping", "")
	for ctx.Err() == nil {
		if s.wakeFlag.Swap(false) {
			s.Converse(ctx)
			continue
		}
		select {
		case <-time.After(200 * time.Millisecond):
		case <-ctx.Done():
		}
	}
}

func (s *Session) Announce(ctx context.Context, text string) {
	text = spoken(text)
	if text == "" {
		return
	}
	if s.Awake() {
		s.Say(ctx, text)
		return
	}
	s.noticeMu.Lock()
	defer s.noticeMu.Unlock()
	LaunchOrb(s.ui)
	PlayFile(config.Path("chime.wav"))
	s.Say(ctx, text)
	SetMode("notice", text)
	select {
	case <-time.After(6 * time.Second):
	case <-ctx.Done():
	}
	if !s.Awake() {
		SetMode("sleeping", "")
	}
}

func spoken(text string) string {
	var b strings.Builder
	for _, r := range strings.NewReplacer("\n", ". ", "*", "", "#", "").Replace(text) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) || strings.ContainsRune(".,:;!?'()-/@&", r) {
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func (s *Session) Sleep() {
	s.convMu.Lock()
	if s.stopConv != nil {
		s.stopConv()
	}
	s.convMu.Unlock()
}

func (s *Session) Converse(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	s.convMu.Lock()
	s.stopConv = cancel
	s.convMu.Unlock()
	s.awake.Store(true)
	defer func() {
		s.convMu.Lock()
		s.stopConv = nil
		s.convMu.Unlock()
		cancel()
		s.awake.Store(false)
		SetMode("sleeping", "")
	}()
	SetMode("awake", "")
	LaunchOrb(s.ui)
	s.log("wake", "woke up (Space + K)")
	PlayFile(config.Path("chime.wav"))
	s.Say(ctx, "Yes, "+s.owner+"?")
	text := ""
	for ctx.Err() == nil {
		if text == "" {
			SetMode("listening", "")
			idle := time.Duration(s.cfg.IdleSeconds) * time.Second
			pcm := s.record(ctx, RecordOptions{WaitStart: idle, MaxLength: 30 * time.Second, Silence: 1200 * time.Millisecond})
			if ctx.Err() != nil {
				s.log("voice", "put to sleep")
				return
			}
			if pcm == nil {
				s.log("voice", fmt.Sprintf("heard no speech (loudest %.3f, speech starts above %.3f); going to sleep", LastPeak(), s.cfg.SilenceLevel))
				s.Say(ctx, "Going to sleep.")
				return
			}
			SetMode("thinking", "...")
			heard, err := Transcribe(ctx, s.cfg, pcm)
			if err != nil {
				s.log("error", "speech to text: "+err.Error())
				s.Say(ctx, "Sorry, I could not hear that.")
				continue
			}
			text = strings.TrimSpace(heard)
			s.log("voice", "heard: "+text)
		}
		if text == "" {
			continue
		}
		if ok, _ := MatchPhrase(text, s.cfg.SleepPhrases); ok {
			s.Say(ctx, "Going to sleep.")
			return
		}
		SetMode("thinking", "“"+text+"”")
		reply, stopped := s.work(ctx, text)
		if ctx.Err() != nil {
			return
		}
		if stopped {
			reply = "Okay, I stopped."
		}
		s.Say(ctx, reply)
		text = ""
	}
}

func (s *Session) Say(ctx context.Context, text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	s.busy.Store(true)
	defer s.busy.Store(false)
	previous := Mode()
	SetMode("speaking", text)
	if err := Speak(ctx, s.cfg, text); err != nil {
		s.log("error", "text to speech: "+err.Error())
		exec.Command("notify-send", "Kairo", text).Run()
	}
	SetMode(previous, "")
}

func (s *Session) work(ctx context.Context, text string) (string, bool) {
	taskCtx, stopTask := context.WithCancel(ctx)
	defer stopTask()
	watchCtx, stopWatch := context.WithCancel(ctx)
	defer stopWatch()
	done := make(chan string, 1)
	go func() {
		done <- s.handle(taskCtx, text)
		stopWatch()
	}()
	for {
		select {
		case reply := <-done:
			return reply, false
		default:
		}
		if s.busy.Load() {
			select {
			case reply := <-done:
				return reply, false
			case <-time.After(200 * time.Millisecond):
			}
			continue
		}
		pcm := s.listen(watchCtx, RecordOptions{WaitStart: 1500 * time.Millisecond, MaxLength: 4 * time.Second, Silence: 700 * time.Millisecond})
		if pcm == nil || watchCtx.Err() != nil {
			continue
		}
		heard, err := Transcribe(watchCtx, s.cfg, pcm)
		if err == nil && isInterrupt(heard) {
			s.log("voice", "interrupted: "+heard)
			stopTask()
			<-done
			return "", true
		}
	}
}

func isInterrupt(text string) bool {
	heard := " " + normalize(text) + " "
	for _, w := range []string{"stop", "cancel", "abort", "wait", "hold on", "never mind", "nevermind", "don t", "dont", "do not"} {
		if strings.Contains(heard, " "+w+" ") {
			return true
		}
	}
	return false
}

func (s *Session) interruptListen() {
	s.cancelMu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	s.cancelMu.Unlock()
}

func (s *Session) AskYesNo(ctx context.Context, question string, wait time.Duration) string {
	s.busy.Store(true)
	defer s.busy.Store(false)
	s.interruptListen()
	LaunchOrb(s.ui)
	s.Say(ctx, question)
	previous := Mode()
	SetMode("listening", question)
	defer SetMode(previous, "")
	pcm := s.record(ctx, RecordOptions{WaitStart: wait, MaxLength: 8 * time.Second, Silence: time.Second})
	if pcm == nil {
		return approval.Timeout
	}
	heard, err := Transcribe(ctx, s.cfg, pcm)
	if err != nil {
		return approval.Timeout
	}
	s.log("voice", "answer: "+heard)
	if answer := approval.ParseAnswer(heard); answer != "" {
		return answer
	}
	return approval.Timeout
}

func (s *Session) record(ctx context.Context, o RecordOptions) []byte {
	s.micMu.Lock()
	defer s.micMu.Unlock()
	o.Threshold = s.cfg.SilenceLevel
	o.Mic = s.cfg.Mic
	pcm, err := Record(ctx, o)
	if err != nil && ctx.Err() == nil {
		s.log("error", "microphone: "+err.Error())
		time.Sleep(2 * time.Second)
	}
	return pcm
}

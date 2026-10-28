package config

import (
	"bufio"
	_ "embed"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed default_config.yaml
var DefaultConfig string

//go:embed default_persona.yaml
var DefaultPersona string

//go:embed default_env
var DefaultEnv string

type Config struct {
	Name     string `yaml:"name"`
	Owner    string `yaml:"owner"`
	Timezone string `yaml:"timezone"`
	Model    string `yaml:"model"`
	Effort   string `yaml:"effort"`
	MaxSteps int    `yaml:"max_steps"`

	DefaultDelegate string     `yaml:"default_delegate"`
	Delegates       []Delegate `yaml:"delegates"`
	Voice           Voice      `yaml:"voice"`
	Jev             Jev        `yaml:"jev"`
	Mail            Mail       `yaml:"mail"`
	Calendar        Calendar   `yaml:"calendar"`
	Memory          Memory     `yaml:"memory"`
	Channels        Channels   `yaml:"channels"`
	Notify          Notify     `yaml:"notify"`
	Browser         Browser    `yaml:"browser"`
	Learning        Learning   `yaml:"learning"`
	Skills          Skills     `yaml:"skills"`
	Guardrails      Guardrails `yaml:"guardrails"`
	UI              UI         `yaml:"ui"`
}

type Skills struct {
	Paths []string `yaml:"paths"`
	Allow []string `yaml:"allow"`
}

type Guardrails struct {
	ProtectedPaths []string `yaml:"protected_paths"`
}

type Delegate struct {
	Name    string   `yaml:"name"`
	Command []string `yaml:"command"`
}

type Voice struct {
	Enabled        bool     `yaml:"enabled"`
	SleepPhrases   []string `yaml:"sleep_phrases"`
	IdleSeconds    int      `yaml:"idle_seconds"`
	WhisperBin     string   `yaml:"whisper_bin"`
	WhisperModel   string   `yaml:"whisper_model"`
	STT            string   `yaml:"stt"`
	TTS            string   `yaml:"tts"`
	ElevenVoiceID  string   `yaml:"elevenlabs_voice_id"`
	ElevenTTSModel string   `yaml:"elevenlabs_tts_model"`
	ElevenSTTModel string   `yaml:"elevenlabs_stt_model"`
	PiperBin       string   `yaml:"piper_bin"`
	PiperModel     string   `yaml:"piper_model"`
	Language       string   `yaml:"language"`
	SilenceLevel   float64  `yaml:"silence_level"`
	Mic            string   `yaml:"mic"`
}

type Jev struct {
	Model     string  `yaml:"model"`
	Threshold float64 `yaml:"threshold"`
}

type Mail struct {
	Enabled bool   `yaml:"enabled"`
	Address string `yaml:"address"`
	Server  string `yaml:"server"`
	Mailbox string `yaml:"mailbox"`
}

type Calendar struct {
	Enabled          bool   `yaml:"enabled"`
	CalendarID       string `yaml:"calendar_id"`
	RemindMinutes    int    `yaml:"remind_minutes"`
	JoinMeetings     bool   `yaml:"join_meetings"`
	AskBeforeJoin    bool   `yaml:"ask_before_join"`
	AskMinutesBefore int    `yaml:"ask_minutes_before"`
	WaitSeconds      int    `yaml:"wait_for_answer_seconds"`
	IfNoAnswer       string `yaml:"if_no_answer"`
}

type Memory struct {
	CompactAt       string `yaml:"compact_at"`
	MaxFacts        int    `yaml:"max_facts"`
	MaxThreadLength int    `yaml:"max_thread_length"`
}

type Channels struct {
	Telegram Telegram `yaml:"telegram"`
	Slack    Slack    `yaml:"slack"`
	WhatsApp WhatsApp `yaml:"whatsapp"`
}

type Telegram struct {
	Enabled     bool  `yaml:"enabled"`
	OwnerChatID int64 `yaml:"owner_chat_id"`
}

type Slack struct {
	Enabled     bool   `yaml:"enabled"`
	OwnerUserID string `yaml:"owner_user_id"`
}

type WhatsApp struct {
	Enabled     bool   `yaml:"enabled"`
	OwnerNumber string `yaml:"owner_number"`
}

type Notify struct {
	SendTo          string `yaml:"send_to"`
	Primary         string `yaml:"primary"`
	VoiceCommands   bool   `yaml:"voice_commands"`
	QuietFrom       string `yaml:"quiet_from"`
	QuietTo         string `yaml:"quiet_to"`
	ApprovalSeconds int    `yaml:"approval_seconds"`
}

type Browser struct {
	Bin              string `yaml:"bin"`
	ProfileDir       string `yaml:"profile_dir"`
	PasswordStore    string `yaml:"password_store"`
	Mode             string `yaml:"mode"`
	ProfileDirectory string `yaml:"profile_directory"`
}

type Learning struct {
	Rate      float64 `yaml:"rate"`
	AutoScore float64 `yaml:"auto_score"`
	AutoCount int     `yaml:"auto_count"`
}

type UI struct {
	Orb bool `yaml:"orb"`
}

func Home() string {
	if h := os.Getenv("KAIRO_HOME"); h != "" {
		return h
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".kairo")
}

func Path(name string) string {
	return filepath.Join(Home(), name)
}

func Expand(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[2:])
	}
	return p
}

func Load() (*Config, error) {
	LoadEnv(Path(".env"))
	cfg := &Config{}
	if err := yaml.Unmarshal([]byte(DefaultConfig), cfg); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(Path("config.yaml"))
	if err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	}
	cfg.Browser.ProfileDir = Expand(cfg.Browser.ProfileDir)
	cfg.Voice.WhisperModel = Expand(cfg.Voice.WhisperModel)
	cfg.Voice.PiperModel = Expand(cfg.Voice.PiperModel)
	return cfg, nil
}

func LoadEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if os.Getenv(key) == "" && value != "" {
			os.Setenv(key, value)
		}
	}
}

# Kairo setup guide

Do these steps once, in order. Each step ends with a **Check** so you know it worked. It takes about 20 minutes.

| Step | What | Required? |
|---|---|---|
| 1 | Install packages | yes |
| 2 | Build and install Kairo | yes |
| 3 | API keys | yes |
| 4 | Your persona | yes |
| 5 | Google: Gmail + Calendar | recommended |
| 6 | Telegram on your phone | recommended |
| 7 | Space + K hotkey | yes |
| 8 | Browser for links and meetings | recommended |
| 9 | Start Kairo and run it automatically | yes |
| 10 | First things to try | |

---

## 1. Install packages

```bash
sudo pacman -S --needed go gcc pipewire wtype grim keyd whisper-cpp
```

| Package | Used for |
|---|---|
| `go`, `gcc` | Building Kairo (the orb window needs a C compiler) |
| `wtype`, `grim` | Typing and screenshots |
| `keyd` | The Space + K hotkey |
| `whisper-cpp` | Optional local speech-to-text |

Optional extras:
- `yay -S piper-tts`: a fully local voice
- **Claude Code** (`claude`) and/or **Codex**, so Kairo can hand off coding work
- The **ECC** skill library cloned to `~/Documents/research/ECC`

**Check:** `go version` prints Go 1.23 or newer.

---

## 2. Build and install Kairo

```bash
git clone <this repo> ~/kairo && cd ~/kairo
make install          # builds and copies `kairo` to ~/.local/bin
kairo init            # creates ~/.kairo with config.yaml, persona.yaml, .env, skills/
```

**Check:** `kairo status` prints `daemon: stopped`.

---

## 3. API keys

Open `~/.kairo/.env` and paste your keys after the `=`:

```
ANTHROPIC_API_KEY=        required: console.anthropic.com
ELEVENLABS_API_KEY=       required for voice: elevenlabs.io → Profile → API keys
TYPESAFE_API_KEY=         optional: Jev fast decisions (typesafe.ai)
GOOGLE_CLIENT_ID=         step 5
GOOGLE_CLIENT_SECRET=     step 5
TELEGRAM_BOT_TOKEN=       step 6
SLACK_BOT_TOKEN=          optional
SLACK_APP_TOKEN=          optional
```

Keep this file private and never commit it.

**Check:**
```bash
kairo ask "what time is it?"
```
Claude answers in the terminal.

---

## 4. Your persona

Edit `~/.kairo/persona.yaml`. Kairo reads it on every request:

```yaml
owner: Your Name
role: Engineer on the tech team
company: your-company
about: |
  What the company does and what you work on.
greeting: "Good morning."
work_style:
  - Keep spoken answers short and clear.
  - Always ask before sending messages to other people.
projects:
  - name: Platform
    path: ~/work/platform
    notes: server, ui, admin
boundaries:
  - Never reply to customers for me; draft only and ask me.
  - Never join a meeting with mic or camera on.
routine:
  - at: "09:30"
    days: [mon, tue, wed, thu, fri]
    do: greet_and_brief
  - at: "18:30"
    days: [mon, tue, wed, thu, fri]
    do: evening_summary
```

Also set `owner` in `~/.kairo/config.yaml`. To keep folders off-limits (keys, secrets, personal files), add them to `config.yaml`:

```yaml
guardrails:
  protected_paths:
    - ~/work/ssh_keys
    - ~/Documents/personal
```

**Check:**
```bash
kairo ask "who am I and what are two things you must never do for me?"
```

---

## 5. Google: Gmail + Calendar (OAuth 2.0)

### 5a. Create an OAuth client (one time)
1. Open https://console.cloud.google.com and create a project, for example `kairo`.
2. **APIs & Services → Library**: enable **Gmail API** and **Google Calendar API**.
3. **Google Auth Platform (OAuth consent screen)**:
   - User type: **External**
   - App name: Kairo, with your email as support email
   - **Audience → Test users**: add your own Gmail address
   - **Publish app → In production.** This stops the sign-in expiring every 7 days. For your own account you can ignore the verification notice.
4. **Credentials → Create credentials → OAuth client ID → Desktop app**.
5. Copy the **Client ID** and **Client secret** into `~/.kairo/.env`.

### 5b. Sign in
```bash
kairo google login
```
- Chrome opens. Choose your account.
- Google warns that the app isn't verified. Click **Continue**.
- Allow Gmail, Calendar and Google Drive (Drive is only used to send you files bigger than 50 MB).

If you see "Error 400", the OAuth client is probably not a Desktop app or you're not a test user. Or the sign-in opened in Brave, whose Shields block it: paste the printed link into Chrome.

### 5c. Turn mail and calendar on
In `~/.kairo/config.yaml`:
```yaml
mail:
  enabled: true
  address: you@company.com
calendar:
  enabled: true
```

**Check:**
```bash
kairo mail check 5     # rates your last 5 emails
kairo calendar ls      # your next 2 days of events
```

---

## 6. Telegram on your phone

1. In Telegram, talk to **@BotFather**, send `/newbot`, pick a name, and copy the token into `.env` as `TELEGRAM_BOT_TOKEN`.
2. Run:
   ```bash
   kairo channels telegram-setup
   ```
3. Send **hi** to your new bot. Kairo saves your chat ID and switches Telegram on.

Only your chat can control Kairo. Don't use the same bot token in any other app, or that app will take Kairo's messages.

**Check:** `kairo channels test`. A message arrives on your phone.

---

## 7. Space + K hotkey

Hyprland can't bind Space as a modifier key, so `keyd` turns "hold Space + K" into the F13 key:

```bash
make hotkey      # installs scripts/keyd-kairo.conf to /etc/keyd and starts keyd (sudo)
```

Then add these lines to `~/.config/hypr/bindings.lua`:
```lua
o.bind("F13", "Kairo listen", "kairo listen")
o.bind("XF86Tools", "Kairo listen", "kairo listen")
o.bind("SUPER + SHIFT + J", "Kairo sleep", "kairo sleep")
```
Reload with `hyprctl reload`.

Tapping Space still types a space. Holding Space and pressing K wakes Kairo. SUPER + SHIFT + J stops everything.

**Check:** `hyprctl binds -j | grep -c Kairo` prints `3`.

---

## 8. Browser for links and meetings

**Option A: use your own browser profile** (recommended, already signed in). Find the profile folder name in `~/.config/BraveSoftware/Brave-Browser/Local State` (Chrome: `~/.config/google-chrome/Local State`), for example `Profile 3`. Then set:

```yaml
browser:
  bin: brave
  mode: existing
  profile_directory: Profile 3
```

Kairo opens links and meetings in that profile and turns mic and camera off. You click **Join now**.

**Option B: fully automatic joins.** Kairo uses its own browser and clicks Join itself:
```yaml
browser:
  bin: google-chrome-stable
  mode: kairo
  profile_dir: ~/.kairo/chrome-profile
```
Sign in to Google once in that browser:
```bash
google-chrome-stable --user-data-dir=$HOME/.kairo/chrome-profile https://accounts.google.com
```

**Check:**
```bash
kairo ask "open my Gmail inbox"
```
It opens in the right browser and profile.

---

## 9. Start Kairo and run it automatically

Try it in the foreground first:
```bash
kairo start          # Ctrl+C stops it
```
Hold **Space + K**. The orb appears and Kairo says "Yes?". Ask "what's on my calendar today?".

When it works, make it permanent. Run this from a normal terminal, so Kairo can find `claude` and `codex`:
```bash
kairo service install            # starts now and with every login, restarts on crash
systemctl --user status kairo
journalctl --user -u kairo -f    # live logs
systemctl --user restart kairo   # after editing config.yaml, persona.yaml or .env
```

**Check:** `kairo status` shows `daemon: running` and your channels.

---

## 10. First things to try

| Try | What happens |
|---|---|
| Space + K → "what's on my calendar today?" | Spoken answer, steps shown in the pill |
| Space + K → "open Slack" → "stop" | Interrupts the task |
| `kairo tasks add "test reminder" --due 1m` | One minute later: orb, voice and phone notification |
| Telegram: "remind me at 6 to call Rahul" | Task created from your phone |
| Telegram: `/tasks` | Your open tasks |
| `kairo ask "fix the failing tests in ~/work/app"` | Asks your OK, hands it to Claude Code, messages you the result |
| `kairo schedule join <meet link> 15:00` | Opens the meeting at 15:00 with mic and camera off |
| `kairo ask "read ~/.ssh/id_ed25519"` | Refused: guardrails |
| `kairo tui` | Live dashboard |

---

## Troubleshooting

| Problem | Fix |
|---|---|
| Space + K does nothing | `systemctl status keyd`, check step 7 bindings, `kairo status` |
| Kairo doesn't hear you | `kairo mic`; lower `voice.silence_level` or set `voice.mic` (`wpctl status` lists inputs) |
| No voice reply | Check `ELEVENLABS_API_KEY`; `kairo activity -n 20` |
| Telegram silent | `kairo channels telegram-setup`, restart Kairo, and make sure no other app uses the token |
| Google error 400 / not signed in | Step 5 (Desktop app, test user, use Chrome), then `kairo google login` |
| Mail or Calendar stops after a week | Publish the Google app (step 5a) |
| Wrong browser opens | Step 8 |
| Claude Code "command not found" | `kairo service install` again from a normal terminal |
| Anything else | `kairo activity -n 30`, `journalctl --user -u kairo -f` |

package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
)

type Telegram struct {
	token string
	owner int64
}

type tgUpdate struct {
	UpdateID int64 `json:"update_id"`
	Message  *struct {
		Text string `json:"text"`
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		From struct {
			Username string `json:"username"`
		} `json:"from"`
	} `json:"message"`
	CallbackQuery *struct {
		ID   string `json:"id"`
		Data string `json:"data"`
		From struct {
			ID int64 `json:"id"`
		} `json:"from"`
	} `json:"callback_query"`
}

func NewTelegram(token string, owner int64) *Telegram {
	return &Telegram{token: token, owner: owner}
}

func (t *Telegram) Name() string {
	return "telegram"
}

func (t *Telegram) Start(ctx context.Context, on func(Incoming)) error {
	if t.token == "" || t.owner == 0 {
		return errors.New("set TELEGRAM_BOT_TOKEN and channels.telegram.owner_chat_id")
	}
	offset := int64(0)
	for ctx.Err() == nil {
		updates, err := t.updates(ctx, offset, 30)
		if err != nil {
			return err
		}
		for _, u := range updates {
			offset = u.UpdateID + 1
			if u.Message != nil && u.Message.Chat.ID == t.owner && u.Message.Text != "" {
				on(Incoming{Channel: "telegram", Text: u.Message.Text})
			}
			if u.CallbackQuery != nil && u.CallbackQuery.From.ID == t.owner {
				t.call(ctx, "answerCallbackQuery", map[string]any{"callback_query_id": u.CallbackQuery.ID}, nil)
				on(Incoming{Channel: "telegram", Text: u.CallbackQuery.Data})
			}
		}
	}
	return nil
}

func (t *Telegram) Send(ctx context.Context, u Update) error {
	body := map[string]any{"chat_id": t.owner, "text": u.Text}
	if len(u.Buttons) > 0 {
		var row []map[string]string
		for _, b := range u.Buttons {
			row = append(row, map[string]string{"text": b.Label, "callback_data": b.Data})
		}
		body["reply_markup"] = map[string]any{"inline_keyboard": [][]map[string]string{row}}
	}
	return t.call(ctx, "sendMessage", body, nil)
}

func (t *Telegram) SendFile(ctx context.Context, path, caption string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	reader, writer := io.Pipe()
	form := multipart.NewWriter(writer)
	go func() {
		form.WriteField("chat_id", strconv.FormatInt(t.owner, 10))
		form.WriteField("caption", caption)
		part, err := form.CreateFormFile("document", filepath.Base(path))
		if err == nil {
			_, err = io.Copy(part, f)
		}
		form.Close()
		writer.CloseWithError(err)
	}()
	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendDocument", url.PathEscape(t.token))
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", form.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var out struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	if !out.OK {
		return fmt.Errorf("telegram sendDocument: %s", out.Description)
	}
	return nil
}

func (t *Telegram) updates(ctx context.Context, offset int64, timeout int) ([]tgUpdate, error) {
	var updates []tgUpdate
	err := t.call(ctx, "getUpdates", map[string]any{"offset": offset, "timeout": timeout}, &updates)
	return updates, err
}

func (t *Telegram) call(ctx context.Context, method string, body any, result any) error {
	data, _ := json.Marshal(body)
	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/%s", url.PathEscape(t.token), method)
	req, _ := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var out struct {
		OK          bool            `json:"ok"`
		Description string          `json:"description"`
		Result      json.RawMessage `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}
	if !out.OK {
		return fmt.Errorf("telegram %s: %s", method, out.Description)
	}
	if result != nil {
		return json.Unmarshal(out.Result, result)
	}
	return nil
}

func TelegramChats(ctx context.Context, token string) ([]string, error) {
	t := NewTelegram(token, 0)
	updates, err := t.updates(ctx, 0, 0)
	if err != nil {
		return nil, err
	}
	var chats []string
	for _, u := range updates {
		if u.Message != nil {
			chats = append(chats, fmt.Sprintf("chat_id %d  (@%s: %s)", u.Message.Chat.ID, u.Message.From.Username, u.Message.Text))
		}
	}
	return chats, nil
}

func WaitForChat(ctx context.Context, token string) (int64, string, error) {
	t := NewTelegram(token, 0)
	offset := int64(0)
	for ctx.Err() == nil {
		updates, err := t.updates(ctx, offset, 30)
		if err != nil {
			return 0, "", err
		}
		for _, u := range updates {
			offset = u.UpdateID + 1
			if u.Message != nil {
				t.updates(ctx, offset, 0)
				return u.Message.Chat.ID, u.Message.From.Username, nil
			}
		}
	}
	return 0, "", ctx.Err()
}

package channels

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"

	"kairo/internal/config"
)

const botPrefix = "🤖 "

type WhatsApp struct {
	owner  string
	client *whatsmeow.Client
}

func NewWhatsApp(owner string) *WhatsApp {
	return &WhatsApp{owner: strings.TrimPrefix(owner, "+")}
}

func (w *WhatsApp) Name() string {
	return "whatsapp"
}

func openWhatsApp(ctx context.Context) (*whatsmeow.Client, error) {
	db, err := sql.Open("sqlite", "file:"+config.Path("whatsapp.db")+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	container := sqlstore.NewWithDB(db, "sqlite3", waLog.Noop)
	if err := container.Upgrade(ctx); err != nil {
		return nil, err
	}
	device, err := container.GetFirstDevice(ctx)
	if err != nil {
		return nil, err
	}
	return whatsmeow.NewClient(device, waLog.Noop), nil
}

func (w *WhatsApp) Start(ctx context.Context, on func(Incoming)) error {
	if w.owner == "" {
		return errors.New("set channels.whatsapp.owner_number")
	}
	client, err := openWhatsApp(ctx)
	if err != nil {
		return err
	}
	if client.Store.ID == nil {
		return errors.New("whatsapp is not paired, run: kairo channels pair-whatsapp")
	}
	client.AddEventHandler(func(evt any) {
		msg, ok := evt.(*events.Message)
		if !ok || msg.Info.Sender.User != w.owner {
			return
		}
		text := msg.Message.GetConversation()
		if text == "" {
			text = msg.Message.GetExtendedTextMessage().GetText()
		}
		if text != "" && !strings.HasPrefix(text, botPrefix) {
			on(Incoming{Channel: "whatsapp", Text: text})
		}
	})
	if err := client.Connect(); err != nil {
		return err
	}
	w.client = client
	<-ctx.Done()
	client.Disconnect()
	w.client = nil
	return nil
}

func (w *WhatsApp) Send(ctx context.Context, u Update) error {
	if w.client == nil {
		return errors.New("whatsapp is not connected")
	}
	text := botPrefix + u.Text
	for _, b := range u.Buttons {
		text += fmt.Sprintf("\n• %s → reply %s", b.Label, b.Data)
	}
	to := types.NewJID(w.owner, types.DefaultUserServer)
	_, err := w.client.SendMessage(ctx, to, &waE2E.Message{Conversation: proto.String(text)})
	return err
}

func (w *WhatsApp) SendFile(ctx context.Context, path, caption string) error {
	return errors.New("sending files over whatsapp is not supported yet")
}

func PairWhatsApp(ctx context.Context) error {
	client, err := openWhatsApp(ctx)
	if err != nil {
		return err
	}
	if client.Store.ID != nil {
		fmt.Println("whatsapp is already paired")
		return nil
	}
	qr, err := client.GetQRChannel(ctx)
	if err != nil {
		return err
	}
	if err := client.Connect(); err != nil {
		return err
	}
	defer client.Disconnect()
	fmt.Println("Open WhatsApp > Linked devices > Link a device, then scan:")
	for item := range qr {
		switch item.Event {
		case "code":
			qrterminal.GenerateHalfBlock(item.Code, qrterminal.L, os.Stdout)
		case "success":
			fmt.Println("paired")
			return nil
		default:
			fmt.Println("whatsapp:", item.Event)
		}
	}
	return nil
}

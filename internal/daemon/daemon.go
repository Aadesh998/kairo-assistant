package daemon

import (
	"context"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"kairo/internal/app"
	"kairo/internal/channels"
	"kairo/internal/config"
	"kairo/internal/scheduler"
	"kairo/internal/voice"
)

func Run(ctx context.Context) error {
	a, err := app.Build(true)
	if err != nil {
		return err
	}
	defer a.Store.Close()
	os.WriteFile(config.Path("daemon.pid"), []byte(strconv.Itoa(os.Getpid())), 0o600)
	defer os.Remove(config.Path("daemon.pid"))

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	voice.SetMode("sleeping", "")
	defer voice.SetMode("sleeping", "")
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, syscall.SIGUSR1, syscall.SIGUSR2)
	go func() {
		for sig := range signals {
			if sig == syscall.SIGUSR1 && a.Voice == nil {
				continue
			}
			if sig == syscall.SIGUSR1 {
				a.Voice.Wake()
			} else {
				a.StopAll()
			}
		}
	}()

	a.Store.Log("daemon", "started")
	a.Notifier.Start(ctx, func(in channels.Incoming) { handleIncoming(ctx, a, in) })
	go scheduler.Run(ctx, a)
	if a.Voice != nil {
		go a.Voice.Run(ctx)
	}
	if a.Config.Mail.Enabled {
		go a.Mail.Run(ctx)
	}
	if a.Config.Calendar.Enabled {
		go a.Calendar.Run(ctx)
	}
	a.Notifier.Send(ctx, channels.Update{Text: "🟢 " + a.Config.Name + " is online"})
	<-ctx.Done()
	a.Store.Log("daemon", "stopped")
	return nil
}

func RunningPID() (int, bool) {
	data, err := os.ReadFile(config.Path("daemon.pid"))
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, false
	}
	return pid, syscall.Kill(pid, 0) == nil
}

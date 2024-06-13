package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var VERSION = "1.0.0"

var Ping = discord.SlashCommandCreate{
	Name:        "ping",
	Description: "Pings the bot",
	Options:     nil,
}

func (b *Bot) Ping(e *handler.CommandEvent) error {
	slog.Info("Bot was Pinged!")

	message := discord.MessageCreate{
		Content: "Pong!",
	}

	return e.CreateMessage(message)
}

func setupLogger(cfg *Config) {
	opts := &slog.HandlerOptions{
		AddSource: cfg.Log.AddSource,
		Level:     cfg.Log.Level,
	}

	var sHandler slog.Handler
	switch cfg.Log.Format {
	case "json":
		sHandler = slog.NewJSONHandler(os.Stdout, opts)
	case "text":
		sHandler = slog.NewTextHandler(os.Stdout, opts)
	default:
		slog.Error("Unknown log format", slog.String("format", cfg.Log.Format))
		os.Exit(-1)
	}
	slog.SetDefault(slog.New(sHandler))
}

func (b *Bot) registerHandlers() *handler.Mux {
	h := handler.New()

	h.Command("/ping", b.Ping)

	return h
}

func main() {
	shouldSyncCmds := flag.Bool("sync", false, "Sync Commands")
	flag.Parse()

	cfg, err := LoadConfig("./config.toml")
	if err != nil {
		slog.Error("Failed to load config", slog.Any("err", err))
		os.Exit(1)
	}

	setupLogger(cfg)
	slog.Info("Starting KaijiBot...", slog.String("version", VERSION), slog.String("commit", VERSION))

	b := NewBot(cfg, VERSION)
	h := b.registerHandlers()
	listeners := []bot.EventListener{
		h,
		bot.NewListenerFunc(b.OnReady),
	}
	if err = b.SetupBot(listeners...); err != nil {
		slog.Error("Failed to setup bot", slog.Any("err", err))
		os.Exit(-1)
	}

	if *shouldSyncCmds {
		slog.Info("Syncing commands...")
		commands := []discord.ApplicationCommandCreate{Ping}
		if err = handler.SyncCommands(b.Client, commands, cfg.Bot.DevGuilds); err != nil {
			slog.Error("Failed to sync commands", slog.Any("err", err))
			os.Exit(-1)
		}
		slog.Info("Commands synced")
	}

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		b.Client.Close(ctx)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err = b.Client.OpenGateway(ctx); err != nil {
		slog.Error("Failed to open gateway", slog.Any("err", err))
		os.Exit(-1)
	}

	slog.Info("Bot is running. Press CTRL-C to exit.")
	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGINT, syscall.SIGTERM)
	<-s
	slog.Info("Shutting down bot...")
}

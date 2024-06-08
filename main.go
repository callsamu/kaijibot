package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/cache"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
	"github.com/spf13/viper"
)

var (
	VERSION = "dev"
	COMMIT = "unknown"
)

type Config struct {
	Bot struct {
		Token string `mapstructure:"token"`
	}`toml:"bot"`

	Log struct {
		Level     slog.Level `toml:"level"`
		Format    string     `toml:"format"`
		AddSource bool       `toml:"add_source"`
	} `toml:"log"`
}

func LoadConfig(path string) (*Config, error) {
	viper.SetEnvPrefix("KAIJIBOT")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	err := viper.BindEnv("bot.token")
	if err != nil {
		return nil, err
	}

	viper.SetConfigType("toml")
	viper.SetConfigFile(path)
	err = viper.ReadInConfig()
	if err != nil {
		return nil, err
	}

	var cfg Config
	err = viper.Unmarshal(&cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

type Bot struct {
	Cfg       *Config
	Client    bot.Client
	Version   string
	Commit    string
}

func NewBot(cfg *Config, version string, commit string) *Bot {
	return &Bot{
		Cfg:       cfg,
		Version:   version,
		Commit:    commit,
	}
}

func (b *Bot) SetupBot(listeners ...bot.EventListener) error {
	client, err := disgo.New(b.Cfg.Bot.Token,
		bot.WithGatewayConfigOpts(gateway.WithIntents(gateway.IntentGuilds, gateway.IntentGuildMessages)),
		bot.WithCacheConfigOpts(cache.WithCaches(cache.FlagGuilds)),
		bot.WithEventListeners(listeners...),
	)
	if err != nil {
		return err
	}

	b.Client = client
	return nil
}

func (b *Bot) OnReady(_ *events.Ready) {
	slog.Info("KaijiBot is ready")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := b.Client.SetPresence(ctx, gateway.WithListeningActivity("you"), gateway.WithOnlineStatus(discord.OnlineStatusOnline)); err != nil {
		slog.Error("Failed to set presence", slog.Any("err", err))
	}
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

func main() {
	cfg, err := LoadConfig("./config.toml")
	if err != nil {
		slog.Error("Failed to load config", slog.Any("err", err))
		os.Exit(1)
	}

	setupLogger(cfg)
	slog.Info("Starting KaijiBot...", slog.String("version", VERSION), slog.String("commit", COMMIT))

	b := NewBot(cfg, VERSION, COMMIT)
	if err = b.SetupBot(bot.NewListenerFunc(b.OnReady)); err != nil {
		slog.Error("Failed to setup bot", slog.Any("err", err))
		os.Exit(-1)
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

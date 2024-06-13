package main

import (
	"context"
	"flag"
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
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/snowflake/v2"
	"github.com/spf13/viper"
)

var VERSION = "1.0.0"

type Config struct {
	Bot struct {
		Env       string   		  `mapstructure:"environment"`
		Token     string   		  `toml:"token"`
		DevGuilds []snowflake.ID  `toml:"dev_guilds"`
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
	Commit    string
}

func NewBot(cfg *Config, commit string) *Bot {
	return &Bot{
		Cfg:       cfg,
		Commit:    commit,
	}
}

func (b *Bot) SetupBot(listeners ...bot.EventListener) error {
	intents := []gateway.Intents{
		gateway.IntentGuilds,
		gateway.IntentGuildMessages,
	}

	client, err := disgo.New(b.Cfg.Bot.Token,
		bot.WithGatewayConfigOpts(gateway.WithIntents(intents...)),
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

var MyPing = discord.SlashCommandCreate{
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
		commands := []discord.ApplicationCommandCreate{MyPing}
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

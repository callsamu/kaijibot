package main

import (
	"log/slog"
	"strings"

	"github.com/disgoorg/snowflake/v2"
	"github.com/spf13/viper"
)

type Config struct {
	Bot struct {
		Env       string         `mapstructure:"environment"`
		Token     string         `toml:"token"`
		DevGuilds []snowflake.ID `toml:"dev_guilds"`
	} `toml:"bot"`

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

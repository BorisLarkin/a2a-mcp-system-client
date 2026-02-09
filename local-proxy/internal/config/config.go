// ./local-proxy/internal/config/config.go
package config

import (
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server struct {
		Port         string        `mapstructure:"port"`
		ReadTimeout  time.Duration `mapstructure:"read_timeout"`
		WriteTimeout time.Duration `mapstructure:"write_timeout"`
		IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
	} `mapstructure:"server"`

	Database struct {
		Host     string `mapstructure:"host"`
		Port     int    `mapstructure:"port"`
		User     string `mapstructure:"user"`
		Password string `mapstructure:"password"`
		Name     string `mapstructure:"name"`
		SSLMode  string `mapstructure:"ssl_mode"`
	} `mapstructure:"database"`

	Redis struct {
		Host     string `mapstructure:"host"`
		Port     int    `mapstructure:"port"`
		Password string `mapstructure:"password"`
		DB       int    `mapstructure:"db"`
	} `mapstructure:"redis"`

	Orchestrator struct {
		URL     string        `mapstructure:"url"`
		APIKey  string        `mapstructure:"api_key"`
		Timeout time.Duration `mapstructure:"timeout"`
	} `mapstructure:"orchestrator"`

	Auth struct {
		JWTSecret          string        `mapstructure:"jwt_secret"`
		AccessTokenExpiry  time.Duration `mapstructure:"access_token_expiry"`
		RefreshTokenExpiry time.Duration `mapstructure:"refresh_token_expiry"`
	} `mapstructure:"auth"`

	Telegram struct {
		WebhookURL string `mapstructure:"webhook_url"`
	} `mapstructure:"telegram"`

	// Локальные настройки диспетчерской
	Dispatcher struct {
		Name                string `mapstructure:"name"`
		AutoResponseEnabled bool   `mapstructure:"auto_response_enabled"`
		DefaultChannel      string `mapstructure:"default_channel"`
		WorkHoursStart      string `mapstructure:"work_hours_start"`
		WorkHoursEnd        string `mapstructure:"work_hours_end"`
	} `mapstructure:"dispatcher"`
}

func LoadConfig(path string) (*Config, error) {
	viper.SetConfigFile(path)
	viper.SetConfigType("yaml")

	// Установка значений по умолчанию
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.read_timeout", "30s")
	viper.SetDefault("orchestrator.timeout", "30s")
	viper.SetDefault("auth.access_token_expiry", "24h")

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

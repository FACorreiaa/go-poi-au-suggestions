package config

import (
	"bytes"
	_ "embed"
	"fmt"
	"time"

	"github.com/spf13/viper"
)

//go:embed config.yml
var embeddedConfig []byte

type Config struct {
	Mode     string `mapstructure:"mode"`
	Dotenv   string `mapstructure:"dotenv"`
	Handlers struct {
		ExternalAPI struct {
			Port      string `mapstrucutre:"port"`
			CertFile  string `mapstructure:"certFile"`
			KeyFile   string `mapstructure:"keyFile"`
			EnableTLS bool   `mapstracture:"enableTLS"`
		} `mapstructure:"externalAPI"`
		Pprof struct {
			Port      string `mapstructure:"port"`
			CertFile  string `mapstructure:"certFile"`
			KeyFile   string `mapstructure:"keyFile"`
			EnableTLS bool   `mapstructure:"enableTLS"`
		}
		Prometheus struct {
			Port      string `mapstructure:"port"`
			CertFile  string `mapstructure:"certFile"`
			KeyFile   string `mapstructure:"keyFile"`
			EnableTLS bool   `mapstructure:"enableTLS"`
		}
	} `mapstructure:"handlers"`
	Repositories struct {
		Postgres struct {
			Host              string `mapstructure:"host"`
			Password          string `mapstructure:"password"`
			Port              string `mapstructure:"port"`
			Username          string `mapstructure:"username"`
			DB                string `mapstructure:"db"`
			SSLMODE           string `mapstructure:"SSLMODE"`
			MAXCONWAITINGTIME int    `mapstructure:"MAXCONWAITINGTIME"`
		} `mapstructure:"postgres"`
	}
	Server struct {
		HTTPPort string        `mapstructure:"HTTPPort"`
		Timeout  time.Duration `mapstructure:"HTTPTimeout"`
	} `mapstructure:"server"`
}

func InitConfig() (Config, error) {
	var config Config
	v := viper.New()

	// Add file-based config paths
	v.AddConfigPath(".")
	v.AddConfigPath("config")
	v.AddConfigPath("/app/config")
	v.AddConfigPath("/usr/local/bin")
	v.AddConfigPath("/usr/local/bin/inkme")

	v.SetConfigName("config")
	v.SetConfigType("yml")

	// Try to load file-based config
	err := v.ReadInConfig()
	if err != nil {
		fmt.Printf("Warning: Failed to find file-based config: %s. Falling back to embedded config.\n", err)
		if err = v.ReadConfig(bytes.NewReader(embeddedConfig)); err != nil {
			return Config{}, fmt.Errorf("failed to read embedded config: %s", err)
		}
	}

	// Unmarshal the config into the Config struct
	if err = v.Unmarshal(&config); err != nil {
		return Config{}, fmt.Errorf("failed to unmarshal config: %s", err)
	}
	fmt.Println("Successfully loaded app configs...")
	return config, nil
}

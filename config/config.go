package config

import (
	"bytes"
	"fmt"

	"github.com/spf13/viper"
)

//go:embed config.yml
var embeddedConfig []byte

type Config struct {
	Mode             string                 `mapstructure:"mode"`
	Dotenv           string                 `mapstructure:"dotenv"`
	Handlers         HandlersConfig         `mapstructure:"handlers"`
	Server           ServerConfig           `mapstructure:"server"`
	UpstreamServices UpstreamServicesConfig `mapstructure:"upstream_services"`
	Database         DatabaseConfig         `mapstructure:"database"`
}

type HandlersConfig struct {
	ExternalAPI struct {
		Port      string `mapstructure:"port"`
		CertFile  string `mapstructure:"certFile"`
		KeyFile   string `mapstructure:"keyFile"`
		EnableTLS bool   `mapstructure:"enableTLS"`
	} `mapstructure:"externalAPI"`
	Pprof struct {
		Port      string `mapstructure:"port"`
		CertFile  string `mapstructure:"certFile"`
		KeyFile   string `mapstructure:"keyFile"`
		EnableTLS bool   `mapstructure:"enableTLS"`
	} `mapstructure:"pprof"`
	Prometheus struct {
		Port      string `mapstructure:"port"`
		CertFile  string `mapstructure:"certFile"`
		KeyFile   string `mapstructure:"keyFile"`
		EnableTLS bool   `mapstructure:"enableTLS"`
	} `mapstructure:"prometheus"`
}

type ServerConfig struct {
	Port                   string `mapstructure:"port"`
	CertFile               string `mapstructure:"certFile"`
	KeyFile                string `mapstructure:"keyFile"`
	EnableTLS              bool   `mapstructure:"enableTLS"`
	Timeout                int    `mapstructure:"timeout"`
	IdleTimeout            int    `mapstructure:"idleTimeout"`
	ReadTimeout            int    `mapstructure:"readTimeout"`
	WriteTimeout           int    `mapstructure:"writeTimeout"`
	IdleConnsClosedTimeout int    `mapstructure:"idleConnsClosedTimeout"`
	ShutdownTimeout        int    `mapstructure:"shutdownTimeout"`
}

type UpstreamServicesConfig struct {
	AuthService struct {
		Host string `mapstructure:"host"`
		Port string `mapstructure:"port"`
	} `mapstructure:"authService"`
	PaymentService struct {
		Host string `mapstructure:"host"`
		Port string `mapstructure:"port"`
	} `mapstructure:"paymentService"`
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Database string `mapstructure:"database"`
	SSLMode  string `mapstructure:"sslmode"`
	PoolSize int    `mapstructure:"poolSize"`
	Timeout  int    `mapstructure:"timeout"`
	IdleTime int    `mapstructure:"idleTime"`
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

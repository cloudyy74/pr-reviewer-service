package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Env        string `yaml:"env" env-default:"local"`
	DBURL      string `yaml:"db_url" env-required:"true"`
	HTTPServer `yaml:"http_server"`
}

type HTTPServer struct {
	Addr        string        `yaml:"addr" env-default:"localhost:8080"`
	Timeout     time.Duration `yaml:"timeout" env-default:"4s"`
	IdleTimeout time.Duration `yaml:"idle_timeout" env-default:"60s"`
}

func MustLoadConfig() *Config {
	config, err := LoadConfig()
	if err != nil {
		panic(err)
	}

	return config
}

func LoadConfig() (*Config, error) {
	configPath, ok := getConfigPath()
	if !ok {
		return nil, errors.New("config path is not set")
	}

	configData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config Config
	if err = yaml.Unmarshal(configData, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

func getConfigPath() (configPath string, ok bool) {
	flag.StringVar(&configPath, "config_path", "", "path to config")
	flag.Parse()

	if configPath == "" {
		configPath = os.Getenv("CONFIG_PATH")
	}

	return configPath, configPath != ""
}

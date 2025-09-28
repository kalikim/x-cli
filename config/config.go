package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	APIKey       string `json:"api_key"`
	APISecret    string `json:"api_secret"`
	AccessToken  string `json:"access_token"`
	AccessSecret string `json:"access_secret"`
}

var errConfigNotFound = errors.New("config file not found")

func LoadConfig() Config {
	cfg, err := readConfigFile()
	switch {
	case err == nil:
		// file loaded successfully
	case errors.Is(err, errConfigNotFound):
		log.Printf("⚠️ No config file found, relying on environment variables")
	default:
		log.Printf("⚠️ Failed to read config file: %v", err)
	}

	applyEnvOverrides(&cfg)

	return cfg
}

func (c Config) Validate() error {
	var missing []string

	if strings.TrimSpace(c.APIKey) == "" {
		missing = append(missing, "TWITTER_API_KEY")
	}
	if strings.TrimSpace(c.APISecret) == "" {
		missing = append(missing, "TWITTER_API_SECRET")
	}
	if strings.TrimSpace(c.AccessToken) == "" {
		missing = append(missing, "TWITTER_ACCESS_TOKEN")
	}
	if strings.TrimSpace(c.AccessSecret) == "" {
		missing = append(missing, "TWITTER_ACCESS_SECRET")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing credentials: %s", strings.Join(missing, ", "))
	}

	return nil
}

func readConfigFile() (Config, error) {
	var cfg Config

	for _, path := range candidatePaths() {
		data, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return cfg, fmt.Errorf("reading %s: %w", path, err)
		}

		if err := json.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("parsing %s: %w", path, err)
		}

		return cfg, nil
	}

	return cfg, errConfigNotFound
}

func candidatePaths() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}

	var paths []string
	if home != "" {
		paths = append(paths, filepath.Join(home, ".x-cli", "config.json"))
	}
	paths = append(paths, "config.json")
	return paths
}

func applyEnvOverrides(cfg *Config) {
	if v := strings.TrimSpace(os.Getenv("TWITTER_API_KEY")); v != "" {
		cfg.APIKey = v
	}
	if v := strings.TrimSpace(os.Getenv("TWITTER_API_SECRET")); v != "" {
		cfg.APISecret = v
	}
	if v := strings.TrimSpace(os.Getenv("TWITTER_ACCESS_TOKEN")); v != "" {
		cfg.AccessToken = v
	}
	if v := strings.TrimSpace(os.Getenv("TWITTER_ACCESS_SECRET")); v != "" {
		cfg.AccessSecret = v
	}
}

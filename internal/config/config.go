package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zalando/go-keyring"
)

const (
	DefaultBaseURL = "https://app.supervisible.com"
	keyringService = "supervisible-cli"
)

// Config contains user-level CLI settings.
type Config struct {
	BaseURL string `json:"base_url,omitempty"`
	Token   string `json:"token,omitempty"`
}

// Store provides config + token persistence.
type Store struct {
	path string
}

func NewStore(path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		p, err := DefaultConfigPath()
		if err != nil {
			return nil, err
		}
		path = p
	}

	return &Store{path: path}, nil
}

func DefaultConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}

	return filepath.Join(dir, "supervisible", "config.json"), nil
}

func (s *Store) Path() string {
	return s.path
}

func (s *Store) Load() (Config, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, nil
		}
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	if len(data) == 0 {
		return Config{}, nil
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}

	return cfg, nil
}

func (s *Store) Save(cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(s.path, data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

// SaveToken stores API keys in keyring if available and falls back to config file.
func (s *Store) SaveToken(baseURL, token string) (string, error) {
	baseURL = strings.TrimSpace(baseURL)
	token = strings.TrimSpace(token)
	if token == "" {
		return "", fmt.Errorf("api key is required")
	}

	if err := keyring.Set(keyringService, keyringAccount(baseURL), token); err == nil {
		cfg, cfgErr := s.Load()
		if cfgErr != nil {
			return "keyring", cfgErr
		}
		if cfg.Token != "" {
			cfg.Token = ""
			if saveErr := s.Save(cfg); saveErr != nil {
				return "keyring", saveErr
			}
		}
		return "keyring", nil
	}

	cfg, err := s.Load()
	if err != nil {
		return "", err
	}
	cfg.Token = token
	if err := s.Save(cfg); err != nil {
		return "", err
	}

	return "config", nil
}

func (s *Store) LoadToken(baseURL string) (string, string, error) {
	baseURL = strings.TrimSpace(baseURL)

	token, err := keyring.Get(keyringService, keyringAccount(baseURL))
	if err == nil && strings.TrimSpace(token) != "" {
		return token, "keyring", nil
	}
	if err != nil && !errors.Is(err, keyring.ErrNotFound) {
		// Ignore non-fatal keyring lookup errors and continue with config fallback.
	}

	cfg, cfgErr := s.Load()
	if cfgErr != nil {
		return "", "", cfgErr
	}
	if strings.TrimSpace(cfg.Token) != "" {
		return cfg.Token, "config", nil
	}

	return "", "", nil
}

func (s *Store) DeleteToken(baseURL string) error {
	baseURL = strings.TrimSpace(baseURL)

	err := keyring.Delete(keyringService, keyringAccount(baseURL))
	if err != nil && !errors.Is(err, keyring.ErrNotFound) {
		// Ignore keyring errors and continue with config fallback cleanup.
	}

	cfg, cfgErr := s.Load()
	if cfgErr != nil {
		return cfgErr
	}
	if cfg.Token != "" {
		cfg.Token = ""
		if saveErr := s.Save(cfg); saveErr != nil {
			return saveErr
		}
	}

	return nil
}

func (s *Store) SaveBaseURL(baseURL string) error {
	cfg, err := s.Load()
	if err != nil {
		return err
	}
	cfg.BaseURL = strings.TrimSpace(baseURL)
	return s.Save(cfg)
}

func keyringAccount(baseURL string) string {
	if baseURL == "" {
		return "default"
	}
	return "base_url:" + strings.TrimSpace(baseURL)
}

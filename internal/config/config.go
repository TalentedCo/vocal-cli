package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	DefaultAPIURL  = "https://chatterboxagent.com/api/v1"
	DefaultProfile = "default"
)

type File struct {
	DefaultProfile string                   `json:"default_profile,omitempty"`
	Profiles       map[string]ProfileConfig `json:"profiles,omitempty"`
}

type ProfileConfig struct {
	APIURL string `json:"api_url,omitempty"`
	APIKey string `json:"api_key,omitempty"`
}

type ResolveOptions struct {
	ConfigPath string
	Profile    string
	APIURL     string
	APIKey     string
	Getenv     func(string) string
}

type Resolved struct {
	Profile    string `json:"profile"`
	APIURL     string `json:"api_url"`
	APIKey     string `json:"-"`
	KeySource  string `json:"key_source"`
	ConfigPath string `json:"config_path"`
}

func DefaultConfigPath() string {
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		return filepath.Join(dir, "vocal", "config.json")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".vocal", "config.json")
	}
	return filepath.Join(".vocal", "config.json")
}

func Load(path string) (File, error) {
	if path == "" {
		path = DefaultConfigPath()
	}
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return File{Profiles: map[string]ProfileConfig{}}, nil
		}
		return File{}, err
	}

	var cfg File
	if err := json.Unmarshal(content, &cfg); err != nil {
		return File{}, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]ProfileConfig{}
	}
	return cfg, nil
}

func Save(path string, cfg File) error {
	if path == "" {
		path = DefaultConfigPath()
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]ProfileConfig{}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	content, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')
	return os.WriteFile(path, content, 0o600)
}

func Resolve(opts ResolveOptions) (Resolved, error) {
	getenv := opts.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}

	path := opts.ConfigPath
	if path == "" {
		path = DefaultConfigPath()
	}

	cfg, err := Load(path)
	if err != nil {
		return Resolved{}, err
	}

	profile := firstNonEmpty(
		opts.Profile,
		getenv("VOCAL_PROFILE"),
		getenv("CHATTERBOX_PROFILE"),
		cfg.DefaultProfile,
		DefaultProfile,
	)

	selected := cfg.Profiles[profile]
	defaultProfile := cfg.Profiles[DefaultProfile]

	apiURL := firstNonEmpty(
		opts.APIURL,
		getenv("VOCAL_API_URL"),
		getenv("CHATTERBOX_API_URL"),
		selected.APIURL,
		defaultProfile.APIURL,
		DefaultAPIURL,
	)

	apiKey, source := resolveKey(opts.APIKey, getenv, selected, defaultProfile)

	return Resolved{
		Profile:    profile,
		APIURL:     strings.TrimRight(apiURL, "/"),
		APIKey:     apiKey,
		KeySource:  source,
		ConfigPath: path,
	}, nil
}

func SaveAPIKey(path, profile, apiURL, apiKey string) (Resolved, error) {
	if strings.TrimSpace(apiKey) == "" {
		return Resolved{}, errors.New("api key is required")
	}
	if profile == "" {
		profile = DefaultProfile
	}
	if apiURL == "" {
		apiURL = DefaultAPIURL
	}

	cfg, err := Load(path)
	if err != nil {
		return Resolved{}, err
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]ProfileConfig{}
	}
	existing := cfg.Profiles[profile]
	existing.APIURL = strings.TrimRight(apiURL, "/")
	existing.APIKey = strings.TrimSpace(apiKey)
	cfg.Profiles[profile] = existing
	if cfg.DefaultProfile == "" {
		cfg.DefaultProfile = profile
	}

	if err := Save(path, cfg); err != nil {
		return Resolved{}, err
	}
	return Resolve(ResolveOptions{ConfigPath: path, Profile: profile})
}

func Logout(path, profile string) error {
	if profile == "" {
		profile = DefaultProfile
	}
	cfg, err := Load(path)
	if err != nil {
		return err
	}
	existing := cfg.Profiles[profile]
	existing.APIKey = ""
	cfg.Profiles[profile] = existing
	return Save(path, cfg)
}

func MaskKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		if len(key) <= 4 {
			return strings.Repeat("*", len(key))
		}
		return key[:2] + "..." + key[len(key)-2:]
	}
	if strings.HasPrefix(key, "sk_") && len(key) > 12 {
		return key[:7] + "..." + key[len(key)-4:]
	}
	return key[:4] + "..." + key[len(key)-4:]
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func resolveKey(cliKey string, getenv func(string) string, selected, defaultProfile ProfileConfig) (string, string) {
	switch {
	case strings.TrimSpace(cliKey) != "":
		return strings.TrimSpace(cliKey), "flag"
	case strings.TrimSpace(getenv("VOCAL_API_KEY")) != "":
		return strings.TrimSpace(getenv("VOCAL_API_KEY")), "env:VOCAL_API_KEY"
	case strings.TrimSpace(getenv("CHATTERBOX_API_KEY")) != "":
		return strings.TrimSpace(getenv("CHATTERBOX_API_KEY")), "env:CHATTERBOX_API_KEY"
	case strings.TrimSpace(selected.APIKey) != "":
		return strings.TrimSpace(selected.APIKey), "profile"
	case strings.TrimSpace(defaultProfile.APIKey) != "":
		return strings.TrimSpace(defaultProfile.APIKey), "default_profile"
	default:
		return "", "none"
	}
}

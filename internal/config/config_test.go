package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePrecedence(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	cfg := File{
		DefaultProfile: "default",
		Profiles: map[string]ProfileConfig{
			"default": {APIURL: "https://default.example/api/v1", APIKey: "default-key"},
			"agent":   {APIURL: "https://profile.example/api/v1", APIKey: "profile-key"},
		},
	}
	if err := Save(configPath, cfg); err != nil {
		t.Fatal(err)
	}

	env := map[string]string{
		"VOCAL_PROFILE": "agent",
		"VOCAL_API_URL": "https://env.example/api/v1",
		"VOCAL_API_KEY": "env-key",
	}
	resolved, err := Resolve(ResolveOptions{
		ConfigPath: configPath,
		Getenv: func(key string) string {
			return env[key]
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Profile != "agent" {
		t.Fatalf("profile = %q, want agent", resolved.Profile)
	}
	if resolved.APIURL != "https://env.example/api/v1" {
		t.Fatalf("api url = %q", resolved.APIURL)
	}
	if resolved.APIKey != "env-key" || resolved.KeySource != "env:VOCAL_API_KEY" {
		t.Fatalf("api key/source = %q/%q", resolved.APIKey, resolved.KeySource)
	}

	resolved, err = Resolve(ResolveOptions{
		ConfigPath: configPath,
		Profile:    "agent",
		APIURL:     "https://flag.example/api/v1",
		APIKey:     "flag-key",
		Getenv:     func(string) string { return "" },
	})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.APIURL != "https://flag.example/api/v1" {
		t.Fatalf("flag api url = %q", resolved.APIURL)
	}
	if resolved.APIKey != "flag-key" || resolved.KeySource != "flag" {
		t.Fatalf("flag api key/source = %q/%q", resolved.APIKey, resolved.KeySource)
	}
}

func TestSaveAPIKeyUsesPrivatePermissionsAndMasks(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	resolved, err := SaveAPIKey(configPath, "agent", "https://api.example/v1", "sk_live_1234567890abcdef")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Profile != "agent" || resolved.APIKey != "sk_live_1234567890abcdef" {
		t.Fatalf("unexpected resolved config: %#v", resolved)
	}
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("config permissions = %o, want 600", got)
	}
	if masked := MaskKey(resolved.APIKey); masked != "sk_live...cdef" {
		t.Fatalf("masked key = %q", masked)
	}
}

func TestLogoutClearsOnlyAPIKey(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	if _, err := SaveAPIKey(configPath, "agent", "https://api.example/v1", "secret-key"); err != nil {
		t.Fatal(err)
	}
	if err := Logout(configPath, "agent"); err != nil {
		t.Fatal(err)
	}
	resolved, err := Resolve(ResolveOptions{
		ConfigPath: configPath,
		Profile:    "agent",
		Getenv:     func(string) string { return "" },
	})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.APIKey != "" {
		t.Fatalf("api key after logout = %q", resolved.APIKey)
	}
	if resolved.APIURL != "https://api.example/v1" {
		t.Fatalf("api url after logout = %q", resolved.APIURL)
	}
}

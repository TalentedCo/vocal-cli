package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	vocalconfig "github.com/TalentedCo/vocal-cli/internal/config"
)

func TestAuthSaveAndStatusJSONMasksKey(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	var stdout, stderr bytes.Buffer
	code := ExecuteWithOptions(context.Background(), []string{"--config", configPath, "auth", "save", "--api-key-stdin", "--json"}, &Options{
		Stdin:  strings.NewReader("sk_live_1234567890abcdef\n"),
		Stdout: &stdout,
		Stderr: &stderr,
		Getenv: func(string) string { return "" },
	})
	if code != ExitOK {
		t.Fatalf("exit = %d stderr=%s", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "1234567890abcdef") {
		t.Fatalf("save output leaked full key: %s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = ExecuteWithOptions(context.Background(), []string{"--config", configPath, "auth", "status", "--json"}, &Options{
		Stdout: &stdout,
		Stderr: &stderr,
		Getenv: func(string) string { return "" },
	})
	if code != ExitOK {
		t.Fatalf("exit = %d stderr=%s", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "1234567890abcdef") {
		t.Fatalf("status output leaked full key: %s", stdout.String())
	}
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	data := envelope["data"].(map[string]any)
	if data["authenticated"] != true {
		t.Fatalf("authenticated = %#v", data["authenticated"])
	}
	if data["api_key_masked"] != "sk_live...cdef" {
		t.Fatalf("masked = %#v", data["api_key_masked"])
	}
}

func TestCallsCreateRequiresAuth(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	var stdout, stderr bytes.Buffer
	code := ExecuteWithOptions(context.Background(), []string{
		"--config", configPath,
		"calls", "create",
		"--phone-number", "+14155550123",
		"--call-objective", "Schedule a demo",
		"--from-name", "Sarah",
		"--json",
	}, &Options{
		Stdout: &stdout,
		Stderr: &stderr,
		Getenv: func(string) string { return "" },
	})
	if code != ExitAuth {
		t.Fatalf("exit = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "auth_error") {
		t.Fatalf("stderr = %s", stderr.String())
	}
}

func TestCallsCreateJSONAndIdempotency(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	var gotIdempotency string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIdempotency = r.Header.Get("Idempotency-Key")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"callId":"call_123","status":"initiated"}`))
	}))
	defer server.Close()

	var stdout, stderr bytes.Buffer
	code := ExecuteWithOptions(context.Background(), []string{
		"--config", configPath,
		"--api-url", server.URL,
		"--api-key", "sk_test_123",
		"--json",
		"calls", "create",
		"--phone-number", "+14155550123",
		"--call-objective", "Schedule a demo",
		"--from-name", "Sarah",
		"--idempotency-key", "idem-123",
	}, &Options{
		Stdout: &stdout,
		Stderr: &stderr,
		Getenv: func(string) string { return "" },
	})
	if code != ExitOK {
		t.Fatalf("exit = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if gotIdempotency != "idem-123" {
		t.Fatalf("idempotency = %q", gotIdempotency)
	}
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope["ok"] != true {
		t.Fatalf("ok = %#v", envelope["ok"])
	}
	data := envelope["data"].(map[string]any)
	if data["callId"] != "call_123" {
		t.Fatalf("callId = %#v", data["callId"])
	}
}

func TestAuthSaveAndLogoutUseResolvedProfileAndAPIURL(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := vocalconfig.Save(configPath, vocalconfig.File{
		DefaultProfile: "agent",
		Profiles: map[string]vocalconfig.ProfileConfig{
			"agent": {APIURL: "https://profile.example/api/v1"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := ExecuteWithOptions(context.Background(), []string{"--config", configPath, "auth", "save", "--api-key-stdin", "--json"}, &Options{
		Stdin:  strings.NewReader("sk_live_abcdef123456\n"),
		Stdout: &stdout,
		Stderr: &stderr,
		Getenv: func(string) string { return "" },
	})
	if code != ExitOK {
		t.Fatalf("save exit = %d stderr=%s", code, stderr.String())
	}

	resolved, err := vocalconfig.Resolve(vocalconfig.ResolveOptions{
		ConfigPath: configPath,
		Getenv:     func(string) string { return "" },
	})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Profile != "agent" {
		t.Fatalf("profile = %q", resolved.Profile)
	}
	if resolved.APIURL != "https://profile.example/api/v1" {
		t.Fatalf("api url = %q", resolved.APIURL)
	}
	if resolved.APIKey != "sk_live_abcdef123456" {
		t.Fatalf("api key = %q", resolved.APIKey)
	}

	stdout.Reset()
	stderr.Reset()
	code = ExecuteWithOptions(context.Background(), []string{"--config", configPath, "auth", "logout", "--json"}, &Options{
		Stdout: &stdout,
		Stderr: &stderr,
		Getenv: func(string) string { return "" },
	})
	if code != ExitOK {
		t.Fatalf("logout exit = %d stderr=%s", code, stderr.String())
	}
	resolved, err = vocalconfig.Resolve(vocalconfig.ResolveOptions{
		ConfigPath: configPath,
		Getenv:     func(string) string { return "" },
	})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Profile != "agent" || resolved.APIKey != "" {
		t.Fatalf("after logout = %#v", resolved)
	}
}

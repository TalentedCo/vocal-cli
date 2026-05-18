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
	"github.com/spf13/cobra"
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

func TestCallsCreateWebhookHeaders(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
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
		"--webhook-url", "https://example.com/webhooks/vocal",
		"--webhook-header", "Authorization=Bearer webhook-secret",
		"--webhook-header", "X-Trace-ID=abc=123",
	}, &Options{
		Stdout: &stdout,
		Stderr: &stderr,
		Getenv: func(string) string { return "" },
	})
	if code != ExitOK {
		t.Fatalf("exit = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if payload["webhookUrl"] != "https://example.com/webhooks/vocal" {
		t.Fatalf("webhookUrl = %#v", payload["webhookUrl"])
	}
	headers := payload["webhookHeaders"].(map[string]any)
	if headers["Authorization"] != "Bearer webhook-secret" || headers["X-Trace-ID"] != "abc=123" {
		t.Fatalf("webhookHeaders = %#v", headers)
	}
}

func TestCallsCreateRejectsMalformedWebhookHeader(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := ExecuteWithOptions(context.Background(), []string{
		"--api-key", "sk_test_123",
		"--json",
		"calls", "create",
		"--phone-number", "+14155550123",
		"--call-objective", "Schedule a demo",
		"--from-name", "Sarah",
		"--webhook-header", "Authorization",
	}, &Options{
		Stdout: &stdout,
		Stderr: &stderr,
		Getenv: func(string) string { return "" },
	})
	if code != ExitUsage {
		t.Fatalf("exit = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "invalid --webhook-header") {
		t.Fatalf("stderr = %s", stderr.String())
	}
}

func TestSignupAgentJSONAndIdempotency(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	var gotAuth, gotIdempotency string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotIdempotency = r.Header.Get("Idempotency-Key")
		if r.URL.Path != "/agent-signups" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"req_123","apiKey":"sk_agent_secret","freeCallLimit":3}`))
	}))
	defer server.Close()

	var stdout, stderr bytes.Buffer
	code := ExecuteWithOptions(context.Background(), []string{
		"--config", configPath,
		"--api-url", server.URL,
		"--json",
		"signup", "agent",
		"--agent-email", "agent@example.com",
		"--agent-name", "Build Agent",
		"--owner-email", "owner@example.com",
		"--project-name", "Demo",
		"--idempotency-key", "idem-agent-123",
	}, &Options{
		Stdout: &stdout,
		Stderr: &stderr,
		Getenv: func(string) string { return "" },
	})
	if code != ExitOK {
		t.Fatalf("exit = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if gotAuth != "" {
		t.Fatalf("authorization = %q", gotAuth)
	}
	if gotIdempotency != "idem-agent-123" {
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
	if data["apiKey"] != "sk_agent_secret" {
		t.Fatalf("apiKey = %#v", data["apiKey"])
	}
}

func TestSignupAgentRequiresIdentityAndIdempotency(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := ExecuteWithOptions(context.Background(), []string{
		"--json",
		"signup", "agent",
		"--agent-email", "agent@example.com",
	}, &Options{
		Stdout: &stdout,
		Stderr: &stderr,
		Getenv: func(string) string { return "" },
	})
	if code != ExitUsage {
		t.Fatalf("exit = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "signup agent requires") {
		t.Fatalf("stderr = %s", stderr.String())
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

func TestVersionCheckUpdateSuggestsInstall(t *testing.T) {
	restoreVersion := setVersionTestData("0.1.0", "1111111111111111111111111111111111111111", "2026-05-01T00:00:00Z")
	defer restoreVersion()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"sha":"2222222222222222222222222222222222222222"}`))
	}))
	defer server.Close()
	updateCheckURL = server.URL

	var stdout, stderr bytes.Buffer
	code := ExecuteWithOptions(context.Background(), []string{"--json", "version", "--check-update"}, &Options{
		Stdout:     &stdout,
		Stderr:     &stderr,
		HTTPClient: server.Client(),
		Getenv:     func(string) string { return "" },
	})
	if code != ExitOK {
		t.Fatalf("exit = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	data := envelope["data"].(map[string]any)
	if data["update_available"] != true {
		t.Fatalf("update_available = %#v", data["update_available"])
	}
	if data["update_command"] != updateInstallCommand {
		t.Fatalf("update_command = %#v", data["update_command"])
	}
}

func TestUpdateCheckReportsNewerCLI(t *testing.T) {
	restoreVersion := setVersionTestData("0.1.0", "1111111111111111111111111111111111111111", "")
	defer restoreVersion()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"sha":"2222222222222222222222222222222222222222"}`))
	}))
	defer server.Close()
	updateCheckURL = server.URL

	var stdout, stderr bytes.Buffer
	code := ExecuteWithOptions(context.Background(), []string{"--json", "update", "--check"}, &Options{
		Stdout:     &stdout,
		Stderr:     &stderr,
		HTTPClient: server.Client(),
		Getenv:     func(string) string { return "" },
	})
	if code != ExitOK {
		t.Fatalf("exit = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	data := envelope["data"].(map[string]any)
	if data["update_available"] != true || data["check_only"] != true {
		t.Fatalf("update check data = %#v", data)
	}
	if data["update_command"] != updateInstallCommand {
		t.Fatalf("update_command = %#v", data["update_command"])
	}
}

func TestUpdateInstallsWhenNewerCLIAvailable(t *testing.T) {
	restoreVersion := setVersionTestData("0.1.0", "1111111111111111111111111111111111111111", "")
	defer restoreVersion()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"sha":"2222222222222222222222222222222222222222"}`))
	}))
	defer server.Close()
	updateCheckURL = server.URL

	called := false
	oldRunner := runUpdateInstall
	runUpdateInstall = func(cmd *cobra.Command) error {
		called = true
		return nil
	}
	defer func() { runUpdateInstall = oldRunner }()

	var stdout, stderr bytes.Buffer
	code := ExecuteWithOptions(context.Background(), []string{"--json", "update"}, &Options{
		Stdout:     &stdout,
		Stderr:     &stderr,
		HTTPClient: server.Client(),
		Getenv:     func(string) string { return "" },
	})
	if code != ExitOK {
		t.Fatalf("exit = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !called {
		t.Fatal("expected update installer to run")
	}
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	data := envelope["data"].(map[string]any)
	if data["installed"] != true {
		t.Fatalf("installed = %#v", data["installed"])
	}
}

func TestDoctorCheckUpdatesReportsNewerCLI(t *testing.T) {
	restoreVersion := setVersionTestData("0.1.0", "1111111111111111111111111111111111111111", "")
	defer restoreVersion()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"sha":"2222222222222222222222222222222222222222"}`))
	}))
	defer server.Close()
	updateCheckURL = server.URL

	var stdout, stderr bytes.Buffer
	code := ExecuteWithOptions(context.Background(), []string{"--config", filepath.Join(t.TempDir(), "config.json"), "--json", "doctor", "--check-updates"}, &Options{
		Stdout:     &stdout,
		Stderr:     &stderr,
		HTTPClient: server.Client(),
		Getenv:     func(string) string { return "" },
	})
	if code != ExitOK {
		t.Fatalf("exit = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	data := envelope["data"].(map[string]any)
	checks := data["checks"].([]any)
	for _, rawCheck := range checks {
		check := rawCheck.(map[string]any)
		if check["name"] == "cli_update" {
			if check["status"] != "warn" || !strings.Contains(check["message"].(string), updateInstallCommand) {
				t.Fatalf("cli_update check = %#v", check)
			}
			return
		}
	}
	t.Fatalf("cli_update check missing: %#v", checks)
}

func setVersionTestData(testVersion, testCommit, testDate string) func() {
	oldVersion, oldCommit, oldDate, oldURL := version, commit, buildDate, updateCheckURL
	version = testVersion
	commit = testCommit
	buildDate = testDate
	return func() {
		version = oldVersion
		commit = oldCommit
		buildDate = oldDate
		updateCheckURL = oldURL
	}
}

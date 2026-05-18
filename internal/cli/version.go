package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/TalentedCo/vocal-cli/internal/output"
	"github.com/spf13/cobra"
)

const updateInstallCommand = "go install github.com/TalentedCo/vocal-cli/cmd/vocal@latest"

var (
	version        = "dev"
	commit         = ""
	buildDate      = ""
	updateCheckURL = "https://api.github.com/repos/TalentedCo/vocal-cli/commits/main"
)

type versionInfo struct {
	Version          string `json:"version"`
	Commit           string `json:"commit"`
	Date             string `json:"date,omitempty"`
	LatestCommit     string `json:"latest_commit,omitempty"`
	UpdateAvailable  bool   `json:"update_available"`
	UpdateCommand    string `json:"update_command,omitempty"`
	UpdateCheckError string `json:"update_check_error,omitempty"`
}

func newVersionCommand(opts *Options) *cobra.Command {
	var checkUpdate bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show VOCAL CLI version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			info := currentVersionInfo()
			if checkUpdate {
				latestCommit, err := fetchLatestCommit(cmd.Context(), httpClient(opts))
				if err != nil {
					info.UpdateCheckError = err.Error()
				} else {
					applyLatestCommit(&info, latestCommit)
				}
			}
			return output.Success(opts.Stdout, opts.JSON, info, versionSummary(info))
		},
	}
	cmd.Flags().BoolVar(&checkUpdate, "check-update", false, "check whether a newer CLI build is available")
	return cmd
}

func currentVersionInfo() versionInfo {
	info := versionInfo{
		Version: version,
		Commit:  commit,
		Date:    buildDate,
	}
	if info.Version == "" {
		info.Version = "dev"
	}

	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		if (info.Version == "" || info.Version == "dev") && buildInfo.Main.Version != "" && buildInfo.Main.Version != "(devel)" {
			info.Version = buildInfo.Main.Version
		}
		for _, setting := range buildInfo.Settings {
			switch setting.Key {
			case "vcs.revision":
				if info.Commit == "" {
					info.Commit = setting.Value
				}
			case "vcs.time":
				if info.Date == "" {
					info.Date = setting.Value
				}
			}
		}
	}

	if info.Commit == "" {
		info.Commit = "unknown"
	}
	return info
}

func fetchLatestCommit(ctx context.Context, client *http.Client) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, updateCheckURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "vocal-cli")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("GitHub update check returned HTTP %d", resp.StatusCode)
	}

	var payload struct {
		SHA string `json:"sha"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.SHA) == "" {
		return "", fmt.Errorf("GitHub update check response did not include a commit SHA")
	}
	return strings.TrimSpace(payload.SHA), nil
}

func applyLatestCommit(info *versionInfo, latestCommit string) {
	info.LatestCommit = latestCommit
	if info.Commit != "" && info.Commit != "unknown" && !sameCommit(info.Commit, latestCommit) {
		info.UpdateAvailable = true
		info.UpdateCommand = updateInstallCommand
	}
}

func sameCommit(current, latest string) bool {
	current = strings.ToLower(strings.TrimSpace(current))
	latest = strings.ToLower(strings.TrimSpace(latest))
	if current == "" || latest == "" {
		return false
	}
	return current == latest || strings.HasPrefix(current, latest) || strings.HasPrefix(latest, current)
}

func shortCommit(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 12 {
		return value[:12]
	}
	return value
}

func versionSummary(info versionInfo) string {
	summary := fmt.Sprintf("vocal %s (%s)", info.Version, shortCommit(info.Commit))
	if info.UpdateCheckError != "" {
		return summary + "; update check failed: " + info.UpdateCheckError
	}
	if info.UpdateAvailable {
		return fmt.Sprintf("%s; newer CLI available (%s), run: %s", summary, shortCommit(info.LatestCommit), info.UpdateCommand)
	}
	if info.LatestCommit != "" {
		return fmt.Sprintf("%s; latest checked commit is %s", summary, shortCommit(info.LatestCommit))
	}
	return summary
}

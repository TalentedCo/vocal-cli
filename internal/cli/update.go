package cli

import (
	"fmt"
	"os/exec"

	"github.com/TalentedCo/vocal-cli/internal/output"
	"github.com/spf13/cobra"
)

type updateResult struct {
	CurrentVersion  string `json:"current_version"`
	CurrentCommit   string `json:"current_commit"`
	LatestCommit    string `json:"latest_commit,omitempty"`
	UpdateAvailable bool   `json:"update_available"`
	UpdateCommand   string `json:"update_command"`
	Installed       bool   `json:"installed"`
	CheckOnly       bool   `json:"check_only"`
}

var runUpdateInstall = func(cmd *cobra.Command) error {
	installCmd := exec.CommandContext(cmd.Context(), "go", "install", "github.com/TalentedCo/vocal-cli/cmd/vocal@latest")
	return installCmd.Run()
}

func newUpdateCommand(opts *Options) *cobra.Command {
	var checkOnly bool
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Check for and install a newer VOCAL CLI build",
		Long: `Check for and install a newer VOCAL CLI build.

VOCAL CLI does not currently publish platform release binaries, so this command
uses the Go toolchain update path:

  go install github.com/TalentedCo/vocal-cli/cmd/vocal@latest

Use --check to only ping GitHub for the latest default-branch commit and report
whether the local binary appears stale.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			info := currentVersionInfo()
			latestCommit, err := fetchLatestCommit(cmd.Context(), httpClient(opts))
			if err != nil {
				return exitError(ExitNetwork, "failed to check for VOCAL CLI updates", err)
			}
			applyLatestCommit(&info, latestCommit)

			result := updateResult{
				CurrentVersion:  info.Version,
				CurrentCommit:   info.Commit,
				LatestCommit:    info.LatestCommit,
				UpdateAvailable: info.UpdateAvailable,
				UpdateCommand:   updateInstallCommand,
				CheckOnly:       checkOnly,
			}

			if checkOnly {
				return output.Success(opts.Stdout, opts.JSON, result, updateSummary(result))
			}
			if !info.UpdateAvailable && info.Commit != "unknown" {
				return output.Success(opts.Stdout, opts.JSON, result, updateSummary(result))
			}
			if err := runUpdateInstall(cmd); err != nil {
				return exitError(ExitGeneral, fmt.Sprintf("failed to run %s", updateInstallCommand), err)
			}
			result.Installed = true
			return output.Success(opts.Stdout, opts.JSON, result, updateSummary(result))
		},
	}
	cmd.Flags().BoolVar(&checkOnly, "check", false, "check for a newer CLI build without installing")
	return cmd
}

func updateSummary(result updateResult) string {
	if result.UpdateAvailable {
		if result.Installed {
			return fmt.Sprintf("Installed latest VOCAL CLI build from %s", shortCommit(result.LatestCommit))
		}
		return fmt.Sprintf("Newer VOCAL CLI build available (%s); run: %s", shortCommit(result.LatestCommit), result.UpdateCommand)
	}
	if result.LatestCommit != "" {
		return fmt.Sprintf("VOCAL CLI is up to date at %s", shortCommit(result.LatestCommit))
	}
	return "VOCAL CLI update status is unknown"
}

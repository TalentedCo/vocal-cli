package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	vocalconfig "github.com/TalentedCo/vocal-cli/internal/config"
	"github.com/TalentedCo/vocal-cli/internal/output"
	"github.com/spf13/cobra"
)

type authStatus struct {
	Authenticated bool   `json:"authenticated"`
	Profile       string `json:"profile"`
	APIURL        string `json:"api_url"`
	KeySource     string `json:"key_source"`
	APIKeyMasked  string `json:"api_key_masked,omitempty"`
	ConfigPath    string `json:"config_path"`
}

func newAuthCommand(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage stored VOCAL API credentials",
	}
	cmd.AddCommand(newAuthSaveCommand(opts))
	cmd.AddCommand(newAuthStatusCommand(opts))
	cmd.AddCommand(newAuthLogoutCommand(opts))
	return cmd
}

func newAuthSaveCommand(opts *Options) *cobra.Command {
	var apiKeyFile string
	var apiKeyStdin bool
	cmd := &cobra.Command{
		Use:   "save",
		Short: "Save an API key for the active profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			key, err := readAPIKey(opts, opts.APIKey, apiKeyFile, apiKeyStdin)
			if err != nil {
				return exitError(ExitUsage, err.Error(), err)
			}
			apiURL := opts.APIURL
			if apiURL == "" {
				apiURL = vocalconfig.DefaultAPIURL
			}
			profile := opts.Profile
			if profile == "" {
				profile = vocalconfig.DefaultProfile
			}
			resolved, err := vocalconfig.SaveAPIKey(opts.ConfigPath, profile, apiURL, key)
			if err != nil {
				return exitError(ExitConfig, "failed to save API key", err)
			}
			data := authStatusFromResolved(resolved)
			return output.Success(opts.Stdout, opts.JSON, data, fmt.Sprintf("Saved API key for profile %q (%s)", data.Profile, data.APIKeyMasked))
		},
	}
	cmd.Flags().StringVar(&apiKeyFile, "api-key-file", "", "read API key from file")
	cmd.Flags().BoolVar(&apiKeyStdin, "api-key-stdin", false, "read API key from stdin")
	return cmd
}

func newAuthStatusCommand(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show auth status without revealing secrets",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, err := resolve(opts)
			if err != nil {
				return err
			}
			data := authStatusFromResolved(resolved)
			summary := "Not authenticated"
			if data.Authenticated {
				summary = fmt.Sprintf("Authenticated as profile %q with %s", data.Profile, data.APIKeyMasked)
			}
			return output.Success(opts.Stdout, opts.JSON, data, summary)
		},
	}
}

func newAuthLogoutCommand(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove the stored API key for the active profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			profile := opts.Profile
			if profile == "" {
				profile = vocalconfig.DefaultProfile
			}
			if err := vocalconfig.Logout(opts.ConfigPath, profile); err != nil {
				return exitError(ExitConfig, "failed to remove API key", err)
			}
			return output.Success(opts.Stdout, opts.JSON, map[string]string{"profile": profile}, fmt.Sprintf("Removed API key for profile %q", profile))
		},
	}
}

func newStatusCommand(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show resolved profile, API URL, and auth state",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, err := resolve(opts)
			if err != nil {
				return err
			}
			data := authStatusFromResolved(resolved)
			summary := fmt.Sprintf("Profile %q points at %s", data.Profile, data.APIURL)
			if data.Authenticated {
				summary += " with an API key"
			} else {
				summary += " without an API key"
			}
			return output.Success(opts.Stdout, opts.JSON, data, summary)
		},
	}
}

func readAPIKey(opts *Options, apiKey, apiKeyFile string, apiKeyStdin bool) (string, error) {
	sources := 0
	if strings.TrimSpace(apiKey) != "" {
		sources++
	}
	if apiKeyFile != "" {
		sources++
	}
	if apiKeyStdin {
		sources++
	}
	if sources != 1 {
		return "", fmt.Errorf("provide exactly one of --api-key, --api-key-file, or --api-key-stdin")
	}

	var content []byte
	var err error
	switch {
	case apiKey != "":
		content = []byte(apiKey)
	case apiKeyFile != "":
		content, err = os.ReadFile(apiKeyFile)
	case apiKeyStdin:
		content, err = io.ReadAll(opts.Stdin)
	}
	if err != nil {
		return "", err
	}
	key := strings.TrimSpace(string(content))
	if key == "" {
		return "", fmt.Errorf("api key is empty")
	}
	return key, nil
}

func authStatusFromResolved(resolved vocalconfig.Resolved) authStatus {
	return authStatus{
		Authenticated: resolved.APIKey != "",
		Profile:       resolved.Profile,
		APIURL:        resolved.APIURL,
		KeySource:     resolved.KeySource,
		APIKeyMasked:  vocalconfig.MaskKey(resolved.APIKey),
		ConfigPath:    resolved.ConfigPath,
	}
}

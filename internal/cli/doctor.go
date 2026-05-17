package cli

import (
	"fmt"
	"net/url"
	"os"

	vocalclient "github.com/TalentedCo/vocal-cli/internal/client"
	"github.com/TalentedCo/vocal-cli/internal/output"
	"github.com/spf13/cobra"
)

type doctorResult struct {
	Profile string        `json:"profile"`
	APIURL  string        `json:"api_url"`
	Checks  []doctorCheck `json:"checks"`
}

type doctorCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

func newDoctorCommand(opts *Options) *cobra.Command {
	var checkAPI bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run local diagnostics for agent automation",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, err := resolve(opts)
			if err != nil {
				return err
			}
			result := doctorResult{
				Profile: resolved.Profile,
				APIURL:  resolved.APIURL,
				Checks:  localDoctorChecks(resolved.ConfigPath, resolved.APIURL, resolved.APIKey != ""),
			}
			if checkAPI {
				check := doctorCheck{Name: "api_auth", Status: "pass", Message: "API key authenticated"}
				if resolved.APIKey == "" {
					check = doctorCheck{Name: "api_auth", Status: "fail", Message: "No API key configured"}
				} else {
					apiClient := vocalclient.New(resolved.APIURL, resolved.APIKey, httpClient(opts))
					if _, err := apiClient.ListCalls(cmd.Context(), "", 1, 0); err != nil {
						check = doctorCheck{Name: "api_auth", Status: "fail", Message: err.Error()}
					}
				}
				result.Checks = append(result.Checks, check)
			}
			summary := "Doctor completed"
			return output.Success(opts.Stdout, opts.JSON, result, summary)
		},
	}
	cmd.Flags().BoolVar(&checkAPI, "check-api", false, "make a network request to verify API authentication")
	return cmd
}

func localDoctorChecks(configPath, apiURL string, hasKey bool) []doctorCheck {
	checks := []doctorCheck{}
	if _, err := url.ParseRequestURI(apiURL); err != nil {
		checks = append(checks, doctorCheck{Name: "api_url", Status: "fail", Message: err.Error()})
	} else {
		checks = append(checks, doctorCheck{Name: "api_url", Status: "pass", Message: apiURL})
	}

	if info, err := os.Stat(configPath); err != nil {
		if os.IsNotExist(err) {
			checks = append(checks, doctorCheck{Name: "config_file", Status: "warn", Message: fmt.Sprintf("%s does not exist yet", configPath)})
		} else {
			checks = append(checks, doctorCheck{Name: "config_file", Status: "fail", Message: err.Error()})
		}
	} else if info.Mode().Perm()&0o077 != 0 {
		checks = append(checks, doctorCheck{Name: "config_permissions", Status: "warn", Message: fmt.Sprintf("%s should not be readable by other users", configPath)})
	} else {
		checks = append(checks, doctorCheck{Name: "config_permissions", Status: "pass", Message: "config file permissions are private"})
	}

	if hasKey {
		checks = append(checks, doctorCheck{Name: "api_key", Status: "pass", Message: "API key is configured"})
	} else {
		checks = append(checks, doctorCheck{Name: "api_key", Status: "warn", Message: "No API key configured"})
	}
	return checks
}

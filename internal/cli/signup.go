package cli

import (
	"strings"

	vocalclient "github.com/TalentedCo/vocal-cli/internal/client"
	"github.com/TalentedCo/vocal-cli/internal/output"
	"github.com/spf13/cobra"
)

func newSignupCommand(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "signup",
		Short: "Agent-first VOCAL signup flows",
	}
	cmd.AddCommand(newSignupAgentCommand(opts))
	return cmd
}

func newSignupAgentCommand(opts *Options) *cobra.Command {
	var request vocalclient.AgentSignupRequest
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Request a constrained provisional VOCAL account for a human owner",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(request.AgentEmail) == "" ||
				strings.TrimSpace(request.OwnerEmail) == "" ||
				strings.TrimSpace(request.IdempotencyKey) == "" {
				return exitError(ExitUsage, "signup agent requires --agent-email, --owner-email, and --idempotency-key", nil)
			}

			resolved, err := resolve(opts)
			if err != nil {
				return err
			}

			apiClient := vocalclient.New(resolved.APIURL, "", httpClient(opts))
			data, err := apiClient.CreateAgentSignup(cmd.Context(), request)
			if err != nil {
				return classifyClientError(err)
			}
			return output.Success(opts.Stdout, opts.JSON, data, summarizeAgentSignup(data))
		},
	}
	cmd.Flags().StringVar(&request.AgentEmail, "agent-email", "", "agent email initiating the signup")
	cmd.Flags().StringVar(&request.AgentName, "agent-name", "", "optional agent display name")
	cmd.Flags().StringVar(&request.OwnerEmail, "owner-email", "", "human owner email")
	cmd.Flags().StringVar(&request.ProjectName, "project-name", "", "optional project or organization name")
	cmd.Flags().StringVar(&request.IdempotencyKey, "idempotency-key", "", "required retry-safe idempotency key")
	return cmd
}

func summarizeAgentSignup(data any) string {
	payload, ok := data.(map[string]any)
	if !ok {
		return "Agent signup requested"
	}
	if value, ok := payload["apiKey"].(string); ok && value != "" {
		return "Agent signup created with provisional API key"
	}
	if duplicate, ok := payload["duplicate"].(bool); ok && duplicate {
		return "Agent signup already exists"
	}
	return "Agent signup requested for owner acceptance"
}

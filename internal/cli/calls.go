package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	vocalclient "github.com/TalentedCo/vocal-cli/internal/client"
	"github.com/TalentedCo/vocal-cli/internal/output"
	"github.com/spf13/cobra"
)

func newCallsCommand(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "calls",
		Short: "Create and inspect VOCAL calls",
	}
	cmd.AddCommand(newCallsCreateCommand(opts))
	cmd.AddCommand(newCallsGetCommand(opts))
	cmd.AddCommand(newCallsListCommand(opts))
	cmd.AddCommand(newCallsStreamCommand(opts))
	return cmd
}

func newCallsCreateCommand(opts *Options) *cobra.Command {
	var request vocalclient.CreateCallRequest
	var idempotencyKey string
	var webhookHeaders []string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an outbound call",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(request.PhoneNumber) == "" || strings.TrimSpace(request.CallObjective) == "" || strings.TrimSpace(request.FromName) == "" {
				return exitError(ExitUsage, "calls create requires --phone-number, --call-objective, and --from-name", nil)
			}
			headers, err := parseWebhookHeaders(webhookHeaders)
			if err != nil {
				return exitError(ExitUsage, err.Error(), err)
			}
			request.WebhookHeaders = headers
			resolved, err := resolve(opts)
			if err != nil {
				return err
			}
			if resolved.APIKey == "" {
				return exitError(ExitAuth, "no API key configured; run vocal auth save or set VOCAL_API_KEY", nil)
			}
			apiClient := vocalclient.New(resolved.APIURL, resolved.APIKey, httpClient(opts))
			data, err := apiClient.CreateCall(cmd.Context(), request, idempotencyKey)
			if err != nil {
				return classifyClientError(err)
			}
			return output.Success(opts.Stdout, opts.JSON, data, summarizeCallCreate(data))
		},
	}
	cmd.Flags().StringVar(&request.ExternalID, "external-id", "", "client reference ID")
	cmd.Flags().StringVar(&request.PhoneNumber, "phone-number", "", "phone number to call")
	cmd.Flags().StringVar(&request.CallObjective, "call-objective", "", "specific call objective")
	cmd.Flags().StringVar(&request.FromName, "from-name", "", "person or company calling on behalf of")
	cmd.Flags().StringVar(&request.RecipientName, "recipient-name", "", "optional recipient name")
	cmd.Flags().StringVar(&request.Voice, "voice", "", "voice name, such as Elliot or Paige")
	cmd.Flags().IntVar(&request.MaxRetries, "max-retries", 0, "maximum retries; defaults to 0 for agent safety")
	cmd.Flags().BoolVar(&request.SkipObjectiveValidation, "skip-objective-validation", false, "skip objective completeness validation")
	cmd.Flags().StringVar(&request.WebhookURL, "webhook-url", "", "webhook URL for final call results")
	cmd.Flags().StringArrayVar(&webhookHeaders, "webhook-header", nil, "custom webhook header KEY=VALUE for final call results; repeatable")
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "idempotency key sent as Idempotency-Key")
	return cmd
}

func parseWebhookHeaders(values []string) (map[string]string, error) {
	if len(values) == 0 {
		return nil, nil
	}

	headers := make(map[string]string, len(values))
	for _, raw := range values {
		parts := strings.SplitN(raw, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid --webhook-header %q; expected KEY=VALUE", raw)
		}

		key := strings.TrimSpace(parts[0])
		if key == "" {
			return nil, fmt.Errorf("invalid --webhook-header %q; header key is required", raw)
		}
		if !isHTTPHeaderToken(key) {
			return nil, fmt.Errorf("invalid --webhook-header %q; header key contains invalid characters", raw)
		}

		headers[key] = strings.TrimSpace(parts[1])
	}
	return headers, nil
}

func isHTTPHeaderToken(value string) bool {
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case strings.ContainsRune("!#$%&'*+-.^_`|~", r):
		default:
			return false
		}
	}
	return value != ""
}

func newCallsGetCommand(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "get CALL_ID",
		Short: "Get call status/details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			apiClient, err := resolvedClient(opts)
			if err != nil {
				return err
			}
			data, err := apiClient.GetCall(cmd.Context(), args[0])
			if err != nil {
				return classifyClientError(err)
			}
			return output.Success(opts.Stdout, opts.JSON, data, summarizeCall(data))
		},
	}
}

func newCallsListCommand(opts *Options) *cobra.Command {
	var status string
	var limit, offset int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List calls visible to the active API key",
		RunE: func(cmd *cobra.Command, args []string) error {
			apiClient, err := resolvedClient(opts)
			if err != nil {
				return err
			}
			data, err := apiClient.ListCalls(cmd.Context(), status, limit, offset)
			if err != nil {
				return classifyClientError(err)
			}
			return output.Success(opts.Stdout, opts.JSON, data, "Calls fetched")
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "filter by call status")
	cmd.Flags().IntVar(&limit, "limit", 10, "maximum calls to return")
	cmd.Flags().IntVar(&offset, "offset", 0, "offset for pagination")
	return cmd
}

func newCallsStreamCommand(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "stream CALL_ID",
		Short: "Stream call events as text or JSON lines",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			apiClient, err := resolvedClient(opts)
			if err != nil {
				return err
			}
			resp, err := apiClient.StreamCall(cmd.Context(), args[0])
			if err != nil {
				return classifyClientError(err)
			}
			defer resp.Body.Close()
			return streamEvents(opts, resp.Body)
		},
	}
}

func resolvedClient(opts *Options) (*vocalclient.Client, error) {
	resolved, err := resolve(opts)
	if err != nil {
		return nil, err
	}
	if resolved.APIKey == "" {
		return nil, exitError(ExitAuth, "no API key configured; run vocal auth save or set VOCAL_API_KEY", nil)
	}
	return vocalclient.New(resolved.APIURL, resolved.APIKey, httpClient(opts)), nil
}

func summarizeCallCreate(data any) string {
	if value, ok := fieldString(data, "callId"); ok {
		return "Created call " + value
	}
	if value, ok := fieldString(data, "id"); ok {
		return "Created call " + value
	}
	return "Call created"
}

func summarizeCall(data any) string {
	if value, ok := fieldString(data, "id"); ok {
		return "Call " + value
	}
	if value, ok := fieldString(data, "callId"); ok {
		return "Call " + value
	}
	return "Call fetched"
}

func fieldString(data any, field string) (string, bool) {
	payload, ok := data.(map[string]any)
	if !ok {
		return "", false
	}
	value, ok := payload[field].(string)
	return value, ok && value != ""
}

func streamEvents(opts *Options, reader io.Reader) error {
	scanner := bufio.NewScanner(reader)
	var eventName string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event:") {
			eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		rawData := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if opts.JSON {
			var payload any
			if err := json.Unmarshal([]byte(rawData), &payload); err != nil {
				payload = rawData
			}
			if err := output.JSONLine(opts.Stdout, map[string]any{"event": eventName, "data": payload}); err != nil {
				return err
			}
		} else {
			if _, err := fmt.Fprintf(opts.Stdout, "%s %s\n", eventName, rawData); err != nil {
				return err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return exitError(ExitNetwork, err.Error(), err)
	}
	return nil
}

package cli

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	vocalconfig "github.com/TalentedCo/vocal-cli/internal/config"
	"github.com/TalentedCo/vocal-cli/internal/output"
	"github.com/spf13/cobra"
)

type Options struct {
	Profile        string
	APIURL         string
	APIKey         string
	ConfigPath     string
	JSON           bool
	NonInteractive bool
	Timeout        time.Duration

	Stdin      io.Reader
	Stdout     io.Writer
	Stderr     io.Writer
	HTTPClient *http.Client
	Getenv     func(string) string
}

func Execute(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return ExecuteWithOptions(ctx, args, &Options{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
		Getenv: os.Getenv,
	})
}

func ExecuteWithOptions(ctx context.Context, args []string, opts *Options) int {
	if opts == nil {
		opts = &Options{}
	}
	if opts.Stdin == nil {
		opts.Stdin = os.Stdin
	}
	if opts.Stdout == nil {
		opts.Stdout = io.Discard
	}
	if opts.Stderr == nil {
		opts.Stderr = io.Discard
	}
	if opts.Getenv == nil {
		opts.Getenv = os.Getenv
	}
	if opts.Timeout == 0 {
		opts.Timeout = 30 * time.Second
	}

	cmd := NewRootCommand(opts)
	cmd.SetArgs(args)
	if err := cmd.ExecuteContext(ctx); err != nil {
		code := exitCode(err)
		_ = output.Error(opts.Stderr, opts.JSON, errorCode(err), err.Error())
		return code
	}
	return ExitOK
}

func NewRootCommand(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "vocal",
		Short:         "Agent-friendly CLI for ChatterBox VOCAL",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.PersistentFlags().StringVar(&opts.Profile, "profile", "", "named profile to use")
	cmd.PersistentFlags().StringVar(&opts.APIURL, "api-url", "", "VOCAL API base URL")
	cmd.PersistentFlags().StringVar(&opts.APIKey, "api-key", "", "API key; prefer VOCAL_API_KEY or auth save for automation")
	cmd.PersistentFlags().StringVar(&opts.ConfigPath, "config", "", "config file path")
	cmd.PersistentFlags().BoolVar(&opts.JSON, "json", false, "emit structured JSON")
	cmd.PersistentFlags().BoolVar(&opts.NonInteractive, "non-interactive", false, "disable prompts and require explicit flags")
	cmd.PersistentFlags().DurationVar(&opts.Timeout, "timeout", 30*time.Second, "HTTP timeout")

	cmd.AddCommand(newAuthCommand(opts))
	cmd.AddCommand(newStatusCommand(opts))
	cmd.AddCommand(newDoctorCommand(opts))
	cmd.AddCommand(newCallsCommand(opts))
	cmd.AddCommand(newSignupCommand(opts))
	return cmd
}

func resolve(opts *Options) (vocalconfig.Resolved, error) {
	resolved, err := vocalconfig.Resolve(vocalconfig.ResolveOptions{
		ConfigPath: opts.ConfigPath,
		Profile:    opts.Profile,
		APIURL:     opts.APIURL,
		APIKey:     opts.APIKey,
		Getenv:     opts.Getenv,
	})
	if err != nil {
		return vocalconfig.Resolved{}, exitError(ExitConfig, "failed to resolve config", err)
	}
	return resolved, nil
}

func httpClient(opts *Options) *http.Client {
	if opts.HTTPClient != nil {
		return opts.HTTPClient
	}
	return &http.Client{Timeout: opts.Timeout}
}

func classifyClientError(err error) error {
	if err == nil {
		return nil
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return exitError(ExitNetwork, err.Error(), err)
	}
	return exitError(ExitAPI, err.Error(), err)
}

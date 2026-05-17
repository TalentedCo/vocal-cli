# ChatterBox VOCAL CLI

`vocal` is the public, agent-friendly CLI for [ChatterBox VOCAL](https://chatterboxagent.com), an API for outbound AI phone calls.

The CLI is designed for non-interactive automation: explicit config precedence, JSON output, deterministic exit codes, safe API-key handling, diagnostics, shell completions, and idempotency headers on mutating commands.

## Install

```bash
go install github.com/TalentedCo/vocal-cli/cmd/vocal@latest
```

Confirm the binary is available:

```bash
vocal --help
vocal doctor --json
```

## Configure Auth

Use stdin or a file when possible so API keys do not land in shell history:

```bash
printf '%s\n' "$VOCAL_API_KEY" | vocal auth save --api-key-stdin --profile default
vocal auth status --json
```

For one-off automation, environment variables or explicit flags are supported:

```bash
VOCAL_API_KEY=sk_live_... vocal calls list --json
vocal --api-key sk_live_... calls list --json
```

Stored credentials are written to `~/.config/vocal/config.json` with `0600` permissions. CLI output only shows masked key fingerprints such as `sk_live...abcd`.

## Config Precedence

Highest to lowest:

1. CLI flags: `--api-key`, `--api-url`, `--profile`, `--config`
2. Environment variables: `VOCAL_API_KEY`, `VOCAL_API_URL`, `VOCAL_PROFILE`
3. Compatibility environment variables: `CHATTERBOX_API_KEY`, `CHATTERBOX_API_URL`, `CHATTERBOX_PROFILE`
4. Selected named profile in the config file
5. Default profile in the config file
6. Built-in default API URL: `https://chatterboxagent.com/api/v1`

## Agent Workflow

Run diagnostics:

```bash
vocal doctor --json
```

Create a call with explicit non-interactive flags:

```bash
vocal calls create \
  --json \
  --non-interactive \
  --idempotency-key "agent-run-$(date +%s)" \
  --phone-number "+14155551234" \
  --from-name "Sarah from ABC Corp" \
  --recipient-name "John Smith" \
  --call-objective "Schedule a 30-minute Zoom demo. Sarah is available Tuesday 2-5pm ET or Wednesday 10am-2pm ET. Send the invite to sarah@example.com." \
  --max-retries 0
```

Inspect and stream status:

```bash
vocal calls get CALL_ID --json
vocal calls stream CALL_ID --json
```

## Commands

```text
vocal auth save --api-key-stdin|--api-key-file PATH|--api-key VALUE
vocal auth status
vocal auth logout
vocal status
vocal doctor [--check-api]
vocal calls create --phone-number ... --call-objective ... --from-name ...
vocal calls get CALL_ID
vocal calls list [--status STATUS] [--limit N] [--offset N]
vocal calls stream CALL_ID
vocal completion bash|zsh|fish|powershell
```

## JSON Output

Use `--json` for automation. Successful commands use:

```json
{
  "ok": true,
  "data": {},
  "summary": "Call created"
}
```

Errors use:

```json
{
  "ok": false,
  "error": {
    "code": "auth_error",
    "message": "no API key configured; run vocal auth save or set VOCAL_API_KEY"
  }
}
```

## Exit Codes

| Code | Meaning |
| ---- | ------- |
| `0` | Success |
| `1` | General error |
| `2` | Usage error |
| `3` | Config error |
| `4` | Auth error |
| `5` | VOCAL API error |
| `6` | Network error |

`auth status`, `status`, and `doctor` are safe to run in fresh environments and return structured state instead of revealing secrets.

## Idempotency

Mutating commands accept `--idempotency-key` and send it as the `Idempotency-Key` HTTP header. The CLI exposes this foundation for agent-first signup and other retry-safe workflows.

## Shell Completions

```bash
vocal completion bash > /usr/local/etc/bash_completion.d/vocal
vocal completion zsh > "${fpath[1]}/_vocal"
vocal completion fish > ~/.config/fish/completions/vocal.fish
```

## Development

```bash
go test ./...
go vet ./...
```

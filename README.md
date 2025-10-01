# x-cli

`x-cli` is a simple command-line tool for posting text or media updates to X (formerly Twitter) with scheduling capabilities. It uses Go's standard library (plus Cobra for CLI ergonomics) and signs OAuth 1.0a requests manually: tweets are created through the v2 `POST /2/tweets` endpoint, and media uploads go through the v1.1 media API.

> **Note:** You still need legacy-style user credentials with the `tweet.write` capability. If your keys are limited to the newer Essential tier, the platform rejects write attempts with HTTP 403 errors.

## Requirements

- Linux system with network access to the X API endpoints
- Go 1.22 or newer installed (`go version`)
- Twitter/X API key, API secret, access token, and access secret with permissions to post tweets (OAuth 1.0a user context)

## Installation

Clone the repository and change into it:

```bash
git clone https://github.com/limo39/x-cli.git
cd x-cli
```

Install the binary locally (optional):

```bash
go install ./...
```

This places the executable in `$GOBIN` (or `$GOPATH/bin`). You can also run the tool directly with `go run .` without installing.

### Building standalone binaries

To produce platform-specific binaries without installing them globally, use Go's cross-compilation support. Run these commands from the project root:

#### Linux (x86_64)

```bash
GOOS=linux GOARCH=amd64 go build -o dist/x-cli-linux-amd64
```

#### macOS (Apple Silicon)

```bash
GOOS=darwin GOARCH=arm64 go build -o dist/x-cli-darwin-arm64
```

#### macOS (Intel)

```bash
GOOS=darwin GOARCH=amd64 go build -o dist/x-cli-darwin-amd64
```

Adjust `GOARCH` if you need other architectures (e.g. `386`, `arm`). The compiled binaries can be copied to any machine with the matching OS/architecture and run directly.

## Configuration

The CLI looks for credentials in a JSON config file or fallbacks to environment variables. Environment variables take precedence when both are set.

### Option 1: JSON config file

1. Create the configuration directory:
   ```bash
   mkdir -p ~/.x-cli
   ```
2. Create `~/.x-cli/config.json` with your credentials:
   ```json
   {
     "api_key": "YOUR_API_KEY",
     "api_secret": "YOUR_API_SECRET",
     "access_token": "YOUR_ACCESS_TOKEN",
     "access_secret": "YOUR_ACCESS_SECRET"
   }
   ```
3. (Recommended) Restrict permissions:
   ```bash
   chmod 600 ~/.x-cli/config.json
   ```

You can alternatively place `config.json` in the project root when running from the source tree.

### Option 2: Environment variables

Export the credentials before running the CLI:

```bash
export TWITTER_API_KEY="YOUR_API_KEY"
export TWITTER_API_SECRET="YOUR_API_SECRET"
export TWITTER_ACCESS_TOKEN="YOUR_ACCESS_TOKEN"
export TWITTER_ACCESS_SECRET="YOUR_ACCESS_SECRET"
```

These variables override values in `config.json`.

## Quick Start with Scheduling

1. **Schedule a tweet**:
   ```bash
   go run . --text "My first scheduled tweet! 🚀" --schedule "18:00"
   ```

2. **Check your scheduled tweets**:
   ```bash
   go run . scheduler list
   ```

3. **Start the scheduler daemon** (in a separate terminal or background):
   ```bash
   go run . scheduler daemon
   ```

4. **Your tweet will be posted automatically** at the scheduled time!

## Usage

### Immediate Posting

Post a text-only update:

```bash
go run . --text "Hello, X!"
```

Post with an image:

```bash
go run . --text "New blog post!" --image /path/to/image.png
```

### Scheduled Tweets

Schedule a tweet for a specific date and time:

```bash
go run . --text "Happy New Year! 🎉" --schedule "2024-12-31 23:59"
```

Schedule a tweet for later today:

```bash
go run . --text "Good morning!" --schedule "09:00"
```

Schedule a tweet with an image:

```bash
go run . --text "Check out this photo!" --image photo.jpg --schedule "15:30"
```

### Managing Scheduled Tweets

List all scheduled tweets:

```bash
go run . scheduler list
```

Run the scheduler daemon (keeps running and posts tweets at scheduled times):

```bash
go run . scheduler daemon
```

Cancel a scheduled tweet:

```bash
go run . scheduler cancel tweet_1234567890
```

If you installed the binary, replace `go run .` with `x-cli`.

### Command Reference

#### Main Commands
- `--text`, `-t` *(required)*: Tweet text.
- `--image`, `-i`: Path to a media file (currently sent as-is with a base64 upload).
- `--schedule`, `-s`: Schedule tweet for later posting. Supports multiple formats:
  - `YYYY-MM-DD HH:MM` - Full date and time
  - `MM-DD HH:MM` - Month, day, and time (current year)
  - `HH:MM` - Time only (today's date)

#### Scheduler Commands
- `scheduler list` - Show all scheduled tweets
- `scheduler daemon` - Run background process to post scheduled tweets
- `scheduler cancel [tweet-id]` - Cancel a specific scheduled tweet

### Scheduling Features

- **Flexible time formats**: Use full dates, month-day, or time-only formats
- **Persistent storage**: Scheduled tweets are saved locally in `scheduled_tweets.json`
- **Background daemon**: Run the scheduler to automatically post tweets at the right time
- **Image support**: Schedule tweets with media attachments
- **Management tools**: List, view, and cancel scheduled tweets
- **Validation**: Prevents scheduling tweets in the past

Errors from the API are surfaced verbatim to help diagnose credential or access issues.

## Troubleshooting

- **Missing credentials**: The CLI reports which environment variables are required.
- **HTTP 401/403 responses**: Ensure your app still has access to `tweet.write` for v2 and that the tokens match the OAuth 1.0a user flow.
- **Timeouts**: Network connectivity to `api.twitter.com` and `upload.twitter.com` is required. Check firewalls or proxies.
- **Schedule time validation**: Ensure scheduled times are in the future. Use formats like `YYYY-MM-DD HH:MM`, `MM-DD HH:MM`, or `HH:MM`.
- **Scheduler daemon**: The daemon must be running to post scheduled tweets. Use `x-cli scheduler daemon` to start it.
- **Scheduled tweets storage**: Scheduled tweets are stored in `scheduled_tweets.json` in the current directory.

## Development Notes

- The project purposely avoids external Twitter client libraries. All requests are signed manually using Go's standard library.
- Contributions should adhere to Go formatting (`gofmt`) and target Go 1.22 compatibility.

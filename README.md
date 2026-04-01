# GitFive-Go

A Go port of [GitFive](https://github.com/mxrch/GitFive) — an OSINT tool to investigate GitHub profiles.

Single binary, cross-platform, zero runtime dependencies.

## Features

- Usernames / names history and variations
- Email address to GitHub account lookup
- Batch email processing
- Lists identities used by the target
- Clones and analyzes every target's repos
- Highlights emails tied to the target's GitHub account
- Finds local identities (UPNs)
- Finds potential secondary GitHub accounts
- Generates email address combinations and checks for matches
- Dumps SSH public keys
- JSON export

## Installation

Download a prebuilt binary from [Releases](https://github.com/zackey-heuristics/GitFive-Go/releases), or build from source:

```bash
git clone https://github.com/zackey-heuristics/GitFive-Go.git
cd GitFive-Go
make build
```

The binary will be at `bin/gitfive-go`.

### Requirements

- Git (must be on PATH — used for repo cloning and commit operations)

## Usage

First, login to GitHub *(preferably with a secondary account)*:

```bash
gitfive-go login
```

Then:

```
Usage:
  gitfive-go [command]

Available Commands:
  login       Authenticate to GitHub
  user        Track down a GitHub user by username
  email       Track down a GitHub user by email address
  emails      Find GitHub usernames for a list of email addresses
  light       Quickly find email addresses from a GitHub username

Flags:
  -h, --help      help for gitfive-go
  -v, --version   version for gitfive-go
```

### Examples

```bash
# Full reconnaissance on a user
gitfive-go user <username>

# Export results as JSON
gitfive-go user <username> --json output.json

# Quick email discovery
gitfive-go light <username>

# Reverse email lookup
gitfive-go email <email_address>

# Batch email lookup
gitfive-go emails emails.txt -t <target_username>
```

## Building

```bash
make build          # Build binary
make test           # Run tests
make clean          # Remove build artifacts
```

### Cross-compilation

Static binaries for all platforms are built with `CGO_ENABLED=0`:

```bash
GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build -o gitfive-go-linux-amd64       ./cmd/gitfive
GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 go build -o gitfive-go-darwin-arm64       ./cmd/gitfive
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o gitfive-go-windows-amd64.exe  ./cmd/gitfive
```

## Credits

- Original [GitFive](https://github.com/mxrch/GitFive) by [mxrch](https://github.com/mxrch)

## License

[MPL-2.0](LICENSE.md)

# GitFive-Go

> **Unofficial implementation.** This is a third-party Go port of
> [mxrch/GitFive](https://github.com/mxrch/GitFive), maintained
> independently. It is not affiliated with, endorsed by, or supported by
> the original author. Bug reports for this Go port go to
> [this repository's issue tracker](https://github.com/zackey-heuristics/GitFive-Go/issues),
> not upstream.

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

### GitHub authentication token

GitFive-Go authenticates with a single **fine-grained personal access token** (token starts with `github_pat_`). No GitHub username, password, or 2FA challenge is required — the token is the only secret you provide. Classic PATs (`ghp_*`) and OAuth/server tokens are rejected.

Create one at <https://github.com/settings/tokens?type=beta> with the following settings:

- **Resource owner**: yourself
- **Repository access**: **All repositories** (required — the tool creates a private temporary repository at runtime, which "Selected repositories" cannot cover)
- **Repository permissions**:
  - **Contents**: Read and write
  - **Administration**: Read and write
  - **Metadata**: Read

All other permissions can be left at "No access". Fine-grained PATs require an expiration date (max 1 year); GitFive-Go warns when the token is within 30 days of expiry, and you must regenerate it before then.

Documentation: <https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens#creating-a-fine-grained-personal-access-token>

### On-disk state

Credentials, sessions, and per-target temp data are stored under `~/.gitfive_go/`. Delete that directory to fully reset.

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

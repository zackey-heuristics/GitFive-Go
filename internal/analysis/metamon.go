package analysis

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/zackey-heuristics/gitfive-go/internal/gitcred"
	"github.com/zackey-heuristics/gitfive-go/internal/scraper"
	"github.com/zackey-heuristics/gitfive-go/internal/ui"
	"github.com/zackey-heuristics/gitfive-go/internal/util"
)

// StartMetamon creates a temporary repo with spoofed email commits and pushes it.
// Returns the temp repo name and a map of commit_hash -> email.
func StartMetamon(ctx context.Context, owner, token string, emails []string) (string, map[string]string, error) {
	if len(emails) == 0 {
		return "", nil, nil
	}

	tempRepoName := "gitfive-tmp-" + uuid.New().String()[:8]

	// Create remote repo
	if err := scraper.CreateRepo(ctx, token, tempRepoName); err != nil {
		return "", nil, fmt.Errorf("metamon: create repo: %w", err)
	}

	// Create local temp dir
	dir, err := util.GitfiveDir()
	if err != nil {
		return tempRepoName, nil, err
	}
	tmpDir := filepath.Join(dir, ".tmp", owner, "fake", tempRepoName)
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return tempRepoName, nil, err
	}

	// Init git repo. The remote URL is token-less; authentication for the
	// later push flows through GIT_ASKPASS (see internal/gitcred). Storing
	// a token-less URL also keeps the temp repo's `.git/config` clean if
	// the run is interrupted before cleanup.
	repoURL := fmt.Sprintf("https://github.com/%s/%s", owner, tempRepoName)

	if err := gitExec(ctx, tmpDir, "init"); err != nil {
		return tempRepoName, nil, err
	}
	if err := gitExec(ctx, tmpDir, "remote", "add", "origin", repoURL); err != nil {
		return tempRepoName, nil, err
	}
	if err := gitExec(ctx, tmpDir, "config", "user.name", "gitfive_hunter"); err != nil {
		return tempRepoName, nil, err
	}
	if err := gitExec(ctx, tmpDir, "config", "user.email", "hunter@gitfive.local"); err != nil {
		return tempRepoName, nil, err
	}

	// Create initial commit
	dummyFile := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(dummyFile, []byte("temp"), 0o644); err != nil {
		return tempRepoName, nil, err
	}
	if err := gitExec(ctx, tmpDir, "add", "."); err != nil {
		return tempRepoName, nil, err
	}
	if err := gitExec(ctx, tmpDir, "commit", "-m", "init"); err != nil {
		return tempRepoName, nil, err
	}

	// Create spoofed commits — one per email
	bar := ui.NewProgressBar(len(emails), "Creating spoofed commits...")
	emailsIndex := make(map[string]string)

	for _, email := range emails {
		// Write a unique file for each commit
		if err := os.WriteFile(dummyFile, []byte(email), 0o644); err != nil {
			continue
		}
		if err := gitExec(ctx, tmpDir, "add", "."); err != nil {
			continue
		}

		// Commit with spoofed author
		handle := strings.SplitN(email, "@", 2)[0]
		if err := gitExecEnv(ctx, tmpDir,
			[]string{
				"GIT_AUTHOR_NAME=" + handle,
				"GIT_AUTHOR_EMAIL=" + email,
			},
			"commit", "-m", fmt.Sprintf("spoof %s", email),
		); err != nil {
			continue
		}

		// Get the commit hash
		hash, err := gitOutput(ctx, tmpDir, "rev-parse", "HEAD")
		if err != nil {
			continue
		}
		emailsIndex[strings.TrimSpace(hash)] = email
		_ = bar.Add(1)
	}
	_ = bar.Finish()

	// Rename branch to mirage and push. The push is the one operation that
	// requires GitHub authentication; route it through GIT_ASKPASS so the
	// token never lands in argv.
	if err := gitExec(ctx, tmpDir, "branch", "-M", "mirage"); err != nil {
		return tempRepoName, nil, err
	}
	pushCmd, err := gitcred.CommandWithToken(ctx, "git", "push", "-u", "origin", "mirage")
	if err != nil {
		return tempRepoName, nil, fmt.Errorf("metamon: configure push auth: %w", err)
	}
	pushCmd.Dir = tmpDir
	if err := pushCmd.Run(); err != nil {
		return tempRepoName, nil, fmt.Errorf("metamon: push failed: %w", err)
	}

	return tempRepoName, emailsIndex, nil
}

func gitExec(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

func gitExecEnv(ctx context.Context, dir string, env []string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

func gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return string(out), err
}

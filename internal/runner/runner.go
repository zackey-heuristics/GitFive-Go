package runner

import (
	"context"
	"fmt"
	"sync"

	"golang.org/x/sync/semaphore"

	"github.com/zackey-heuristics/gitfive-go/internal/api"
	"github.com/zackey-heuristics/gitfive-go/internal/auth"
	"github.com/zackey-heuristics/gitfive-go/internal/httpclient"
	"github.com/zackey-heuristics/gitfive-go/internal/models"
	"github.com/zackey-heuristics/gitfive-go/internal/ui"
)

// Runner centralizes common state needed during a GitFive run.
type Runner struct {
	TMPrinter *ui.TMPrinter
	Creds     *auth.Credentials
	Target    *models.Target
	Client    *httpclient.Client
	API       *api.Interface

	// Concurrency limiters (replaces trio CapacityLimiters)
	Limiters map[string]*semaphore.Weighted

	// Mutable shared state (protected by mu)
	mu                sync.Mutex
	XrayNearIteration int
	EmailsAccounts    map[string]map[string]interface{}
	ShownEmails       models.StringSet
	ShownNearNames    models.StringSet
	SpoofedEmails     models.StringSet
	AnalyzedUsernames models.StringSet
}

// New creates a new Runner with initialized state.
func New() (*Runner, error) {
	creds, err := auth.NewCredentials()
	if err != nil {
		return nil, err
	}

	client := httpclient.New()

	return &Runner{
		TMPrinter: ui.NewTMPrinter(),
		Creds:     creds,
		Target:    models.NewTarget(),
		Client:    client,
		Limiters: map[string]*semaphore.Weighted{
			"pea_repos":            semaphore.NewWeighted(10),
			"pea_repos_search":     semaphore.NewWeighted(10),
			"pea_followers":        semaphore.NewWeighted(10),
			"social_follows":       semaphore.NewWeighted(50),
			"repos_list":           semaphore.NewWeighted(50),
			"commits_scrape":       semaphore.NewWeighted(50),
			"commits_fetch_avatar": semaphore.NewWeighted(1),
			"orgs_list":            semaphore.NewWeighted(50),
		},
		EmailsAccounts:    make(map[string]map[string]interface{}),
		ShownEmails:       models.NewStringSet(),
		ShownNearNames:    models.NewStringSet(),
		SpoofedEmails:     models.NewStringSet(),
		AnalyzedUsernames: models.NewStringSet(),
	}, nil
}

// Login loads credentials, validates the token, and initializes the API.
func (r *Runner) Login(ctx context.Context) error {
	_ = ctx // kept for signature stability; CheckToken does not need it
	r.Creds.Load()

	if !r.Creds.AreLoaded() {
		return fmt.Errorf("no credentials found — run `gitfive-go login` first")
	}
	if err := auth.CheckToken(r.Creds); err != nil {
		return fmt.Errorf("token check failed: %w", err)
	}
	// CheckToken should always populate Username from the /user response;
	// guard against any future code path that succeeds without setting it.
	if r.Creds.Username == "" {
		return fmt.Errorf("token valid but username not resolved; re-run `gitfive-go login`")
	}

	r.API = api.NewInterface(r.Creds, r.TMPrinter)
	return nil
}

// Lock acquires the runner mutex for shared state access.
func (r *Runner) Lock() {
	r.mu.Lock()
}

// Unlock releases the runner mutex.
func (r *Runner) Unlock() {
	r.mu.Unlock()
}

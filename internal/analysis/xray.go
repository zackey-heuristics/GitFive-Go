package analysis

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/zackey-heuristics/gitfive-go/internal/models"
	"github.com/zackey-heuristics/gitfive-go/internal/ui"
	"github.com/zackey-heuristics/gitfive-go/internal/util"
)

// RepoAnalysisResult holds the result of analyzing a single repository.
type RepoAnalysisResult struct {
	Repo             string
	AllContribs      map[string]*models.ContribEntry
	InternalContribs map[string]*models.ContribEntry
	UsernamesHistory map[string]*models.ContribEntry
}

// AnalyzeRepo clones and analyzes a single git repository.
func AnalyzeRepo(ctx context.Context, token, targetUsername string, targetID int, reposFolder string, repoName string) (*RepoAnalysisResult, error) {
	result := &RepoAnalysisResult{
		Repo:             repoName,
		AllContribs:      make(map[string]*models.ContribEntry),
		InternalContribs: make(map[string]*models.ContribEntry),
		UsernamesHistory: make(map[string]*models.ContribEntry),
	}

	repoID := fmt.Sprintf("%s/%s", targetUsername, repoName)
	repoURL := fmt.Sprintf("https://%s:x-oauth-basic@github.com/%s", token, repoID)
	repoPath := filepath.Join(reposFolder, repoName)

	// Clone with partial filter
	cmd := exec.CommandContext(ctx, "git", "clone", "--filter=tree:0", "--no-checkout", repoURL, repoPath)
	cmd.Run() // Ignore error — may fail on checkout but commits are cloned

	// Check if repo has any refs
	refCmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	refCmd.Dir = repoPath
	if err := refCmd.Run(); err != nil {
		return result, nil // Empty repo
	}

	// Get all commits using git log
	logCmd := exec.CommandContext(ctx, "git", "log", "--all", "--format=%H%n%ae%n%an%n%ce%n%cn")
	logCmd.Dir = repoPath
	output, err := logCmd.Output()
	if err != nil {
		return result, nil
	}

	lines := strings.Split(string(output), "\n")
	for i := 0; i+4 < len(lines); i += 5 {
		// hexsha := lines[i]
		authorEmail := lines[i+1]
		authorName := lines[i+2]
		committerEmail := lines[i+3]
		committerName := lines[i+4]

		for _, entity := range []struct{ email, name string }{
			{authorEmail, authorName},
			{committerEmail, committerName},
		} {
			addContrib(result.AllContribs, entity.email, entity.name, repoID)

			// Track username history from noreply emails
			if strings.HasSuffix(entity.email, "@users.noreply.github.com") {
				if strings.Count(entity.email, "+") == 1 && strings.HasPrefix(entity.email, fmt.Sprintf("%d+", targetID)) {
					username := strings.SplitN(strings.SplitN(entity.email, "+", 2)[1], "@", 2)[0]
					username = util.UnicodePatch(username)
					name := util.UnicodePatch(entity.name)
					addContrib(result.UsernamesHistory, username, name, repoID)
				}
			}

			// Internal contributors (not noreply, not merged)
			if entity.email != "noreply@github.com" && !strings.HasSuffix(entity.email, "@users.noreply.github.com") {
				addContrib(result.InternalContribs, entity.email, entity.name, repoID)
			}
		}
	}

	return result, nil
}

func addContrib(contribs map[string]*models.ContribEntry, email, name, repoID string) {
	if _, ok := contribs[email]; !ok {
		parts := strings.SplitN(email, "@", 2)
		handle := parts[0]
		domain := ""
		if len(parts) > 1 {
			domain = parts[1]
		}
		contribs[email] = &models.ContribEntry{
			Names:  models.NewStringSet(),
			Handle: handle,
			Domain: domain,
		}
	}
	contribs[email].Names.Add(name)
}

// XrayAnalyze runs deep analysis on all source repos.
func XrayAnalyze(ctx context.Context, token, targetUsername string, targetID int,
	repos []models.RepoDetails) ([]*RepoAnalysisResult, error) {

	home, _ := os.UserHomeDir()
	reposFolder := filepath.Join(home, ".malfrats", "gitfive", ".tmp", targetUsername, "repos")
	os.MkdirAll(reposFolder, 0o755)

	var sourceRepos []models.RepoDetails
	for _, r := range repos {
		if r.IsSource {
			sourceRepos = append(sourceRepos, r)
		}
	}

	if len(sourceRepos) == 0 {
		return nil, nil
	}

	bar := ui.NewProgressBar(len(sourceRepos), "[XRAY] Dumping and analyzing repos...")
	var mu sync.Mutex
	var results []*RepoAnalysisResult

	g, ctx := errgroup.WithContext(ctx)
	for _, repo := range sourceRepos {
		repo := repo
		g.Go(func() error {
			result, err := AnalyzeRepo(ctx, token, targetUsername, targetID, reposFolder, repo.Name)
			if err != nil {
				return err
			}
			mu.Lock()
			results = append(results, result)
			bar.Add(1)
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	bar.Finish()
	return results, nil
}

// NearLookup searches for name variations using Levenshtein distance.
func NearLookup(target *models.Target, iteration *int) bool {
	*iteration++
	newVariations := false

	possibleNames := target.PossibleNames()
	fmt.Printf("\n[XRAY] Near names iteration #%d\n", *iteration)
	fmt.Printf("[XRAY] Using names %v\n\n", possibleNames.Slice())

	for email, entry := range target.AllContribs {
		if email == "noreply@github.com" || strings.HasSuffix(email, "@users.noreply.github.com") {
			continue
		}

		handle := entry.Handle
		for name := range possibleNames {
			if util.IsDiffLow(name, handle, 40) {
				if _, exists := target.NearNames[handle]; !exists {
					newVariations = true
					target.NearNames[handle] = map[string]interface{}{
						"related_data": make(map[string]interface{}),
					}
				}
				if rd, ok := target.NearNames[handle].(map[string]interface{}); ok {
					if relData, ok := rd["related_data"].(map[string]interface{}); ok {
						relData[email] = entry
					}
				}
			}
		}

		for entryName := range entry.Names {
			for name := range possibleNames {
				if util.IsDiffLow(name, entryName, 40) {
					if _, exists := target.NearNames[entryName]; !exists {
						newVariations = true
						target.NearNames[entryName] = map[string]interface{}{
							"related_data": make(map[string]interface{}),
						}
					}
					if rd, ok := target.NearNames[entryName].(map[string]interface{}); ok {
						if relData, ok := rd["related_data"].(map[string]interface{}); ok {
							relData[email] = entry
						}
					}
				}
			}
		}
	}

	return newVariations
}

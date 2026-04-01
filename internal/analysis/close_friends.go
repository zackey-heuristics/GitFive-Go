package analysis

import (
	"context"
	"fmt"
	"sort"

	"github.com/zackey-heuristics/gitfive-go/internal/httpclient"
	"github.com/zackey-heuristics/gitfive-go/internal/models"
	"github.com/zackey-heuristics/gitfive-go/internal/scraper"
	"github.com/zackey-heuristics/gitfive-go/internal/ui"
)

// GuessCloseFriends analyzes the social graph to find likely close associates.
func GuessCloseFriends(ctx context.Context, client *httpclient.Client, target *models.Target, sem scraper.Semaphore) (map[string]*models.FriendScore, error) {
	users := make(map[string]*models.FriendScore)
	tmprinter := ui.NewTMPrinter()

	tmprinter.Out("Analyzing if target is PEA...")
	targetSet := models.NewStringSet()
	targetSet.Add(target.Username)
	peaCache, err := scraper.AnalyzePEA(ctx, client, targetSet, sem)
	if err != nil {
		return nil, err
	}
	tmprinter.Clear()

	isPEA := peaCache[target.Username]
	fmt.Printf("Account is PEA: %v\n", isPEA)

	following, err := scraper.GetFollows(ctx, client, target.Username, "following", sem)
	if err != nil {
		return nil, err
	}

	if !isPEA && len(following) == 0 {
		return users, nil
	}

	followers, err := scraper.GetFollows(ctx, client, target.Username, "followers", sem)
	if err != nil {
		return nil, err
	}

	var usernames models.StringSet
	if isPEA {
		usernames = following.Union(followers)
	} else {
		usernames = following
	}

	newPEACache, err := scraper.AnalyzePEA(ctx, client, usernames, sem)
	if err != nil {
		return nil, err
	}
	for k, v := range newPEACache {
		peaCache[k] = v
	}

	if isPEA {
		for u := range followers {
			updateFriend(users, u, "Follower is following PEA")
			if peaCache[u] {
				updateFriend(users, u, "Follower is PEA")
			}
		}
	}

	for u := range following {
		if peaCache[u] {
			updateFriend(users, u, "Following is PEA")
		}
		if followers.Contains(u) {
			updateFriend(users, u, "Follower + Following")
		}
	}

	return users, nil
}

func updateFriend(users map[string]*models.FriendScore, username, reason string) {
	if _, ok := users[username]; !ok {
		users[username] = &models.FriendScore{Points: 0}
	}
	users[username].Points++
	users[username].Reasons = append(users[username].Reasons, reason)
}

// ShowCloseFriends displays close friends results.
func ShowCloseFriends(friends map[string]*models.FriendScore) {
	if len(friends) == 0 {
		fmt.Println("[-] No potential close friends were found.")
		fmt.Println("\n* PEA = Pretty Empty Account")
		return
	}

	fmt.Printf("[+] %d potential close friend(s) found!\n", len(friends))

	// Collect unique point values
	pointSet := make(map[int]bool)
	for _, f := range friends {
		pointSet[f.Points] = true
	}
	var points []int
	for p := range pointSet {
		points = append(points, p)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(points)))

	for _, p := range points {
		var toShow []string
		for u, f := range friends {
			if f.Points == p {
				toShow = append(toShow, u)
			}
		}
		fmt.Printf("\nClose friend(s) with %d point(s):\n", p)
		limit := 14
		if len(toShow) < limit {
			limit = len(toShow)
		}
		for _, u := range toShow[:limit] {
			fmt.Printf("- %s (%s)\n", u, joinReasons(friends[u].Reasons))
		}
		if len(toShow) > 14 {
			fmt.Println("- [...]")
		}
	}

	fmt.Println("\n* PEA = Pretty Empty Account")
}

func joinReasons(reasons []string) string {
	if len(reasons) == 0 {
		return ""
	}
	result := reasons[0]
	for _, r := range reasons[1:] {
		result += ", " + r
	}
	return result
}

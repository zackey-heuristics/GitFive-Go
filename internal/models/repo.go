package models

// RepoDetails holds metadata about a GitHub repository.
type RepoDetails struct {
	Name       string `json:"name"`
	Language   string `json:"language,omitempty"`
	Stars      int    `json:"stars"`
	Forks      int    `json:"forks"`
	IsFork     bool   `json:"is_fork"`
	IsMirror   bool   `json:"is_mirror"`
	IsArchived bool   `json:"is_archived"`
	IsSource   bool   `json:"is_source"`
}

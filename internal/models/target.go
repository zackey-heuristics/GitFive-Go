package models

import (
	"encoding/json"
	"strings"
	"time"
)

// Target holds all reconnaissance data for a GitHub user.
type Target struct {
	Username        string  `json:"username"`
	Name            string  `json:"name"`
	ID              int     `json:"id"`
	Type            string  `json:"type"`
	IsSiteAdmin     bool    `json:"is_site_admin"`
	Hireable        bool    `json:"hireable"`
	Company         string  `json:"company,omitempty"`
	Blog            string  `json:"blog,omitempty"`
	Location        string  `json:"location,omitempty"`
	Bio             string  `json:"bio,omitempty"`
	Twitter         string  `json:"twitter,omitempty"`
	NbPublicRepos   int     `json:"nb_public_repos"`
	NbFollowers     int     `json:"nb_followers"`
	NbFollowing     int     `json:"nb_following"`
	CreatedAt       *time.Time `json:"created_at,omitempty"`
	UpdatedAt       *time.Time `json:"updated_at,omitempty"`
	AvatarURL       string  `json:"avatar_url,omitempty"`
	IsDefaultAvatar bool    `json:"is_default_avatar"`
	NbExtContribs   int     `json:"nb_ext_contribs"`

	PotentialFriends map[string]*FriendScore `json:"potential_friends,omitempty"`
	Repos            []RepoDetails           `json:"repos,omitempty"`
	LanguagesStats   map[string]float64      `json:"languages_stats,omitempty"`
	Orgs             []map[string]interface{} `json:"orgs,omitempty"`

	Usernames StringSet `json:"usernames"`
	Fullnames StringSet `json:"fullnames"`
	Domains   StringSet `json:"domains"`

	SSHKeys []string `json:"ssh_keys,omitempty"`

	AllContribs      map[string]*ContribEntry `json:"all_contribs,omitempty"`
	ExtContribs      map[string]*ContribEntry `json:"ext_contribs,omitempty"`
	InternalContribs *InternalContribs        `json:"internal_contribs,omitempty"`
	UsernamesHistory map[string]*ContribEntry `json:"usernames_history,omitempty"`
	NearNames        map[string]interface{}   `json:"near_names,omitempty"`

	Emails           StringSet                    `json:"emails"`
	GeneratedEmails  StringSet                    `json:"generated_emails"`
	RegisteredEmails map[string]*RegisteredEmail  `json:"registered_emails,omitempty"`
}

// FriendScore tracks potential friend scoring data.
type FriendScore struct {
	Points  int      `json:"points"`
	Reasons []string `json:"reasons,omitempty"`
}

// NewTarget creates a Target with initialized collections.
func NewTarget() *Target {
	return &Target{
		IsDefaultAvatar:  true,
		PotentialFriends: make(map[string]*FriendScore),
		LanguagesStats:   make(map[string]float64),
		Usernames:        NewStringSet(),
		Fullnames:        NewStringSet(),
		Domains:          NewStringSet(),
		AllContribs:      make(map[string]*ContribEntry),
		ExtContribs:      make(map[string]*ContribEntry),
		InternalContribs: NewInternalContribs(),
		UsernamesHistory: make(map[string]*ContribEntry),
		NearNames:        make(map[string]interface{}),
		Emails:           NewStringSet(),
		GeneratedEmails:  NewStringSet(),
		RegisteredEmails: make(map[string]*RegisteredEmail),
	}
}

// PossibleNames returns the lowercase union of usernames and fullnames.
func (t *Target) PossibleNames() StringSet {
	result := NewStringSet()
	for name := range t.Usernames.Union(t.Fullnames) {
		result.Add(strings.ToLower(name))
	}
	return result
}

// AddName adds a name to either Usernames (no space) or Fullnames (has space).
func (t *Target) AddName(name string) {
	if name == "" {
		return
	}
	if strings.Contains(name, " ") {
		t.Fullnames.Add(name)
	} else {
		t.Usernames.Add(name)
	}
}

// ExportJSON serializes the target to pretty-printed JSON.
func (t *Target) ExportJSON() (string, error) {
	data, err := json.MarshalIndent(t, "", "    ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

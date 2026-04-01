package models

import "encoding/json"

// ContribEntry holds contribution data for an email address.
type ContribEntry struct {
	Names  StringSet `json:"names"`
	Handle string    `json:"handle,omitempty"`
	Domain string    `json:"domain,omitempty"`
}

// NewContribEntry creates an empty ContribEntry.
func NewContribEntry() *ContribEntry {
	return &ContribEntry{Names: NewStringSet()}
}

// InternalContribs separates contributions into "all" and "no_github" categories.
type InternalContribs struct {
	All      map[string]*ContribEntry `json:"all"`
	NoGitHub map[string]*ContribEntry `json:"no_github"`
}

// NewInternalContribs creates an empty InternalContribs.
func NewInternalContribs() *InternalContribs {
	return &InternalContribs{
		All:      make(map[string]*ContribEntry),
		NoGitHub: make(map[string]*ContribEntry),
	}
}

// RegisteredEmail holds info about a registered email on GitHub.
type RegisteredEmail struct {
	Avatar   string `json:"avatar,omitempty"`
	FullName string `json:"full_name,omitempty"`
	Username string `json:"username,omitempty"`
	IsTarget bool   `json:"is_target"`
}

// StringSet is a set of strings backed by a map.
type StringSet map[string]struct{}

// NewStringSet creates an empty StringSet.
func NewStringSet() StringSet {
	return make(StringSet)
}

// Add inserts a value into the set.
func (s StringSet) Add(val string) {
	s[val] = struct{}{}
}

// Contains checks if a value is in the set.
func (s StringSet) Contains(val string) bool {
	_, ok := s[val]
	return ok
}

// Slice returns the set contents as a slice.
func (s StringSet) Slice() []string {
	result := make([]string, 0, len(s))
	for k := range s {
		result = append(result, k)
	}
	return result
}

// Union returns a new set with elements from both sets.
func (s StringSet) Union(other StringSet) StringSet {
	result := NewStringSet()
	for k := range s {
		result.Add(k)
	}
	for k := range other {
		result.Add(k)
	}
	return result
}

// MarshalJSON converts the set to a JSON array.
func (s StringSet) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.Slice())
}

package models

import (
	"encoding/json"
	"testing"
)

func TestNewTarget(t *testing.T) {
	target := NewTarget()
	if target == nil {
		t.Fatal("NewTarget returned nil")
	}
	if !target.IsDefaultAvatar {
		t.Error("expected IsDefaultAvatar to be true")
	}
	if target.Usernames == nil || target.Fullnames == nil || target.Domains == nil {
		t.Error("expected sets to be initialized")
	}
}

func TestAddName(t *testing.T) {
	target := NewTarget()

	target.AddName("mxrch")
	if !target.Usernames.Contains("mxrch") {
		t.Error("expected 'mxrch' in Usernames")
	}

	target.AddName("John Doe")
	if !target.Fullnames.Contains("John Doe") {
		t.Error("expected 'John Doe' in Fullnames")
	}

	target.AddName("")
	if len(target.Usernames) != 1 || len(target.Fullnames) != 1 {
		t.Error("empty name should not be added")
	}
}

func TestPossibleNames(t *testing.T) {
	target := NewTarget()
	target.AddName("Alice")
	target.AddName("Bob Smith")

	names := target.PossibleNames()
	if !names.Contains("alice") {
		t.Error("expected 'alice' in PossibleNames")
	}
	if !names.Contains("bob smith") {
		t.Error("expected 'bob smith' in PossibleNames")
	}
}

func TestExportJSON(t *testing.T) {
	target := NewTarget()
	target.Username = "testuser"
	target.Name = "Test User"
	target.ID = 12345

	jsonStr, err := target.ExportJSON()
	if err != nil {
		t.Fatal(err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed["username"] != "testuser" {
		t.Errorf("expected username 'testuser', got %v", parsed["username"])
	}
}

func TestStringSet(t *testing.T) {
	s := NewStringSet()
	s.Add("a")
	s.Add("b")
	s.Add("a") // duplicate

	if len(s) != 2 {
		t.Errorf("expected 2 elements, got %d", len(s))
	}
	if !s.Contains("a") || !s.Contains("b") {
		t.Error("expected set to contain 'a' and 'b'")
	}
	if s.Contains("c") {
		t.Error("set should not contain 'c'")
	}

	s2 := NewStringSet()
	s2.Add("b")
	s2.Add("c")

	union := s.Union(s2)
	if len(union) != 3 {
		t.Errorf("expected union size 3, got %d", len(union))
	}
}

func TestStringSetMarshalJSON(t *testing.T) {
	s := NewStringSet()
	s.Add("hello")

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var arr []string
	_ = json.Unmarshal(data, &arr)
	if len(arr) != 1 || arr[0] != "hello" {
		t.Errorf("expected [\"hello\"], got %s", string(data))
	}
}

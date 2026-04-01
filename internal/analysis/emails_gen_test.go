package analysis

import (
	"testing"

	"github.com/zackey-heuristics/gitfive-go/internal/models"
)

func TestGenerateEmails_Basic(t *testing.T) {
	target := models.NewTarget()
	target.AddName("johndoe")
	target.AddName("John Doe")

	spoofed := models.NewStringSet()

	emails := GenerateEmails(target, spoofed,
		nil,
		[]string{"gmail.com"},
		[]string{"contact"},
	)

	if len(emails) == 0 {
		t.Fatal("expected generated emails, got none")
	}

	// Check that username@domain is generated
	found := false
	for _, e := range emails {
		if e == "johndoe@gmail.com" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'johndoe@gmail.com' in generated emails")
	}

	// Check that name combinations are generated (john.doe, j.doe, etc.)
	foundNameCombo := false
	for _, e := range emails {
		if e == "john.doe@gmail.com" || e == "j.doe@gmail.com" || e == "doe.john@gmail.com" {
			foundNameCombo = true
			break
		}
	}
	if !foundNameCombo {
		t.Error("expected name-based email combination in generated emails")
	}
}

func TestGenerateEmails_Dedup(t *testing.T) {
	target := models.NewTarget()
	target.AddName("alice")

	spoofed := models.NewStringSet()

	emails1 := GenerateEmails(target, spoofed, nil, []string{"gmail.com"}, nil)
	emails2 := GenerateEmails(target, spoofed, nil, []string{"gmail.com"}, nil)

	if len(emails1) == 0 {
		t.Fatal("first call should generate emails")
	}
	if len(emails2) != 0 {
		t.Errorf("second call should return 0 (all already spoofed), got %d", len(emails2))
	}
}

func TestGenerateEmails_CustomDomains(t *testing.T) {
	target := models.NewTarget()
	target.AddName("bob")
	target.Domains.Add("example.com")

	spoofed := models.NewStringSet()

	emails := GenerateEmails(target, spoofed,
		nil,
		[]string{"gmail.com"},
		[]string{"contact", "admin"},
	)

	foundPrefix := false
	for _, e := range emails {
		if e == "contact@example.com" || e == "admin@example.com" {
			foundPrefix = true
			break
		}
	}
	if !foundPrefix {
		t.Error("expected prefix@custom_domain in generated emails")
	}
}

func TestIsLocalName(t *testing.T) {
	if !isLocalName("root") {
		t.Error("'root' should be local name")
	}
	if !isLocalName("ROOT") {
		t.Error("'ROOT' should be local name (case insensitive)")
	}
	if isLocalName("johndoe") {
		t.Error("'johndoe' should not be local name")
	}
}

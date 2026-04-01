package analysis

import (
	"fmt"
	"strings"

	"github.com/zackey-heuristics/gitfive-go/internal/config"
	"github.com/zackey-heuristics/gitfive-go/internal/models"
	"github.com/zackey-heuristics/gitfive-go/internal/util"
)

// GenerateEmails produces combinatorial email candidates from aggregated target data.
func GenerateEmails(target *models.Target, spoofedEmails models.StringSet,
	customDomains, defaultDomains []string, domainPrefixes []string) []string {

	fullnames := models.NewStringSet()
	usernames := models.NewStringSet()
	foundDomains := models.NewStringSet()

	for name := range target.Fullnames {
		fullnames.Add(strings.ToLower(name))
	}
	for name := range target.Usernames {
		usernames.Add(strings.ToLower(name))
	}
	for d := range target.Domains {
		foundDomains.Add(strings.ToLower(d))
	}

	domains := models.NewStringSet()
	for d := range foundDomains {
		domains.Add(d)
	}
	for _, d := range customDomains {
		domains.Add(d)
	}
	for _, d := range defaultDomains {
		domains.Add(d)
	}

	// Enrich from internal contribs
	for _, entry := range target.InternalContribs.All {
		if !util.IsLocalDomain(entry.Domain) {
			for _, d := range util.DetectCustomDomain(entry.Domain) {
				domains.Add(d)
			}
		}
	}
	for _, entry := range target.InternalContribs.NoGitHub {
		if !util.IsLocalDomain(entry.Domain) || !isLocalName(entry.Handle) {
			usernames.Add(strings.ToLower(entry.Handle))
			usernames.Add(strings.ToLower(strings.SplitN(entry.Handle, "+", 2)[0]))
		}
		for name := range entry.Names {
			if name == "" || (util.IsLocalDomain(entry.Domain) && isLocalName(name)) {
				continue
			}
			addName(name, fullnames, usernames)
		}
	}

	// Enrich from ext contribs
	for _, entry := range target.ExtContribs {
		if !util.IsLocalDomain(entry.Domain) || !isLocalName(entry.Handle) {
			usernames.Add(strings.ToLower(entry.Handle))
			usernames.Add(strings.ToLower(strings.SplitN(entry.Handle, "+", 2)[0]))
		}
		for name := range entry.Names {
			if name == "" || (util.IsLocalDomain(entry.Domain) && isLocalName(name)) {
				continue
			}
			addName(name, fullnames, usernames)
		}
	}

	// Enrich from near names
	for name, data := range target.NearNames {
		if name != "" {
			addName(name, fullnames, usernames)
		}
		if relatedData, ok := data.(map[string]interface{}); ok {
			if rd, ok := relatedData["related_data"].(map[string]interface{}); ok {
				for _, emailDataRaw := range rd {
					if emailData, ok := emailDataRaw.(map[string]interface{}); ok {
						if handle, ok := emailData["handle"].(string); ok {
							usernames.Add(strings.ToLower(handle))
							usernames.Add(strings.ToLower(strings.SplitN(handle, "+", 2)[0]))
						}
						if domain, ok := emailData["domain"].(string); ok {
							if !util.IsLocalDomain(domain) {
								for _, d := range util.DetectCustomDomain(domain) {
									domains.Add(d)
								}
							}
						}
					}
				}
			}
		}
	}

	// Split fullnames into first/last
	type namePair struct{ first, last string }
	var splittedNames []namePair
	for name := range fullnames {
		parts := strings.Fields(util.Sanitize(strings.ToLower(name)))
		if len(parts) > 1 {
			first := parts[0]
			last := strings.Join(parts[1:], "")
			splittedNames = append(splittedNames, namePair{first, last})
		} else if len(parts) == 1 {
			usernames.Add(util.Sanitize(strings.ToLower(name)))
		}
	}

	// Add dotless versions of usernames
	for u := range usernames {
		if strings.Contains(u, ".") {
			usernames.Add(strings.ReplaceAll(u, ".", ""))
		}
	}

	emails := models.NewStringSet()

	for domain := range domains {
		if strings.TrimSpace(domain) == "" {
			continue
		}

		// Name-based combinations
		for _, np := range splittedNames {
			if strings.TrimSpace(np.first) == "" && strings.TrimSpace(np.last) == "" {
				continue
			}
			for _, reverse := range []bool{false, true} {
				firstPos, secondPos := np.first, np.last
				if reverse {
					firstPos, secondPos = np.last, np.first
				}
				for nf := 0; nf <= len(firstPos); nf++ {
					for ns := 0; ns <= len(secondPos); ns++ {
						total := nf + ns
						if total == 0 || (nf < 2 && ns < 2) {
							continue
						}
						for _, dot := range []string{"", "."} {
							emails.Add(fmt.Sprintf("%s%s%s@%s", firstPos[:nf], dot, secondPos[:ns], domain))
							if nf == 0 || ns == 0 {
								break
							}
						}
					}
				}
			}
		}

		// Username-based
		for u := range usernames {
			if strings.TrimSpace(u) == "" {
				continue
			}
			emails.Add(fmt.Sprintf("%s@%s", u, domain))
		}
	}

	// Prefix-based for found (non-default) domains
	defaultSet := make(map[string]bool)
	for _, d := range defaultDomains {
		defaultSet[d] = true
	}
	for domain := range foundDomains {
		if strings.TrimSpace(domain) == "" || defaultSet[domain] {
			continue
		}
		for _, prefix := range domainPrefixes {
			if strings.TrimSpace(prefix) == "" {
				continue
			}
			emails.Add(fmt.Sprintf("%s@%s", prefix, domain))
		}
	}

	// Filter already spoofed
	var result []string
	for email := range emails {
		if !spoofedEmails.Contains(email) {
			result = append(result, email)
			spoofedEmails.Add(email)
		}
	}

	return result
}

func addName(name string, fullnames, usernames models.StringSet) {
	sanitized := util.Sanitize(strings.ToLower(name))
	if strings.Contains(name, " ") {
		fullnames.Add(sanitized)
	} else {
		usernames.Add(sanitized)
	}
}

func isLocalName(name string) bool {
	lower := strings.ToLower(name)
	for _, ln := range config.LocalNames {
		if lower == ln {
			return true
		}
	}
	return false
}

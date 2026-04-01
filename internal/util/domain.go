package util

import (
	"net"
	"strings"

	"github.com/zackey-heuristics/gitfive-go/internal/config"
)

// IsLocalDomain returns true if the domain has no dot or ends with a local TLD.
func IsLocalDomain(domain string) bool {
	if !strings.Contains(domain, ".") {
		return true
	}
	for _, tld := range config.LocalTLDs {
		if strings.HasSuffix(domain, "."+tld) {
			return true
		}
	}
	return false
}

// ExtractDomain extracts the domain from a URL at a given sub-level depth.
// subLevel=0 returns the base domain (e.g. "example.com"),
// subLevel=1 includes one more subdomain level (e.g. "sub.example.com").
func ExtractDomain(url string, subLevel int) string {
	host := url
	if strings.Contains(url, "://") {
		parts := strings.SplitN(url, "/", 4)
		if len(parts) >= 3 {
			host = parts[2]
		}
	} else {
		host = strings.SplitN(url, "/", 2)[0]
	}

	labels := strings.Split(host, ".")
	take := subLevel + 2
	if take > len(labels) {
		take = len(labels)
	}
	return strings.Join(labels[len(labels)-take:], ".")
}

// DetectCustomDomain extracts plausible custom domains from a link.
func DetectCustomDomain(link string) []string {
	link = strings.TrimRight(link, "/")
	var domains []string

	if !strings.Contains(link, ".") {
		return domains
	}
	if strings.Count(link, "/") < 2 && strings.Contains(link, "/") {
		// e.g. "path/file" — not a domain
		return domains
	}

	nbDots := strings.Count(link, ".")
	if nbDots > 3 {
		domains = append(domains, ExtractDomain(link, 0))
		domains = append(domains, ExtractDomain(link, nbDots-1))
	} else {
		for subLevel := 0; subLevel < nbDots; subLevel++ {
			d := ExtractDomain(link, subLevel)
			if !strings.HasPrefix(d, "www.") && !strings.HasSuffix(d, "github.io") {
				domains = append(domains, d)
			}
		}
	}
	return domains
}

// IsGHPagesHosted returns true if the domain resolves to a GitHub Pages IP.
func IsGHPagesHosted(domain string) bool {
	ips, err := net.LookupHost(domain)
	if err != nil {
		return false
	}
	for _, ip := range ips {
		for _, ghIP := range config.GHPagesServers {
			if ip == ghIP {
				return true
			}
		}
	}
	return false
}

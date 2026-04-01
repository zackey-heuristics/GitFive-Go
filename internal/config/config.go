package config

import "net/http"

// DefaultHeaders are used for all HTTP requests to GitHub.
var DefaultHeaders = http.Header{
	"User-Agent":  {"Mozilla/5.0 (Windows NT 10.0; rv:68.0) Gecko/20100101 Firefox/68.0"},
	"Connection":  {"Keep-Alive"},
}

// Timeout is the default HTTP client timeout in seconds.
const Timeout = 60

// GHPagesServers are the GitHub Pages server IPs.
var GHPagesServers = []string{
	"185.199.108.153",
	"185.199.109.153",
	"185.199.110.153",
	"185.199.111.153",
}

// EmailsDefaultDomains are common email provider domains for email generation.
var EmailsDefaultDomains = []string{
	"gmail.com", "outlook.com", "yahoo.com", "inbox.com",
	"icloud.com", "mail.com", "aol.com", "yandex.com", "protonmail.com",
}

// EmailCommonDomainsPrefixes are common prefixes for custom domain emails.
var EmailCommonDomainsPrefixes = []string{
	"me", "hello", "hey", "salut", "contact", "email", "mail",
	"admin", "support", "recrutement", "github", "gitlab", "git",
	"work", "works", "colab", "colaborate", "colaborating",
}

// LocalNames are local system usernames to filter out.
var LocalNames = []string{
	"root", "system", "user", "administrator", "kali", "debian", "ubuntu",
}

// LocalTLDs are local TLDs to filter out from domain detection.
var LocalTLDs = []string{
	"local", "lan", "localdomain", "localhost", "(none)",
}

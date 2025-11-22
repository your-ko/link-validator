package link_validator

import "time"

type Config struct {
	Path           string
	PAT            string
	CorpPAT        string
	CorpGitHubUrl  string
	FileMasks      []string
	ExcludePath    string
	LookupPath     string
	Timeout        time.Duration
	IgnoredDomains []string
}

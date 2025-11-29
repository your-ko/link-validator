package regex

import "regexp"

// this package contains no tests because the regexes are being tested in the corresponding packages in *_ExtractLinks tests

// GitHub captures almost all GitHub urls
var GitHub = regexp.MustCompile(`(?i)https://github\.(?:com|[a-z0-9-]+\.[a-z0-9.-]+)(?:/[^\s\x60\]~]*[^\s.,:;!?()\[\]{}\x60~])?`)

// EnterpriseGitHub captures only enterprise GitHub urls and used to distinguish between public and enterprise.
var EnterpriseGitHub = regexp.MustCompile(`github\.[a-z0-9-]+\.[a-z0-9.-]+`)

// Url captures all HTTPS URLs that looks valid.
var Url = regexp.MustCompile(`https://[a-zA-Z0-9.\[\]{}-]+(?:/[^\s)]*[a-zA-Z0-9/#?&=_\[\]{}-]|/)?`)

// LocalPath captures local Markdown links [text](path)
var LocalPath = regexp.MustCompile(`\[[^]]*]\(((?:\.{1,2}/)*[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)*(?:#[^)\s]*)?)\)`)

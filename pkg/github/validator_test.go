package github

import (
	"reflect"
	"testing"
)

func TestInternalLinkProcessor_ExtractLinks(t *testing.T) {
	t.Parallel()

	p, _ := New("https://github.mycorp.com", "", "", 0) // PAT not needed for regex tests

	type tc struct {
		name string
		line string
		want []string
	}

	tests := []tc{
		{
			name: "keeps github blob; drops externals",
			line: `test (https://github.mycorp.com/your-ko/link-validator/blob/main/README.md)
			       test https://google.com/x
			       test https://github.com/your-ko/link-validator/blob/main/README.md`,
			want: []string{
				"https://github.mycorp.com/your-ko/link-validator/blob/main/README.md",
				"https://github.com/your-ko/link-validator/blob/main/README.md",
			},
		},
		{
			name: "Ignores templated GitHub urls",
			line: `test 
			       test https://github.com/your-ko/[repo]/[path]/workflows/link-validator.yaml
			       test https://github.com/your-ko/{repo}/{path}/workflows/link-validator.yaml
			       test https://github.com/your-ko/{{repo}}/{{path}}/workflows/link-validator.yaml
			       test https://github.com/your-ko/link-validator/blob/main/README.md`,
			want: []string{
				"https://github.com/your-ko/link-validator/blob/main/README.md",
			},
		},
		{
			name: "Captures urls separated by new line",
			line: `test  https://github.com/your-ko/link-validator\n\nhttps://github.com/your-ko/link-validator`,
			want: []string{
				"https://github.com/your-ko/link-validator",
				"https://github.com/your-ko/link-validator",
			},
		},
		{
			name: "Ignores GitHub blog",
			line: `test https://github.blog/changelog/2025-11-18-github-copilot-cli-new-models-enhanced-code-search-and-better-image-support/
			       test https://google.com/x
			       test https://github.com/your-ko/link-validator/blob/main/README.md`,
			want: []string{
				"https://github.com/your-ko/link-validator/blob/main/README.md",
			},
		},
		{
			name: "Catches badge url correctly",
			line: `[![Link validation](https://github.com/your-ko/link-validator/actions/workflows/link-validator.yaml/badge.svg)](https://github.com/your-ko/link-validator/actions/workflows/link-validator.yaml)`,
			want: []string{
				"https://github.com/your-ko/link-validator/actions/workflows/link-validator.yaml/badge.svg",
				"https://github.com/your-ko/link-validator/actions/workflows/link-validator.yaml",
			},
		},
		{
			name: "ignores subdomain uploads.* or api* ",
			line: `test https://uploads.github.mycorp.com/org/repo/raw/main/image.png
			       and external https://gitlab.mycorp.com/a/b
			       and api https://api.github.mycorp.com/org/repo/tree/main/folder`,
			want: nil,
		},
		{
			name: "ignores non-matching schemes and hosts",
			line: `scheme http://github.mycorp.com/org/repo/blob/main/README.md
			       non-github https://other.com/org/repo/blob/main/README.md`,
			want: nil,
		},
		{
			name: "handles anchors and doesn't strip query strings",
			line: `https://github.mycorp.com/your-ko/link-validator/blob/main/file.md#L10-L20
			       https://github.com/your-ko/link-validator/blob/main/file.md#L10-L20
			       https://github.mycorp.com/your-ko/link-validator/tree/main/docs?tab=readme
			       https://github.com/your-ko/link-validator/tree/main/docs?tab=readme
			       https://example.com/u/v/raw/main/w.txt?download=1`,
			want: []string{
				"https://github.mycorp.com/your-ko/link-validator/blob/main/file.md#L10-L20",
				"https://github.com/your-ko/link-validator/blob/main/file.md#L10-L20",
				"https://github.mycorp.com/your-ko/link-validator/tree/main/docs?tab=readme",
				"https://github.com/your-ko/link-validator/tree/main/docs?tab=readme",
			},
		},
		{
			name: "captures non-repo urls (without blob|tree|raw|blame|ref)",
			line: `
				https://github.com/your-ko/link-validator/main/docs
				https://github.mycorp.com/your-ko/link-validator/main/docs
				https://github.com/your-ko/link-validator/main/README.md
				https://github.com/your-ko/link-validator/main/README.md
				https://github.com/your-ko/link-validator/pulls
				https://github.com/your-ko/link-validator/issues/4
				`,
			want: []string{
				"https://github.com/your-ko/link-validator/main/docs",
				"https://github.mycorp.com/your-ko/link-validator/main/docs",
				"https://github.com/your-ko/link-validator/main/README.md",
				"https://github.com/your-ko/link-validator/main/README.md",
				"https://github.com/your-ko/link-validator/pulls",
				"https://github.com/your-ko/link-validator/issues/4",
			},
		},
		{
			name: "ignores non-api calls",
			line: `
				https://raw.githubusercontent.com/your-ko/link-validator/refs/heads/main/README.md
				https://uploads.github.mycorp.com/org/repo/raw/main/img.png
				https://api.github.com/repos/your-ko/link-validator/contents/?ref=a96366f66ffacd461de10a1dd561ab5a598e9167
				`,
			want: nil,
		},
		{
			name: "captures refs urls",
			line: `
				particular commit https://github.com/your-ko/link-validator/commit/a96366f66ffacd461de10a1dd561ab5a598e9167 text
				particular commit https://github.mycorp.com/your-ko/link-validator/commit/a96366f66ffacd461de10a1dd561ab5a598e9167 text`,
			want: []string{
				"https://github.com/your-ko/link-validator/commit/a96366f66ffacd461de10a1dd561ab5a598e9167",
				"https://github.mycorp.com/your-ko/link-validator/commit/a96366f66ffacd461de10a1dd561ab5a598e9167",
			},
		},
		{
			name: "excludes trailing backticks from URLs",
			line: `- Software Bill of Materials: ` + "`" + `https://github.com/your-ko/link-validator/releases/download/1.3.0/sbom.spdx.json` + "`" + `
				- Build provenance: ` + "`" + `https://github.com/your-ko/link-validator/releases/download/1.3.0/provenance.intoto.jsonl` + "`" + `
				- Checksums: ` + "`" + `https://github.com/your-ko/link-validator/releases/download/1.3.0/SHASUMS256.txt` + "`",
			want: []string{
				"https://github.com/your-ko/link-validator/releases/download/1.3.0/sbom.spdx.json",
				"https://github.com/your-ko/link-validator/releases/download/1.3.0/provenance.intoto.jsonl",
				"https://github.com/your-ko/link-validator/releases/download/1.3.0/SHASUMS256.txt",
			},
		},
		{
			name: "ignores urls containing special characters",
			line: `https://[github].[mycorp].[com]`,
			want: nil,
		},
		{
			name: "Captures correctly with new lines, tabs and quotes",
			line: `
				"test.\n\nhttps://github.com/your-ko/link-validator\n\nhttps://github.com/your-ko/link-validator"
			`,
			want: []string{
				"https://github.com/your-ko/link-validator",
				"https://github.com/your-ko/link-validator",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := p.ExtractLinks(tt.line)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ExtractLinks mismatch\nline=%q\ngot = %#v\nwant= %#v", tt.line, got, tt.want)
			}
		})
	}
}

func TestInternalLinkProcessor_ParseGitHubUrl(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    *ghURL
		wantErr bool
	}{
		{
			name: "repo url",
			url:  "https://github.com/your-ko/link-validator/",
			want: &ghURL{
				host:  "github.com",
				typ:   "repo",
				owner: "your-ko",
				repo:  "link-validator",
			},
		},
		{
			name: "repo url blob",
			url:  "https://github.com/your-ko/link-validator/blob/main/README.md",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "blob",
				ref:   "main",
				path:  "README.md",
			},
		},
		{
			name: "repo url blob with nested directories",
			url:  "https://github.com/your-ko/link-validator/blob/main/docs/README.md",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "blob",
				ref:   "main",
				path:  "docs/README.md",
			},
		},
		{
			name: "repo url blob referencing file in tag",
			url:  "https://github.com/your-ko/link-validator/blob/1.0.0/README.md",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "blob",
				ref:   "1.0.0",
				path:  "README.md",
			},
		},
		{
			name: "repo url raw",
			url:  "https://github.com/your-ko/link-validator/raw/main/README.md",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "raw",
				ref:   "main",
				path:  "README.md",
			},
		},
		{
			name: "repo url tree",
			url:  "https://github.com/your-ko/link-validator/tree/main/README.md",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "tree",
				ref:   "main",
				path:  "README.md",
			},
		},
		{
			name: "refs heads particular file",
			url:  "https://github.com/your-ko/link-validator/tree/refs/heads/main/Dockerfile",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "tree",
				ref:   "refs",
				path:  "heads/main/Dockerfile",
			},
		},
		{
			name: "repo url blame",
			url:  "https://github.com/your-ko/link-validator/blame/main/README.md",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "blame",
				ref:   "main",
				path:  "README.md",
			},
		},
		{
			name: "tree: nested dir",
			url:  "https://github.com/your-ko/link-validator/tree/main/cmd",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "tree",
				ref:   "main",
				path:  "cmd",
			},
		},
		{
			name: "branches: particular branch",
			url:  "https://github.com/your-ko/link-validator/tree/test",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "tree",
				ref:   "test",
			},
		},
		{
			name: "enterprise repo url blob",
			url:  "https://github.mycorp.com/your-ko/link-validator/blob/main/README.md",
			want: &ghURL{
				enterprise: true,
				host:       "github.mycorp.com",
				owner:      "your-ko",
				repo:       "link-validator",
				typ:        "blob",
				ref:        "main",
				path:       "README.md",
			},
		},
		{
			name: "repo url blob with anchor",
			url:  "https://github.com/your-ko/link-validator/blob/main/README.md#features",
			want: &ghURL{
				host:   "github.com",
				owner:  "your-ko",
				repo:   "link-validator",
				typ:    "blob",
				ref:    "main",
				path:   "README.md",
				anchor: "features",
			},
		},
		{
			name: "Referencing a file in a particular commit",
			url:  "https://github.com/your-ko/link-validator/blob/83e43288254d0f36e723ef2cf3328b8b77836560/README.md",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "blob",
				ref:   "83e43288254d0f36e723ef2cf3328b8b77836560",
				path:  "README.md",
			},
		},
		{
			name: "repo commits",
			url:  "https://github.com/your-ko/link-validator/commits",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "commits",
			},
		},
		{
			name: "particular commit",
			url:  "https://github.com/your-ko/link-validator/commit/a96366f66ffacd461de10a1dd561ab5a598e9167",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "commit",
				ref:   "a96366f66ffacd461de10a1dd561ab5a598e9167",
			},
		},
		{
			name: "releases",
			url:  "https://github.com/your-ko/link-validator/releases",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "releases",
			},
		},
		{
			name: "release: latest",
			url:  "https://github.com/your-ko/link-validator/releases/latest",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "releases",
				path:  "latest",
			},
		},
		{
			name: "release: particular with tag",
			url:  "https://github.com/your-ko/link-validator/releases/tag/1.0.0",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "releases",
				ref:   "tag",
				path:  "1.0.0",
			},
		},
		{
			name: "release: particular without tag",
			url:  "https://github.com/your-ko/link-validator/releases/1.0.0",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "releases",
				path:  "1.0.0",
			},
		},
		{
			name: "release: particular release without tag",
			url:  "https://github.com/your-ko/link-validator/releases/1.0.0",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "releases",
				path:  "1.0.0",
			},
		},
		{
			name: "release: download artifact",
			url:  "https://github.com/your-ko/link-validator/releases/download/1.0.0/sbom.spdx.json",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "releases",
				ref:   "download",
				path:  "1.0.0/sbom.spdx.json",
			},
		},
		{
			name: "issues url",
			url:  "https://github.com/your-ko/link-validator/issues",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "issues",
			},
		},
		{
			name: "particular issue",
			url:  "https://github.com/your-ko/link-validator/issues/123",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "issues",
				ref:   "123",
			},
		},
		{
			name: "pull requests list",
			url:  "https://github.com/your-ko/link-validator/pulls",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "pulls",
			},
		},
		{
			name: "pull: pr with an author",
			url:  "https://github.com/your-ko/link-validator/pulls/your-ko",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "pulls",
			},
		},
		{
			name: "pull: particular PR",
			url:  "https://github.com/your-ko/link-validator/pull/1",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "pull",
				ref:   "1",
			},
		},
		{
			name: "pull:  files tab",
			url:  "https://github.com/your-ko/link-validator/pull/1/files",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "pull",
				ref:   "1",
				path:  "files",
			},
		},
		{
			name: "pull: commits",
			url:  "https://github.com/your-ko/link-validator/pull/1/commits",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "pull",
				ref:   "1",
				path:  "commits",
			},
		},
		{
			name: "pull: issue comment",
			url:  "https://github.com/your-ko/link-validator/pull/1#issuecomment-123456",
			want: &ghURL{
				host:   "github.com",
				owner:  "your-ko",
				repo:   "link-validator",
				typ:    "pull",
				ref:    "1",
				anchor: "issuecomment-123456",
			},
		},
		{
			name: "pull: discussion",
			url:  "https://github.com/your-ko/link-validator/pull/1#discussion_r123456",
			want: &ghURL{
				host:   "github.com",
				owner:  "your-ko",
				repo:   "link-validator",
				typ:    "pull",
				ref:    "1",
				anchor: "discussion_r123456",
			},
		},
		{
			name: "repo url commits in branch",
			url:  "https://github.com/your-ko/link-validator/commits/test",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "commits",
				ref:   "test",
			},
		},
		{
			name: "repo url to  discussions",
			url:  "https://github.com/your-ko/link-validator/discussions",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "discussions",
			},
		},
		{
			name: "repo url to a particular discussion",
			url:  "https://github.com/your-ko/link-validator/discussions/test",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "discussions",
				ref:   "test",
			},
		},
		{
			name: "branches: list",
			url:  "https://github.com/your-ko/link-validator/branches",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "branches",
			},
		},
		{
			name: "branches: list",
			url:  "https://github.com/your-ko/link-validator/search?q=blah",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "search",
			},
		},

		{
			name: "milestones: list",
			url:  "https://github.com/your-ko/link-validator/milestones",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "milestones",
			},
		},
		{
			name: "milestones: particular milestone",
			url:  "https://github.com/your-ko/link-validator/milestone/test",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "milestone",
				ref:   "test",
			},
		},
		{
			name: "repo url projects",
			url:  "https://github.com/your-ko/link-validator/projects",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "projects",
			},
		},
		{
			name: "repo url to a particular project",
			url:  "https://github.com/orgs/your-ko/projects/1",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				typ:   "orgs",
				path:  "projects/1",
			},
		},
		{
			name: "repo settings",
			url:  "https://github.com/your-ko/link-validator/settings/secrets/actions",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "settings",
			},
		},
		{
			name: "Repository settings tabs",
			url:  "https://github.com/your-ko/link-validator/settings/actions",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "settings",
			},
		},
		{
			name: "Security: advisories",
			url:  "https://github.com/your-ko/link-validator/security/advisories",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "security",
			},
		},
		{
			name: "Security: particular advisory",
			url:  "https://github.com/your-ko/link-validator/security/advisories/test",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "advisories",
				ref:   "test",
			},
		},
		{
			name: "Security: policy",
			url:  "https://github.com/your-ko/link-validator/security/policy",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "security",
			},
		},
		{
			name: "compare branches",
			url:  "https://github.com/your-ko/link-validator/compare/main...dev",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "compare",
				ref:   "main...dev",
			},
		},
		{
			name: "compare branches (no default branch specified)",
			url:  "https://github.com/your-ko/link-validator/compare/dev",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "compare",
				ref:   "dev",
			},
		},
		{
			name: "workflows: list",
			url:  "https://github.com/your-ko/link-validator/actions",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "actions",
			},
		},
		{
			name: "workflows: particular workflow run",
			url:  "https://github.com/your-ko/link-validator/actions/runs/19221003183",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "actions",
				ref:   "runs",
				path:  "19221003183",
			},
		},
		{
			name: "workflows: badge",
			url:  "https://github.com/your-ko/link-validator/actions/workflows/main.yaml/badge.svg",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "actions",
				ref:   "workflows",
				path:  "main.yaml/badge.svg",
			},
		},
		{
			name: "workflow: file",
			url:  "https://github.com/your-ko/link-validator/actions/workflows/main.yaml",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "actions",
				ref:   "workflows",
				path:  "main.yaml",
			},
		},
		{
			name: "Workflows: particular run attempt",
			url:  "https://github.com/your-ko/link-validator/actions/runs/19221003178/attempts/1",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "actions",
				ref:   "runs",
				path:  "19221003178/attempts/1",
			},
		},
		{
			name: "Workflows: particular run attempt logs",
			url:  "https://github.com/your-ko/link-validator/actions/runs/19221003178/attempts/1/logs",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "actions",
				ref:   "runs",
				path:  "19221003178/attempts/1/logs",
			},
		},
		{
			name: "Workflow: logs of specific attempt",
			url:  "https://github.com/your-ko/link-validator/actions/runs/19221003178/job/54938961245",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "actions",
				ref:   "runs",
				path:  "19221003178/job/54938961245",
			},
		},
		{
			name: "Workflows: specific job usage",
			url:  "https://github.com/your-ko/link-validator/actions/runs/19221003178/usage",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "actions",
				ref:   "runs",
				path:  "19221003178/usage",
			},
		},
		{
			name: "repo url tags",
			url:  "https://github.com/your-ko/link-validator/tags",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "tags",
			},
		},
		{
			name: "repo url labels",
			url:  "https://github.com/your-ko/link-validator/labels",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "labels",
			},
		},
		{
			name: "repo url particular labels",
			url:  "https://github.com/your-ko/link-validator/labels/test",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "labels",
			},
		},
		{
			name: "github user",
			url:  "https://github.com/your-ko",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				typ:   "user",
			},
		},
		{
			name: "repo: attestations",
			url:  "https://github.com/your-ko/link-validator/attestations",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "attestations",
			},
		},
		{
			name: "repo: attestations",
			url:  "https://github.com/your-ko/link-validator/attestations/13059584",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "attestations",
				ref:   "13059584",
			},
		},
		{
			name: "organizations: settings",
			url:  "https://github.mycorp.com/organizations/your-ko/settings/apps/test",
			want: &ghURL{
				enterprise: true,
				host:       "github.mycorp.com",
				typ:        "orgs",
				owner:      "your-ko",
				path:       "settings/apps/test",
			},
		},
		{
			name: "organization root",
			url:  "https://github.mycorp.com/settings/organizations",
			want: &ghURL{
				enterprise: true,
				host:       "github.mycorp.com",
				typ:        "nope",
				path:       "organizations",
			},
		},
		{
			name: "organization dashboard",
			url:  "https://github.mycorp.com/orgs/your-ko/repositories",
			want: &ghURL{
				enterprise: true,
				host:       "github.mycorp.com",
				typ:        "orgs",
				owner:      "your-ko",
				path:       "repositories",
			},
		},
		{
			name: "organization people",
			url:  "https://github.mycorp.com/orgs/your-ko/people",
			want: &ghURL{
				enterprise: true,
				host:       "github.mycorp.com",
				typ:        "orgs",
				owner:      "your-ko",
				path:       "people",
			},
		},
		{
			name: "repo: packages root",
			url:  "https://github.com/your-ko/link-validator/packages",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "packages",
			},
		},
		{
			name: "repo container package",
			url:  "https://github.com/your-ko/link-validator/pkgs/container/link-validator",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "pkgs",
			},
		},
		//{
		//	name: "api pull request url",
		//	url:  "https://api.github.com/repos/your-ko/link-validator",
		//	want: nil,
		//},
		//{
		//	name: "uploads repo url",
		//	url:  "https://uploads.github.mycorp.com/org/repo/raw/main/image.png",
		//	want: nil,
		//},
		{
			name: "GitHub",
			url:  "https://github.com",
			want: &ghURL{host: "github.com"},
		},
		{
			name: "GitHub enterprise",
			url:  "https://github.mycorp.com",
			want: &ghURL{host: "github.mycorp.com", enterprise: true},
		},
		{
			name: "user profile",
			url:  "https://github.com/your-ko",
			want: &ghURL{host: "github.com", owner: "your-ko", typ: "user"},
		},
		{
			name: "repo wiki root",
			url:  "https://github.com/your-ko/link-validator/wiki",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "wiki",
			},
		},
		{
			name: "repo wiki page",
			url:  "https://github.com/your-ko/link-validator/wiki/Installation-Guide",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "wiki",
				ref:   "Installation-Guide",
			},
		},
		{
			name: "repo wiki page history",
			url:  "https://github.com/your-ko/link-validator/wiki/Installation-Guide/_history",
			want: &ghURL{
				host:  "github.com",
				owner: "your-ko",
				repo:  "link-validator",
				typ:   "wiki",
				ref:   "Installation-Guide",
				path:  "_history",
			},
		},
		{
			name: "github api",
			url:  "https://github.com/api/v3/repos/xxxx",
			want: &ghURL{
				host: "github.com",
				typ:  "nope",
				path: "v3/repos/xxxx",
			},
		},
		//// Gist URLs - these should return nil as they use gist.github.com
		//{
		//	name: "gist root",
		//	url:  "https://gist.github.com/your-ko/e8c76f357ca09860f5a0a9afb461190e",
		//	want: nil, // gist.github.com is not handled by current parser
		//},
		//{
		//	name: "gists",
		//	url:  "https://gist.github.com/your-ko",
		//	want: nil, // gist.github.com is not handled by current parser
		//},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := parseUrl(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("Unexpected error:\n%s", err)
				t.Fail()
			}
			if !reflect.DeepEqual(res, tt.want) {
				t.Errorf("FindStringSubmatch()\n got = %v\nwant = %v", res, tt.want)
			}
		})
	}
}

func Test_joinPath(t *testing.T) {
	tests := []struct {
		name  string
		parts []string
		want  string
	}{
		{
			name:  "empty slice",
			parts: []string{},
			want:  "",
		},
		{
			name:  "single non-empty element",
			parts: []string{"README.md"},
			want:  "README.md",
		},
		{
			name:  "single non-empty element with trailing empty",
			parts: []string{"README.md", ""},
			want:  "README.md",
		},
		{
			name:  "two elements",
			parts: []string{"docs", "README.md"},
			want:  "docs/README.md",
		},
		{
			name:  "multiple elements",
			parts: []string{"aaa", "bbb", "ccc", "ddd", "README.md"},
			want:  "aaa/bbb/ccc/ddd/README.md",
		},
		{
			name:  "multiple elements with trailing empty",
			parts: []string{"src", "docs", "README.md", ""},
			want:  "src/docs/README.md",
		},
		{
			name:  "all empty strings",
			parts: []string{"", "", ""},
			want:  "",
		},
		{
			name:  "mixed with empty at end",
			parts: []string{".github", "workflows", "pr.yml", "", "", ""},
			want:  ".github/workflows/pr.yml",
		},
		{
			name:  "single empty string",
			parts: []string{""},
			want:  "",
		},
		{
			name:  "complex path from URL parsing",
			parts: []string{"blob", "main", "pkg", "github", "validator.go", "", "", "", "", ""},
			want:  "blob/main/pkg/github/validator.go",
		},
		{
			name:  "nil slice",
			parts: nil,
			want:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := joinPath(tt.parts); got != tt.want {
				t.Errorf("joinPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

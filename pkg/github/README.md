# GitHub URL Routing

URLs are routed to one of three groups:

1. **GitHub API** — validated via GitHub API (no HTTP requests)
2. **GitHub HTTP** — repo/user context checked via API first, then HTTP GET for the actual page
3. **HTTP processor** — plain HTTP GET, no GitHub API involved

The split between groups 2 and 3 is controlled by two things:
- `regex.GitHub` — determines whether the GitHub processor claims the URL at all
- `Excludes()` — tells the HTTP processor which URLs the GitHub processor already owns

---

## Route Table

| URL pattern                                     | regex.GitHub | Excluded from GitHub | Processor | Handler                                                 | Notes                                  |
|-------------------------------------------------|:------------:|:--------------------:|-----------|---------------------------------------------------------|----------------------------------------|
| `github.com/owner/repo/blob/…`                  |      ✅       |                      | GitHub    | APIHandler → `handleContents`                           | Full file existence check              |
| `github.com/owner/repo/tree/…`                  |      ✅       |                      | GitHub    | APIHandler → `handleContents`                           |                                        |
| `github.com/owner/repo/raw/…`                   |      ✅       |                      | GitHub    | APIHandler → `handleContents`                           |                                        |
| `github.com/owner/repo/blame/…`                 |      ✅       |                      | GitHub    | APIHandler → `handleContents`                           |                                        |
| `github.com/owner/repo/commit/…`                |      ✅       |                      | GitHub    | APIHandler → `handleCommit`                             |                                        |
| `github.com/owner/repo/commits/…`               |      ✅       |                      | GitHub    | APIHandler → `handleCommit`                             |                                        |
| `github.com/owner/repo/compare/…`               |      ✅       |                      | GitHub    | APIHandler → `handleCompareCommits`                     |                                        |
| `github.com/owner/repo/pull/…`                  |      ✅       |                      | GitHub    | APIHandler → `handlePull`                               |                                        |
| `github.com/owner/repo/pulls`                   |      ✅       |                      | GitHub    | APIHandler → `handleRepoExist`                          |                                        |
| `github.com/owner/repo/issues/…`                |      ✅       |                      | GitHub    | APIHandler → `handleIssue`                              |                                        |
| `github.com/owner/repo/milestone/…`             |      ✅       |                      | GitHub    | APIHandler → `handleMilestone`                          |                                        |
| `github.com/owner/repo/milestones`              |      ✅       |                      | GitHub    | APIHandler → `handleRepoExist`                          |                                        |
| `github.com/owner/repo/releases/…`              |      ✅       |                      | GitHub    | APIHandler → `handleReleases`                           |                                        |
| `github.com/owner/repo/actions/…`               |      ✅       |                      | GitHub    | APIHandler → `handleWorkflow`                           |                                        |
| `github.com/owner/repo/security/advisories/…`   |      ✅       |                      | GitHub    | APIHandler → `handleSecurityAdvisories`                 |                                        |
| `github.com/owner/repo/security/…`              |      ✅       |                      | GitHub    | APIHandler → `handleRepoExist`                          |                                        |
| `github.com/owner/repo/pkgs/…`                  |      ✅       |                      | GitHub    | APIHandler → `handlePackages`                           |                                        |
| `github.com/owner/repo/packages`                |      ✅       |                      | GitHub    | APIHandler → `handlePackages`                           |                                        |
| `github.com/owner/repo/labels`                  |      ✅       |                      | GitHub    | APIHandler → `handleRepoExist`                          |                                        |
| `github.com/owner/repo/branches`                |      ✅       |                      | GitHub    | APIHandler → `handleRepoExist`                          |                                        |
| `github.com/owner/repo/tags`                    |      ✅       |                      | GitHub    | APIHandler → `handleRepoExist`                          |                                        |
| `github.com/owner/repo/settings/…`              |      ✅       |                      | GitHub    | APIHandler → `handleRepoExist`                          |                                        |
| `github.com/owner/repo/settings/environments/…` |      ✅       |                      | GitHub    | APIHandler → `handleEnvironments`                       |                                        |
| `github.com/orgs/org/teams/slug`                |      ✅       |                      | GitHub    | APIHandler → `handleTeams`                              |                                        |
| `github.com/orgs/org/…`                         |      ✅       |                      | GitHub    | APIHandler → `handleOrgExist`                           |                                        |
| `github.com/owner` (user profile)               |      ✅       |                      | GitHub    | APIHandler → `handleUser`                               |                                        |
| `github.com/owner/repo`                         |      ✅       |                      | GitHub    | APIHandler → `handleRepoExist`                          |                                        |
| `gist.github.com/user/id`                       |      ✅       |                      | GitHub    | APIHandler → `handleGist`                               |                                        |
| `github.com/owner/repo/wiki/…`                  |      ✅       |                      | GitHub    | HTTPHandler → `handleHttp` → `getRepository` + HTTP GET | Not in GitHub API                      |
| `github.com/owner/repo/discussions/…`           |      ✅       |                      | GitHub    | HTTPHandler → `handleHttp` → `getRepository` + HTTP GET | Not in GitHub API                      |
| `github.com/owner/repo/attestations/…`          |      ✅       |                      | GitHub    | HTTPHandler → `handleHttp` → `getRepository` + HTTP GET | Not in GitHub API                      |
| `github.com/owner/repo/projects/…`              |      ✅       |                      | GitHub    | HTTPHandler → `handleHttp` → `getRepository` + HTTP GET | Not in GitHub API                      |
| `github.com/owner/repo/assets/…`                |      ✅       |                      | GitHub    | HTTPHandler → `handleHttp` → `getRepository` + HTTP GET | CDN-served assets                      |
| `github.com/users/user/projects/…`              |      ✅       |                      | GitHub    | HTTPHandler → `handleHttp` → `getUser` + HTTP GET       | User-level projects                    |
| `github.com/settings/…`                         |      ✅       |                      | GitHub    | `handleNothing`                                         | Global settings, no repo context       |
| `github.com/search/…`                           |      ✅       |                      | GitHub    | `handleNothing`                                         | Search page                            |
| `github.com/api/v3/…`                           |      ✅       |                      | GitHub    | `handleNothing`                                         | GHES REST path prefix                  |
| `github.com/features/…`                         |      ✅       | ✅ (via `Excludes()`) | HTTP      | plain HTTP GET                                          | Marketing pages, excluded explicitly   |
| `api.github.com/…`                              |      ❌       |                      | HTTP      | plain HTTP GET                                          | `regex.GitHub` doesn't match this host |
| `raw.githubusercontent.com/…`                   |      ❌       |                      | HTTP      | plain HTTP GET                                          | `regex.GitHub` doesn't match this host |
| `uploads.github.com/…`                          |      ❌       |                      | HTTP      | plain HTTP GET                                          | `regex.GitHub` doesn't match this host |
| `docs.github.com/…`                             |      ❌       |                      | HTTP      | plain HTTP GET                                          | `regex.GitHub` doesn't match this host |
| `github.blog/…`                                 |      ❌       |                      | HTTP      | plain HTTP GET                                          | `regex.GitHub` doesn't match this host |

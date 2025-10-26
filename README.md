[![Main](https://github.com/your-ko/link-validator/actions/workflows/main.yaml/badge.svg)](https://github.com/your-ko/link-validator/actions/workflows/main.yaml)
[![golangci-lint](https://github.com/your-ko/link-validator/actions/workflows/golangci-lint.yaml/badge.svg)](https://github.com/your-ko/link-validator/actions/workflows/golangci-lint.yaml)
[![Link validation](https://github.com/your-ko/link-validator/actions/workflows/workflow-link-validator.yaml/badge.svg)](https://github.com/your-ko/link-validator/actions/workflows/workflow-link-validator.yaml)

# Link Validator

Catch broken links in your repo **before** they bite you in an incident.  
This tool scans Markdown files for:
- **GitHub links** (to files, PRs, issues, releases, workflows, etc.)
- **External HTTP(S) links**
- **Local Markdown links** (e.g., `./README.md`, `../docs/intro.md`)

It works on **github.com** and **GitHub Enterprise (GHES)**.

## Features
- ✅ Validates GitHub links (files, PRs, issues, releases, workflows)
- ✅ Checks external HTTP(S) links
- ✅ Verifies local Markdown file references
- ✅ Supports GitHub Enterprise Server (GHES)
- ✅ Rate limiting and authentication support
- ✅ Docker support for easy CI integration

---

## Quick start (GitHub Actions)

```yaml
name: Link validation
on:
  push:
    branches: [ main, master ]
  pull_request:

permissions:
  contents: read

env:
  DOCKER_VALIDATOR: ghcr.io/your-ko/link-validator:0.18.0 # pin a version

jobs:
  link-validator:
    runs-on: ubuntu-22.04     # set your own runner
    steps:
      - name: Checkout
        uses: actions/checkout@v5.0.0

      - name: Run Link validation
        env:
          # Logging
          LV_LOG_LEVEL: "info"
          # What to scan
          LV_FILE_MASKS: "*.md"
          # github.com auth (optional but recommended to reduce rate-limiting)
          LV_PAT: ${{ secrets.GITHUB_TOKEN }}
          # GitHub Enterprise (optional)
          LV_CORP_URL: ""
          LV_CORP_PAT: ${{ secrets.CORP_GITHUB_TOKEN }}
        run: |
          DOCKER_ENV_ARGS=""
          for var in $(env | grep '^LV_' | cut -d'=' -f1); do
            DOCKER_ENV_ARGS="$DOCKER_ENV_ARGS -e $var"
          done

          docker run --rm \
            --env-file .env \
            -v "${{ github.workspace }}:/work" \
            -w /work \
            "${{ env.DOCKER_VALIDATOR }}"
```
## Versioning
Link-validator uses semver.

## Behavior in CI

The container exits non‑zero if broken links are found or a hard error occurs, so the job fails appropriately.

Use LV_LOG_LEVEL=debug temporarily to investigate flakiness (rate‑limits, redirects, etc.).

Tip: keep the image pinned to a tag (e.g., 0.18.0) rather than latest to avoid surprise changes.

The used docker image size is approximately 10Mb.

## Configuration

| Env var         | Required | Description                                                                                                                                              | Default |
|-----------------|----------|----------------------------------------------------------------------------------------------------------------------------------------------------------|---------|
| `LV_LOG_LEVEL`  | false    | Controls verbosity.                                                                                                                                      | 'info'  |
| `LV_FILE_MASKS` | false    | Comma‑separated list of filemasks to scan (e.g., "*.md").                                                                                                | '*.md'  |
| `LV_PAT`        | false    | github.com token. Optional, but reduces throttling and enables checks against private repos you can access.                                              | ''      |
| `LV_CORP_URL`   | false    | Base URL of GHES (e.g., https://[github].[mycorp].com). When set, links on this domain and its subdomains are resolved via the GitHub API using LV_CORP_PAT. | ''      |
| `LV_CORP_PAT`   | false    | PAT for GHES with read access to the referenced repos. Mandatory if LV_CORP_URL is set                                                                   | ''      |

### Token scopes

GitHub.com: GITHUB_TOKEN (default) works for most CI‑read needs; classic PATs can use public_repo / repo.

GHES: a PAT with read access to repositories referenced by your docs.

## How it works
The validator uses three specialized processors:

* GitHub processor – resolves GitHub UI links to API calls (files, PRs/issues, releases, workflows/badges, etc.) and validates existence.
* HTTP(S) processor – checks non‑GitHub links. Follows redirects, treats 2xx as OK, distinguishes common cases (401/403 = private/gated, 404/410 = not found, 429 = rate limit, 5xx = transient server error).
* Local‑path processor – parses Markdown links that reference local files (./a.md, ../b/c.md) and verifies they exist (and optional anchors if present).

This split keeps checks fast and accurate while avoiding false positives common with generic link crawlers.

## Running locally
```shell
export LV_LOG_LEVEL=debug
export LV_FILE_MASKS="*.md"
export LV_PAT=ghp_...           # optional
export LV_CORP_URL=             # optional, e.g., https://[github].[mycorp].com
export LV_CORP_PAT=             # optional

docker run --rm \
  --env LV_LOG_LEVEL \
  --env LV_FILE_MASKS \
  --env LV_PAT \
  --env LV_CORP_URL \
  --env LV_CORP_PAT \
  -v "$PWD:/work" -w /work \
  ghcr.io/your-ko/link-validator:0.18.0
```

## Troubleshooting
### I get redirected to /login for GHES links
Use LV_CORP_URL and LV_CORP_PAT. 
The validator talks to the GHES API (not the HTML UI), which requires a token for private content. 
Also ensure your runner bypasses the corporate proxy for your GHES host (NO_PROXY=github.mycorp.com) and trusts your internal CA if applicable.

### “Too many redirects” or 3xx loops
This often indicates an auth or proxy issue. Run with LV_LOG_LEVEL=debug to see hop targets and fix the root cause (token, proxy, base URL).

### 429 (rate limited)
Provide LV_PAT so requests are authenticated and receive higher limits.

## Exit codes

0 – success (no broken links)
>0 – failures detected or unrecoverable error

Use these to gate merges in CI.

## Security
Tokens are read from env vars only and used to call the GitHub API for validation.

Prefer repository‑scoped tokens (GITHUB_TOKEN) in CI; restrict PAT scopes to the minimum necessary.

## Why this tool?
As repos grow, docs link across services and between repos. Over time, links rot—repos archived, files moved, or pages gated by new auth. 
This validator catches issues early, in the PR that introduces them.

## License
The scripts and documentation in this project are released under the [MIT License](./LICENSE)


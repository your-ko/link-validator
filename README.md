[![Main](https://github.com/your-ko/link-validator/actions/workflows/main.yaml/badge.svg)](https://github.com/your-ko/link-validator/actions/workflows/main.yaml)
[![golangci-lint](https://github.com/your-ko/link-validator/actions/workflows/golangci-lint.yaml/badge.svg)](https://github.com/your-ko/link-validator/actions/workflows/golangci-lint.yaml)
[![Link validation](https://github.com/your-ko/link-validator/actions/workflows/link-validator.yaml/badge.svg)](https://github.com/your-ko/link-validator/actions/workflows/link-validator.yaml)

# Link Validator

Validates links and URLs in Markdown files by checking:
- GitHub links (files, PRs, issues, releases, workflows, etc.)
- External HTTP(S) URLs
- Local file references (`./README.md`, `../docs/intro.md`)

Supports both github.com and GitHub Enterprise Server (GHES).

## Features

- GitHub link validation via API calls
- HTTP(S) link checking with redirect following
- Local Markdown file path verification
- GitHub Enterprise Server support
- Authentication and rate limiting
- Dockerized for CI integration


## Why Use This?

Documentation with broken links is frustrating for users and reflects poorly on your project. Common problems:

- **Files get moved or renamed** - internal links break when you restructure docs
- **External sites change URLs** - third-party links rot over time
- **Private repos become inaccessible** - links work for maintainers but fail for contributors
- **API endpoints get deprecated** - GitHub URLs change when features are moved or removed

Running this in CI catches broken links during PR review instead of after merge. Much easier to fix a link when the author is still working on the change.

This tool understands GitHub's URL patterns and uses the API for accurate validation.

## GitHub Actions Setup

Link-validator can be used either as a independent GitHub workflow or as a GitHub action.

### GitHub action

This can be added, for example, into the workflow that runs on PR. In this case if there are broken links in the documentation,
the step will fail (will be improved in the coming releases)

```yaml
    - name: Link validation
      uses: your-ko/link-validator@1.0.0
      with:
        log-level: 'info'
        pat: ${{ secrets.GITHUB_TOKEN }}
```

### GitHub workflow
This can be added as an independent workflow. Then the scan will be done for the whole repository on the push event, 
so you can always see the status of the documentation. 

```yaml
name: Link validation
on:
  push:
    branches: [ main, master ]
  pull_request:

permissions:
  contents: read  #  # Required to checkout code and read files

env:
  DOCKER_VALIDATOR: ghcr.io/your-ko/link-validator:1.0.0 # pin a version or use 'latest'

jobs:
  link-validator:
    runs-on: ubuntu-22.04     # set your own runner
    steps:
      - name: Checkout
        uses: actions/checkout@v5.0.0

      - name: Run Link validation
        env:
          LOG_LEVEL: "info"
          FILE_MASKS: "*.md"
          PAT: ${{ secrets.GITHUB_TOKEN }}
          CORP_URL: ""
          CORP_PAT: ${{ secrets.CORP_GITHUB_TOKEN }}
        run: |
          DOCKER_ENV_ARGS=""
          for var in $(env | grep '^' | cut -d'=' -f1); do
            DOCKER_ENV_ARGS="$DOCKER_ENV_ARGS -e $var"
          done

          docker run --rm \
            $DOCKER_ENV_ARGS \
            -v "${{ github.workspace }}:/work" \
            -w /work \
            "${{ env.DOCKER_VALIDATOR }}"
```

### Call GitHub workflow

```yaml
jobs:
  link-validation:
    uses: your-ko/link-validator/.github/workflows/link-validator-workflow.yaml@1.0.0
    with:
      log-level: info
```


## Configuration

| Environment Variable | Required | Description                                                             | Default |
|----------------------|----------|-------------------------------------------------------------------------|---------|
| `LOG_LEVEL`          | No       | Controls verbosity (debug, info, warn, error)                           | `info`  |
| `FILE_MASKS`         | No       | Comma-separated file patterns to scan                                   | `*.md`  |
| `PAT`                | No       | GitHub.com personal access token. Optional. Used to avoid rate limiting | `""`    |
| `CORP_URL`           | No       | GitHub Enterprise base URL, for example https://[github].[mycorp].[com] | `""`    |
| `CORP_PAT`           | No       | GitHub Enterprise personal access token                                 | `""`    |

### Authentication

**GitHub.com**: Use `GITHUB_TOKEN` in CI or a PAT with `public_repo`/`repo` scope. Authentication is optional but recommended to avoid rate limiting.

**GitHub Enterprise**: Requires `CORP_URL` and `CORP_PAT`. The PAT needs read access to repositories referenced in your documentation.

## Implementation Details

The validator uses three specialized processors:

**GitHub processor**: Converts GitHub UI links to API endpoints and validates existence. Handles files, pull requests, issues, releases, workflow runs, and badge URLs.

**HTTP processor**: Performs HEAD/GET requests on external links. Follows redirects and interprets HTTP status codes:
- 2xx: Success
- 401/403: Private or authentication required
- 404/410: Not found
- 429: Rate limited
- 5xx: Server error

**Local processor**: Validates local file references and anchor links within Markdown files. Resolves relative paths correctly.

## Troubleshooting

**Enterprise links redirect to login page**
Configure `CORP_URL` and `CORP_PAT`. The validator uses GitHub's API which requires authentication for private repositories.

**Rate limiting (429 responses)**
Provide `PAT` to increase rate limits from 60/hour to 5000/hour.

**Redirect loops or 3xx errors**
Usually indicates authentication or proxy configuration issues. Enable debug logging with `LOG_LEVEL=debug` to trace redirect chains.

## Exit Codes

- `0`: All links validated successfully
- `>0`: Broken links found or validation errors occurred

## Docker Image

Image size: ~10MB

Recommend pinning to specific versions (e.g., `0.18.0`) rather than using `latest` for reproducible builds.

## Security & Supply Chain

### Authentication
Tokens are read from env vars only and used to call the GitHub API for validation.

PAT and Enterprise PAT should both handle the authentication.

### Supply Chain Security
All releases include signed container images with full attestations for supply chain transparency:

- **Container signing**: Images signed with Cosign using keyless signing via GitHub OIDC
- **Build attestations**: GitHub-native build provenance for complete supply chain transparency
- **SBOM**: Software Bill of Materials in SPDX format for dependency tracking
- **Provenance**: Build provenance records for reproducible builds

#### Verification Commands

Replace the version `1.3.0` below with the version you want to verify:

**Verify container signature:**
```bash
cosign verify "ghcr.io/your-ko/link-validator@sha256:[DIGEST]" \
  --certificate-oidc-issuer https://[token.actions.githubusercontent.com] \
  --certificate-identity-regexp "^https://[github.com]/[your-ko/link-validator]/\.github/workflows/.*"
```

**Verify GitHub attestations:**
```bash
gh attestation verify oci://ghcr.io/your-ko/link-validator@sha256:[DIGEST] \
  --repo your-ko/link-validator \
  --signer-workflow your-ko/link-validator/.github/workflows/release.yaml@refs/tags/1.3.0
```

**Download supply chain artifacts:**
Supply chain metadata is available for each release:
- Software Bill of Materials: `https://github.com/your-ko/link-validator/releases/download/1.3.0/sbom.spdx.json`
- Build provenance: `https://github.com/your-ko/link-validator/releases/download/1.3.0/provenance.intoto.jsonl`
- Checksums: `https://github.com/your-ko/link-validator/releases/download/1.3.0/SHASUMS256.txt` 


## Versioning

Uses semantic versioning. Check [releases](https://github.com/your-ko/link-validator/releases) for changelog.

## License

MIT License - see [LICENSE](./LICENSE)

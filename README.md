[![Main](https://github.com/your-ko/link-validator/actions/workflows/main.yaml/badge.svg)](https://github.com/your-ko/link-validator/actions/workflows/main.yaml)
[![golangci-lint](https://github.com/your-ko/link-validator/actions/workflows/golangci-lint.yaml/badge.svg)](https://github.com/your-ko/link-validator/actions/workflows/golangci-lint.yaml)
[![Link validation](https://github.com/your-ko/link-validator/actions/workflows/link-validator.yaml/badge.svg)](https://github.com/your-ko/link-validator/actions/workflows/link-validator.yaml)
[![CodeQL Advanced](https://github.com/your-ko/link-validator/actions/workflows/codeql.yml/badge.svg)](https://github.com/your-ko/link-validator/actions/workflows/codeql.yml)

# Link Validator

Validates links and URLs in Markdown files by checking:
- GitHub links (files, PRs, issues, releases, workflows, etc.)
- External HTTP(S) URLs
- Local file references (`./README.md`, `../docs/intro.md`)
- Datadog URLs (monitors, dashboards, etc)

Supports both public GitHub.com and GitHub Enterprise Server (GHES).

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

The link-validator can be used either as an independent GitHub workflow or as a GitHub action.

### GitHub action

This can be added, for example, to a workflow that runs on PRs. In this case, if there are broken links in the documentation,
the step will fail (this will be improved in future releases).

```yaml
    - name: Link validation
      uses: your-ko/link-validator@1.0.0
      with:
        log-level: 'info'
        pat: ${{ secrets.GITHUB_TOKEN }}
```
In case if you run validator in a repo, containing a lot of documentation and you don't want your PR be constantly failing,
then you can run validation only on files, updated in the RR. Then it will make it easier to "clean up" the repository from dead links. 
Then your PR pipeline should contain following steps:
```yaml
    steps:
      - name: Checkout
        uses: actions/checkout@v4.2.2
        with:
          fetch-depth: '0' # this is necessary to build the list of updated files

      - name: Get changed files
        id: changed-files
        shell: bash
        run: |
          # Get the base branch (target of PR)
          BASE_SHA="${{ github.event.pull_request.base.sha }}"
          HEAD_SHA="${{ github.event.pull_request.head.sha }}"

          # Get only added and modified files (exclude deleted files)
          CHANGED_FILES=$(git diff --name-only --diff-filter=AM $BASE_SHA..$HEAD_SHA)

          echo "Changed files in this PR (excluding deleted):"
          echo "$CHANGED_FILES"

          FILES_LIST=$(echo "$CHANGED_FILES" | tr '\n' ',' | sed 's/,$//')
          echo "files=$FILES_LIST" >> $GITHUB_OUTPUT
      - name: Link validation
        uses: your-ko/link-validator@1.9.0
        with:
          log-level: 'debug'
          files: ${{ steps.changed-files.outputs.files}}
          pat: ${{ secrets.GITHUB_TOKEN }}

```

### GitHub workflow
This can be added as an independent workflow. The scan will then be performed on the whole repository when a push event occurs,
so you can always see the status of your documentation. 

```yaml
name: Link validation
on:
  push:
    branches: [ main, master ]

permissions:
  contents: read  # Required to checkout code and read files

env:
  DOCKER_VALIDATOR: ghcr.io/your-ko/link-validator:1.0.0 # pin a version or use 'latest'

jobs:
  link-validator:
    runs-on: ubuntu-22.04     # set your own runner
    steps:
      - name: Checkout
        uses: actions/checkout@v5.0.0

      - name: Run Link validation
        run: |
          docker run --rm \
            -e 'LOG_LEVEL=debug' \
            -e 'FILE_MASKS=*.md' \
            -e 'PAT=${{ secrets.GITHUB_TOKEN }}' \
            -v "${{ github.workspace }}:/work" \
            -w /work \
            ${DOCKER_VALIDATOR}
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

| Env Variable      | Config         | Required | Description                                                                                                                                                                                                                                                                                                                                    | Default |
|-------------------|----------------|----------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|---------|
| `LOG_LEVEL`       |                | No       | Controls verbosity (debug, info, warn, error)                                                                                                                                                                                                                                                                                                  | `info`  |
| `FILE_MASKS`      | fileMasks      | No       | Comma-separated file patterns to scan. Only md and tf are tested in the current version.                                                                                                                                                                                                                                                       | `*.md`  |
| `PAT`             |                | No       | GitHub.com personal access token. Optional. Used to avoid rate limiting                                                                                                                                                                                                                                                                        | `""`    |
| `CORP_URL`        | corpUrl        | No       | GitHub Enterprise base URL, for example https://github.[mycorp].com                                                                                                                                                                                                                                                                            | `""`    |
| `CORP_PAT`        |                | No       | GitHub Enterprise personal access token                                                                                                                                                                                                                                                                                                        | `""`    |
| `DD_API_KEY`      |                | No       | DataDog API key                                                                                                                                                                                                                                                                                                                                | `""`    |
| `DD_APP_KEY`      |                | No       | DataDog APP key                                                                                                                                                                                                                                                                                                                                | `""`    |
| `IGNORED_DOMAINS` | ignoredDomains | No       | List of domains or their parts that should be ignored during validation. Comma-separated, if passed to GitHub action.                                                                                                                                                                                                                          | `[]`    |
| `TIMEOUT`         | timeout        | No       | HTTP request timeout                                                                                                                                                                                                                                                                                                                           | `5s`    |
| `FILES`           | files          | No       | List of files to run validation on. FileMask is applied on the list, <br/>so resulting list will contain files satisfying both requirements. Comma-separated, if passed to GitHub action.                                                                                                                                                      | `[]`    |
| `EXCLUDE`         | exclude        | No       | List of files or folders to exclude from validation. Is useful to exclude, for example, `/vendor` or `*/charts` because these folders can contain 3rd party documentation, which we don't need to validate. Files also possible to exclude. The path should be relative from the repository root. Comma-separated, if passed to GitHub action. | `[]`    |
| `LOOKUP_PATH`     | lookupPath     | No       | A path to look for the files up (read below).                                                                                                                                                                                                                                                                                                  | `./`    |

### Config file
The config file needs to be called `.link-validator.yaml` and must be located in the repository root.
The file can contain all configuration properties, except tokens. If a token is declared in the config file,
it is ignored, because it is a bad security practice to have tokens checked into Git.

Config file example:
```yaml
fileMasks:
  - "*.md"
timeout: 5s
validators:
  github:
    enabled: true          # If enabled, PAT/CORP_PAT must be set in ENV variables
  datadog:
    enabled: true          # If enabled, DD_API_KEY/DD_APP_KEY must be set in ENV variables
  localPath:
    enabled: true          # Usually always enabled
  http:
    enabled: true          # Usually always enabled (fallback)
    ignoredDomains:
      - https://blah.blah.org

```

How is the configuration applied?
* First, the config is populated with default values.
* Then, values from the config file are applied. If the file is not present, then this step is ignored.
* Finally, environment variables are applied.


### Additional explanation

#### IGNORED_DOMAINS 
You might have some resources in your network behind additional authentication, for example, OKTA or LDAP.
Currently, the link-validator doesn't support such authentication, so any 401 responses are treated as successful.
If you have such resources, you can explicitly list them in this variable so you know they are not validated.

This option is also useful when you have resources that are simply not accessible from GitHub runners due to network limitations.

#### LOOKUP_PATH
Be careful with this option. It sets the folder to look for the documents in. It should be inside the repository, 
otherwise it makes no sense and might not work, right? 

So keep it relative to the repository root. 

It is cancelled by `EXCLUDE`. So it you have:
```yaml
EXCLUDE=./docs
LOOKUP_PATH=./docs
```
then you get a successfully passed validation with no files.

## GitHub

**GitHub.com**: Use `GITHUB_TOKEN` in CI or a Personal Access Token (PAT) with `public_repo`/`repo` scope. Authentication is optional, but recommended to avoid rate limiting.

**GitHub Enterprise**: Requires `CORP_URL` and `CORP_PAT`. The Personal Access Token (PAT) needs read access to repositories referenced in your documentation.

### Datadog
To get APP/API keys you should go to 
* Integrations -> Organisation Setting -> Service Accounts and create a service account with the `Datadog Read Only Role`
* Then make sure that both API and APP keys are created and use these values in the env vars of the app.

Create `Read Only`, following the principle of the least privilege. 

Unfortunately Datadog API are limited so not many resources are validated at the moment. 
To avoid a lot of false negatives, I just perform "mock" validation on those URLs that are not supported by the API.

### Config file vs ENV variables
You can configure the link-validator either via environment variables:
```yaml
      - name: Run Link validation
        run: |
          docker run --rm \
            -e 'LOG_LEVEL=debug' \
            -e 'FILE_MASKS=*.md' \
            -e 'PAT=${{ secrets.GITHUB_TOKEN }}' \
            -v "${{ github.workspace }}:/work" \
            -w /work \
            ${DOCKER_VALIDATOR}
```

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

```markdown
Content of code snippets is ignored because it might contain non-parseable or non-reachable links.
```

## Troubleshooting

**Enterprise links redirect to login page**
Configure `CORP_URL` and `CORP_PAT`. The validator uses GitHub's API which requires authentication for private repositories.

**Rate limiting (429 responses)**
Provide `PAT` to increase rate limits from 60/hour to 5000/hour.

**Redirect loops or 3xx errors**
This usually indicates authentication or proxy configuration issues. Enable debug logging with `LOG_LEVEL=debug` to trace redirect chains.

## Exit Codes

- `0`: All links validated successfully
- `>0`: Broken links found or validation errors occurred

## Docker Image

Image size: ~10MB

Pinning to specific versions (e.g., `1.0.0`) is recommended rather than using `latest` for reproducible builds.

## Security

The keys and tokens should *ONLY* be passed as ENV variables!
These tokens are used only to instantiate corresponding clients (such as GitHub, DataDog, etc) and nothing else. 
They are not logged, they are not sent anywhere else. 

For security considerations, vulnerability reporting, supply chain security details, and verification instructions, see [SECURITY.md](./SECURITY.md).

## Versioning

Uses semantic versioning. Check [releases](https://github.com/your-ko/link-validator/releases) for changelog.

## License

MIT License - see [LICENSE](./LICENSE)

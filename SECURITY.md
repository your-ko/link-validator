# Security Policy

## Supported Versions

We actively support and provide security updates for the following versions:

| Version | Supported          |
|---------|--------------------|
| 1.x.x   | :white_check_mark: |
| < 1.0   | :x:                |

For the most secure experience, always use the latest stable release.

## Reporting a Vulnerability

If you discover a security vulnerability in link-validator, please help us address it responsibly.

### How to Report

**Please do NOT open a public GitHub issue for security vulnerabilities.**

Instead, please report security issues through GitHub Security Advisories:

- Go to https://github.com/your-ko/link-validator/security/advisories
- Click "Report a vulnerability"
- Provide detailed information about the vulnerability

This provides a secure, private channel for reporting and coordinating the resolution of security issues.

### What to Include

When reporting a vulnerability, please include:

- Description of the vulnerability and its potential impact
- Steps to reproduce the issue
- Affected versions
- Any possible mitigations or workarounds
- Your contact information for follow-up questions

### Response Timeline

- **Initial Response**: Within 48 hours
- **Status Update**: Within 7 days
- **Fix Timeline**: Varies based on complexity, typically 14-30 days

We will acknowledge receipt of your report and work with you to understand and resolve the issue promptly.

## Security Considerations

### Network Security

Link-validator makes HTTP/HTTPS requests to external URLs and GitHub APIs. Consider these security implications:

#### External HTTP Requests
- **Timeout Protection**: All HTTP requests have configurable timeouts (default: 3 seconds)
- **Redirect Limits**: Maximum of 3 redirects to prevent redirect loops
- **Body Size Limits**: Only reads first 10KB of response bodies
- **User-Agent**: Identifies itself as `link-validator/1.0` in requests

#### Rate Limiting Protection
- Uses GitHub tokens (PAT) to avoid rate limiting
- Handles 429 responses gracefully
- Does not retry failed requests aggressively

### Authentication & Tokens

#### GitHub Personal Access Tokens (PAT)
- **Environment Variables Only**: Tokens are only read from environment variables
- **Minimal Permissions**: Requires only `public_repo` scope for public repositories
- **Optional Usage**: Authentication is optional but recommended
- **Corporate GitHub**: Supports GitHub Enterprise Server with separate token

#### Token Security Best Practices
- Use GitHub-generated tokens with minimal required scopes
- Store tokens in secure environment variables
- Rotate tokens regularly
- Never commit tokens to version control
- Use GitHub's `GITHUB_TOKEN` in CI/CD when possible

### Input Validation

#### URL Processing
- **Regex Validation**: Uses compiled regex patterns for URL extraction
- **URL Parsing**: Validates URLs using Go's `net/url` package
- **Domain Filtering**: Supports ignored domains to prevent unwanted requests
- **Malformed URL Handling**: Skips malformed URLs gracefully

#### File Processing
- **File Mask Filtering**: Only processes files matching specified patterns (default: `*.md`)
- **Path Validation**: Validates local file paths before processing
- **Content Limits**: Processes files line by line to manage memory usage

### Supply Chain Security

#### Container Images
All published container images include comprehensive supply chain security:

- **Digital Signatures**: Images signed with Cosign using keyless signing
- **Build Attestations**: GitHub-native provenance for build transparency
- **SBOM Available**: Software Bill of Materials in SPDX format
- **Provenance Records**: Complete build provenance for reproducibility

#### Verification Commands

Replace the version `1.3.0` below with the version you want to verify:

**Verify container signature:**
```bash
cosign verify "ghcr.io/your-ko/link-validator@sha256:[DIGEST]" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp "^https://github.com/your-ko/link-validator/\.github/workflows/.*"
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

### Deployment Security

#### CI/CD Integration
- **Minimal Permissions**: Use `contents: read` permission in GitHub Actions
- **Token Scoping**: Use `GITHUB_TOKEN` with minimal required permissions
- **Pin Versions**: Always pin to specific versions instead of `latest`
- **Environment Isolation**: Run in isolated containers/environments

#### Docker Security
- **Minimal Base Images**: Uses small, secure base images (~10MB)
- **Non-root User**: Containers run as non-root user where possible
- **Read-only Filesystem**: Supports read-only container filesystems
- **No Privileged Access**: Does not require privileged container access

### Known Security Limitations

1. **External URL Trust**: The tool makes requests to external URLs found in documentation
2. **DNS Resolution**: Relies on DNS resolution which could be manipulated
3. **Response Body Inspection**: Reads response bodies which could contain malicious content but does not execute it.
4. **Redirect Following**: Follows redirects which could lead to unintended destinations

### Security Updates

Security updates are released as patch versions and communicated through:
- GitHub Security Advisories
- Release notes with `[SECURITY]` prefix
- Docker image tags with updated versions

## Responsible Disclosure

We appreciate security researchers and users who help improve the security of link-validator. We are committed to working with the security community to verify and respond to legitimate security issues.

Thank you for helping keep link-validator and its users secure!

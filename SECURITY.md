# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in DocuMCP, please report it responsibly.

**Email**: Send details to the repository maintainer via the email listed on the GitHub profile.

**Do not** open a public GitHub issue for security vulnerabilities.

## What to Include

- Description of the vulnerability
- Steps to reproduce
- Affected versions
- Potential impact

## Response

You can expect an initial response within 72 hours. We will work with you to understand the issue and coordinate a fix before any public disclosure.

## Supported Versions

| Version | Supported |
|---------|-----------|
| Latest release | Yes |
| Previous releases | Best effort |

## Security Measures

DocuMCP implements the following security controls:

- **OAuth 2.1** with mandatory PKCE (S256) for all public clients
- **AES-256-GCM** encryption at rest for stored credentials
- **HKDF** key derivation for CSRF tokens and HMAC signing
- **SSRF prevention** for user-supplied URLs
- **Rate limiting** on authentication endpoints
- **Content Security Policy**, HSTS, and security headers
- **Non-root container** runtime with minimal Alpine base
- **Supply chain**: all CI action refs SHA-pinned, Docker base images digest-pinned

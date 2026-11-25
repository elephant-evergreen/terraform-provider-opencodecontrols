# Contributing

Contributions are welcome. Write public documentation, examples, diagnostics, issues, pull requests, and commit messages in English, and run `mise run check` before opening a pull request.

## Public information

Everything published from this repository is public. Public documentation, examples, diagnostics, issues, pull requests (including descriptions and comments), commit messages, and release notes must not contain:

- Private repository names or internal project names.
- Hostnames, LAN addresses, or SSIDs.
- Per-device update or verification history.

Use synthetic values in examples and fixtures, including RFC 5737 addresses such as `192.0.2.0/24`, `198.51.100.0/24`, and `203.0.113.0/24`. Document supported and tested versions separately from any per-device history.

Use synthetic OpenCode API keys, organization/project IDs, and fixtures in tests and documentation. Do not include real keys, identifiers, Terraform state, or saved plans.

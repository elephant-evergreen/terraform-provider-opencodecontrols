# OpenCode Controls

A community Terraform provider for OpenCode Console service account API keys.
The initial release is opt-in alpha. Use it to issue restricted keys, list key
metadata, revoke keys, and adopt existing keys without rotating them.

- [Configure the provider](docs/index.md)
- [Manage or import a key](docs/resources/service_account_api_key.md)
- [List keys](docs/data-sources/service_account_api_keys.md)

Create a separate management key in OpenCode Console. Give it service account
read access, permission to write its own account's API keys, and the scopes the
issued keys need. The provider connects directly to OpenCode using that key.

OpenCode reveals newly issued values once. Terraform records these values in
state. Import only reads metadata. Protect state and saved plans accordingly.

## Development

Install the pinned tools with `mise install` and run `mise run check`.
See [RELEASE.md](RELEASE.md) for signed releases and Registry verification.

# Release OpenCode Controls

Use `v0.0.1-alpha.1` for the first opt-in release. Increase the alpha number for
subsequent trials. Use `v0.0.1` when the provider is ready for general selection.

1. Run `mise run check`. Verify creation, refresh, import, and revocation with a
   fixture account. Record any tests run against OpenCode Console in the notes.
2. Add `release-notes/<tag>.md` with upgrade guidance and the verified scope.
3. Merge the reviewed changes. After Check passes on main, push an annotated
   tag on that commit.
4. The Release workflow builds and signs four packages, uploads them to a draft,
   checks the remote digests, and publishes the immutable release.
5. On first publication, select the repository in the Terraform Cloud GitHub App
   and publish it in the public `elephant-evergreen` namespace. Verify the exact
   version installs from Terraform Registry in an empty directory.

The release uses the repository's `GPG_PRIVATE_KEY` Actions secret. The public
key in `keys/release.asc` must be registered in the Terraform Registry namespace.
Use a new tag to correct a published release.

[Terraform Registry publishing](https://developer.hashicorp.com/terraform/registry/providers/publishing)

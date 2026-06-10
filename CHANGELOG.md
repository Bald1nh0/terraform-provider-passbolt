# Changelog

## v1.10.0 — 2026-06-10

### ✨ Added

- Added `metadata_type` opt-in for `passbolt_password` and `passbolt_folder` to explicitly create or upgrade managed items to Passbolt v5 encrypted metadata while preserving the unset server-default behavior.
- Added `metadata_type_actual` on `passbolt_password` and `passbolt_folder` to expose whether the remote item is currently stored as v4 or v5 metadata.
- Added provider-side v5 encrypted metadata support for `passbolt_folder` create, read, update, import, and v4-to-v5 upgrade flows.

### 🛠 Improved

- Folder name and path resolution now decrypts v5 folder metadata, allowing `folder_parent` and `data.passbolt_folders` to work with encrypted folder names.
- `passbolt_password` can now upgrade managed v4 password resources to v5 encrypted metadata with `metadata_type = "v5"`.

## v1.9.0 — 2026-06-02

### ✨ Added

- Added Passbolt v5 encrypted metadata support for `passbolt_password` reads, imports, data source lookups, and updates by upgrading `github.com/passbolt/go-passbolt` to `v0.8.0`.
- New `passbolt_password` resources now follow the Passbolt server's default resource metadata type, creating v5 resources when the server default is encrypted metadata.

### 🛠 Improved

- Replaced the provider's legacy resource-type update switch with the schema-aware `go-passbolt` generic update flow, allowing upgraded `v5-default` resources to be refreshed instead of failing with `Unknown ResourceType: v5-default`.

## v1.8.1 — 2026-04-28

### 📝 Documentation

- Clarified that `data.passbolt_user.include_inactive` only controls user lookup, while `passbolt_group.ignore_inactive_members` controls skipping inactive regular members during group apply.

## v1.8.0 — 2026-04-28

### ✨ Added

- `data.passbolt_user.include_inactive` to opt into resolving inactive, non-deleted users by exact username.
- `data.passbolt_user.active` to expose whether the resolved user has completed activation.

## v1.7.0 — 2026-04-24

### ✨ Added

- `passbolt_group.ignore_inactive_members` to skip inactive regular members with a warning and retry them on later applies after activation.

## v1.6.0 — 2026-04-22

### ✨ Added

- `passbolt_password.password_wo` and `passbolt_password.password_wo_version` to support write-only password workflows that keep the Passbolt secret out of Terraform plan and state artifacts.

### 🛠 Improved

- `passbolt_password` now rotates write-only secrets only when `password_wo_version` changes, while preserving legacy `password` behavior for users who intentionally keep secrets in state.
- `passbolt_password` reads now preserve write-only secret state as `null` instead of rehydrating the decrypted password into Terraform state.

### 📝 Documentation

- Documented the difference between legacy `password` and write-only `password_wo` flows across the README, Registry docs, and AWS SSM guide.
- Clarified that `data.passbolt_password` and classic `data.aws_ssm_parameter` flows still persist decrypted secrets in Terraform state.
- Added an explicit legacy `password` example with a warning that the flow is less secure because Terraform persists the secret in state.

## v1.5.6 — 2026-04-21

### 🛠 Fixed

- `passbolt_group` now preserves the decrypted secret payload when re-encrypting secrets for users added to an existing shared group, fixing broken secrets that were visible but not decryptable.

## v1.5.5 — 2026-04-20

### 🛠 Fixed

- `passbolt_group.managers` and `passbolt_group.members` now use set semantics to avoid perpetual diffs caused by membership ordering returned by Passbolt.

## v1.5.4 — 2026-04-18

### 🛠 Fixed

- Updated the Terraform Plugin Framework dependency set to current compatible releases.

### 📝 Documentation

- Added `tfplugindocs` templates for provider docs, resource/data source navigation subcategories, and workflow guides.
- Added documentation validation through `tfplugindocs validate`.

## v1.5.3 — 2026-04-18

### 🛠 Fixed

- Aligned the provider server address with the published Terraform Registry source (`Bald1nh0/passbolt`).

### 📝 Documentation

- Reworked the README first screen around a registry-ready quick start, supported objects, and common use cases.
- Updated provider examples and generated documentation to use the current `~> 1.5` provider version.

## v1.5.2 — 2026-04-17

### 🛠 Fixed

- `data.passbolt_user` now resolves only exact, active, non-deleted Passbolt users.
- Improved `passbolt_user` data source behavior for deleted or stale Passbolt user records that could previously be returned by search results.

## v1.5.1 — 2026-04-17

### 🛠 Fixed

- `passbolt_group` now sends explicit regular-member role data when adding users to existing groups.
- `passbolt_group` now returns a clear error if Passbolt accepts a membership update but does not apply it, for example when the authenticated API user is not a group manager.
- Added acceptance coverage for adding a regular member to an existing group that already has a shared password.
- Bumped the Go toolchain to `1.26.2` and pinned GitHub Actions to the same version.
- Updated vulnerable transitive dependencies, including `google.golang.org/grpc` to `v1.79.3` and `github.com/cloudflare/circl` to `v1.6.3`.
- Updated GitHub Actions workflows to Node 24-compatible action majors and opted workflows into `FORCE_JAVASCRIPT_ACTIONS_TO_NODE24` for the remaining legacy tagging action.
- Generated acceptance-test emails now default to `example.com`, with optional override via `PASSBOLT_TEST_EMAIL_DOMAIN`.

## v1.5.0 — 2026-04-15

### ✨ Added

- `passbolt_group.members` now manages regular group members alongside required group managers.

### 🛠 Improved

- `passbolt_group` now validates that users are not configured as both managers and members.

## v1.4.0 — 2026-04-10

### ✨ Added

- `passbolt_folder.folder_parent` now accepts a unique folder name, folder UUID, or an absolute path such as `/application_A/prod`.
- `data "passbolt_folders"` now exposes a computed `path` attribute for each folder.

### 🛠 Improved

- Duplicate folder names now return a clear error instead of silently picking an arbitrary parent folder.
- `passbolt_folder` now stores the resolved `folder_parent_id` and applies folder moves when `folder_parent` changes.
- Acceptance tests now require `TF_ACC=1` and use unique resource names so they can run safely against a shared Passbolt instance.
- GitHub Actions CI now runs both unit tests and real acceptance tests.

## v1.2.0 — 2025-07-07

### ✨ Added

- 🔍 `data "passbolt_group"`: allows looking up groups by name and retrieving their UUID for use in `share_groups`.
- ✅ `share_groups`: new attribute for `passbolt_password` resource to support sharing with multiple groups.
- 📁 Example added: `examples/password/shared-by-group.tf` demonstrates sharing a secret using `data "passbolt_group"`.

### 🛠 Improved

- ✅ `passbolt_password`:
  - `share_groups` now takes precedence over the legacy `share_group` field.
  - Eliminated `Permission ... is already Type 7` errors by filtering out existing permissions before sharing.
  - Backward compatibility retained for `share_group`, but it is now considered deprecated.
- 🧠 Refactored `shareResourceWithGroups` and `buildPasswordState` to reduce cyclomatic complexity (`cyclop` linter).
- 💾 Preallocated slices (`make(..., 0, N)`) to optimize memory usage in tight loops (`prealloc` linter compliance).

### 🔄 Changed

- 📚 Updated `README.md`:
  - Replaced `share_group` examples with `share_groups` and `data "passbolt_group"`.
  - Added guidance for future-proof usage patterns.

---

➡️ This is a backward-compatible minor release.  
We recommend migrating from `share_group` to `share_groups` and using `data "passbolt_group"` for better UX and multi-group support.

## v1.1.0 — 2025-05-13

### ✨ Added

- ✅ **Import support** (`ImportState`) for all major resources:
  - `passbolt_user`
  - `passbolt_group`
  - `passbolt_folder`
  - `passbolt_folder_permission`
- 🛠️ Added `examples/resources/*/import.sh` to enable automatic `tfplugindocs` import section generation.

### 📝 Improved

- 📚 Expanded `Schema.Description` fields for all resources, improving Terraform Registry documentation.
- 🧼 Cleaned up Markdown formatting in generated docs.
- ✅ Aligned field descriptions with actual API behavior (e.g., `personal`, `first_name`, `folder_parent`).

---

➡️ This release adds full import support and improves documentation UX.  
It's backward compatible and safe to upgrade.

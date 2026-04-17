# Changelog

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

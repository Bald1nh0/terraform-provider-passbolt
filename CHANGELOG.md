# Changelog

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

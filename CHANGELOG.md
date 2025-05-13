# Changelog

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

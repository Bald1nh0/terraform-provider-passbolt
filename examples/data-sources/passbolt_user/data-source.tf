provider "passbolt" {
  base_url    = "https://passbolt.example.com"
  private_key = file("~/.secrets/passbolt_private_key.pem")
  passphrase  = "mysecurepassphrase"
}

# 🔍 Lookup an existing user by email (must be active in Passbolt)
data "passbolt_user" "lead" {
  username = "lead.dev@example.com"
}

# 👥 Create a group with this user as manager
resource "passbolt_group" "dev_team" {
  name     = "Development Team"
  managers = [data.passbolt_user.lead.id]
}

# 📁 Create folder and share with the group
resource "passbolt_folder" "shared" {
  name = "Dev Shared Secrets"
}

resource "passbolt_folder_permission" "dev_access" {
  folder_id  = passbolt_folder.shared.id
  group_name = passbolt_group.dev_team.name
  permission = "update"
}

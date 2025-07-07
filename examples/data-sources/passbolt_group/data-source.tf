# Lookup existing group by name
data "passbolt_group" "devops" {
  name = "DevOps"
}

# Store a shared password
resource "passbolt_password" "shared_secret" {
  name     = "Docker Registry Token"
  username = "ci-bot"
  uri      = "https://registry.example.com"
  password = "s3cr3t-value"

  # Share with the group found via data source
  share_groups = [data.passbolt_group.devops.id]
}

output "shared_password_id" {
  value = passbolt_password.shared_secret.id
}

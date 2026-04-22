# Lookup existing group by name
data "passbolt_group" "devops" {
  name = "DevOps"
}

variable "shared_secret_value" {
  description = "Secret shared with the looked-up Passbolt group."
  type        = string
  sensitive   = true
}

# Store a shared password
resource "passbolt_password" "shared_secret" {
  name                = "Docker Registry Token"
  username            = "ci-bot"
  uri                 = "https://registry.example.com"
  password_wo         = var.shared_secret_value
  password_wo_version = 1

  # Share with the group found via data source
  share_groups = [data.passbolt_group.devops.id]
}

output "shared_password_id" {
  value = passbolt_password.shared_secret.id
}

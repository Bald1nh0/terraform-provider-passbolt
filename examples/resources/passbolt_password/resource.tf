# Fetch the group by name (recommended over hardcoding)
data "passbolt_group" "devops" {
  name = "DevOps"
}

variable "terraform_admin_password" {
  description = "Password shared through Passbolt."
  type        = string
  sensitive   = true
}

resource "passbolt_password" "example" {
  name                = "Terraform Admin"
  description         = "Credential for Centrifugo admin"
  username            = "centrifugo-admin"
  password_wo         = var.terraform_admin_password
  password_wo_version = 1
  uri                 = "https://centrifugo.example.com"
  folder_parent       = "Terraform Folders"

  # Recommended: use share_groups for future compatibility
  share_groups = [data.passbolt_group.devops.id]

  # Deprecated: use `share_groups` instead of `share_group`
  # share_group = "DevOps"
}

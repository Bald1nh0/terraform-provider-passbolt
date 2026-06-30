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
}

resource "passbolt_password_permission" "devops_group_update" {
  resource_id = passbolt_password.example.id
  group_name  = "DevOps"
  permission  = "update"
}

resource "passbolt_password_permission" "operator_read" {
  resource_id = passbolt_password.example.id
  username    = "operator@example.com"
  permission  = "read"
}

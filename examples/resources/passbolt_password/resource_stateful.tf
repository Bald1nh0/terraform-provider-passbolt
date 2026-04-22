# Less secure legacy example:
# Terraform still stores the secret value in state even when the source variable is sensitive.
variable "terraform_admin_password_stateful" {
  description = "Password shared through the legacy stateful Passbolt flow."
  type        = string
  sensitive   = true
}

resource "passbolt_password" "example_stateful" {
  name          = "Terraform Admin Legacy"
  description   = "Legacy stateful credential example"
  username      = "centrifugo-admin"
  password      = var.terraform_admin_password_stateful
  uri           = "https://centrifugo.example.com"
  folder_parent = "Terraform Folders"
}

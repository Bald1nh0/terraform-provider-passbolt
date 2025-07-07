# Fetch the group by name (recommended over hardcoding)
data "passbolt_group" "devops" {
  name = "DevOps"
}

resource "passbolt_password" "example" {
  name          = "Terraform Admin"
  description   = "Credential for Centrifugo admin"
  username      = "centrifugo-admin"
  password      = "supersecret"
  uri           = "https://centrifugo.example.com"
  folder_parent = "Terraform Folders"

  # Recommended: use share_groups for future compatibility
  share_groups = [data.passbolt_group.devops.id]

  # Deprecated: use `share_groups` instead of `share_group`
  # share_group = "DevOps"
}

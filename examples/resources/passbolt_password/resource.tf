resource "passbolt_password" "example" {
  name          = "Terraform Admin"
  description   = "Credential for Centrifugo admin"
  username      = "centrifugo-admin"
  password      = "supersecret"
  uri           = "https://centrifugo.example.com"
  folder_parent = "Terraform Folders"
  share_group   = "DevOps"
}
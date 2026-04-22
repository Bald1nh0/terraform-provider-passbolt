data "passbolt_folders" "all" {}

output "all_folder_names" {
  value = [for f in data.passbolt_folders.all.folders : f.name]
}

output "all_folder_paths" {
  value = [for f in data.passbolt_folders.all.folders : f.path]
}

# Or use folder path to safely select a UUID in another resource/module:

variable "centrifugo_admin_password" {
  description = "Password stored in Passbolt."
  type        = string
  sensitive   = true
}

resource "passbolt_password" "example" {
  name                = "Centrifugo admin"
  username            = "centrifugo"
  password_wo         = var.centrifugo_admin_password
  password_wo_version = 1
  uri                 = "https://centrifugo.example.com"
  folder_parent       = one([for f in data.passbolt_folders.all.folders : f.id if f.path == "/Terraform Folders"])
}

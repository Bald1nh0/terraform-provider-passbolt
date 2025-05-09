data "passbolt_folders" "all" {}

output "all_folder_names" {
  value = [for f in data.passbolt_folders.all.folders : f.name]
}

# Or use folder info in another resource/module:

resource "passbolt_password" "example" {
  name          = "Centrifugo admin"
  username      = "centrifugo"
  password      = "secret"
  uri           = "https://centrifugo.example.com"
  folder_parent = one([for f in data.passbolt_folders.all.folders : f.name if f.name == "Terraform Folders"])
}
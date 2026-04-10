data "passbolt_folders" "all" {}

output "all_folder_names" {
  value = [for f in data.passbolt_folders.all.folders : f.name]
}

output "all_folder_paths" {
  value = [for f in data.passbolt_folders.all.folders : f.path]
}

# Or use folder path to safely select a UUID in another resource/module:

resource "passbolt_password" "example" {
  name          = "Centrifugo admin"
  username      = "centrifugo"
  password      = "secret"
  uri           = "https://centrifugo.example.com"
  folder_parent = one([for f in data.passbolt_folders.all.folders : f.id if f.path == "/Terraform Folders"])
}

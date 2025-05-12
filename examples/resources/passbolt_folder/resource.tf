resource "passbolt_folder" "example" {
  name = "Terraform Test Folder"
}

resource "passbolt_folder" "nested" {
  name          = "Nested Folder"
  folder_parent = passbolt_folder.example.name
}
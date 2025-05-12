resource "passbolt_folder" "example" {
  name          = "terraform_test_folder"
  folder_parent = ""
}

resource "passbolt_folder_permission" "example" {
  folder_id  = passbolt_folder.example.id
  group_name = "Developers"
  permission = "update"
}
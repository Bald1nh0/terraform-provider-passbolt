resource "passbolt_folder" "application_a" {
  name = "application_A"
}

resource "passbolt_folder" "application_a_prod" {
  name          = "prod"
  folder_parent = passbolt_folder.application_a.id
}

resource "passbolt_folder" "application_a_prod_sub_folder_3" {
  name          = "sub_folder_3"
  folder_parent = "/application_A/prod"
}

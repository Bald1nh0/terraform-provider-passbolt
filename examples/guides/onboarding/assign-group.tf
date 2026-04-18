data "passbolt_user" "platform_lead" {
  username = "platform.lead@example.com"
}

data "passbolt_user" "engineer" {
  username = "platform.engineer@example.com"
}

resource "passbolt_group" "platform" {
  name     = "Platform Team"
  managers = [data.passbolt_user.platform_lead.id]
  members  = [data.passbolt_user.engineer.id]
}

resource "passbolt_folder" "platform" {
  name = "Platform"
}

resource "passbolt_folder_permission" "platform_access" {
  folder_id  = passbolt_folder.platform.id
  group_name = passbolt_group.platform.name
  permission = "update"
}

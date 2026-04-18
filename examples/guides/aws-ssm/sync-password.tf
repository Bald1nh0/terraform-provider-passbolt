data "aws_ssm_parameter" "db_admin_password" {
  name            = "/applications/example/db_admin_password"
  with_decryption = true
}

data "passbolt_group" "platform" {
  name = "Platform Team"
}

resource "passbolt_password" "db_admin" {
  name          = "ExampleDatabaseAdmin"
  description   = "Administrative database credential synced from AWS SSM"
  username      = "db-admin"
  password      = data.aws_ssm_parameter.db_admin_password.value
  uri           = "postgres://db.example.internal:5432/app"
  folder_parent = "/Platform"
  share_groups  = [data.passbolt_group.platform.id]
}

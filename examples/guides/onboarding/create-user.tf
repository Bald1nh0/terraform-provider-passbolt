resource "passbolt_user" "engineer" {
  username   = "platform.engineer@example.com"
  first_name = "Platform"
  last_name  = "Engineer"
  role       = "user"
}

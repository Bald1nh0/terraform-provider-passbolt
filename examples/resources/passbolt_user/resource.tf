resource "passbolt_user" "test" {
  username   = "terraform@example.com"
  first_name = "Terraform"
  last_name  = "Test"
  role       = "user"
}

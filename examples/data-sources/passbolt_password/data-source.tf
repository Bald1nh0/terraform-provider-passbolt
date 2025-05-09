data "passbolt_password" "by_id" {
  id = "1234-5678-90ab-cdef-1234-5678-90ab-cdef"
}

output "passbolt_password_username" {
  value = data.passbolt_password.by_id.username
}

output "passbolt_password_value" {
  sensitive = true
  value     = data.passbolt_password.by_id.password
}

# Use this secret in another resource:

# resource "aws_db_instance" "main" {
#   ...
#   master_username = data.passbolt_password.by_id.username
#   master_password = data.passbolt_password.by_id.password
# }
provider "passbolt" {
  base_url    = "https://passbolt.example.com"
  private_key = file("private.pem")
  passphrase  = "supersecurepassphrase"
}

# Example active user already registered in Passbolt (must be active!)
# You can retrieve this UUID manually or via data source (future).
variable "manager_id" {
  description = "The UUID of an existing active Passbolt user"
  type        = string
}

variable "member_id" {
  description = "The UUID of an existing active Passbolt user to add as a regular group member"
  type        = string
}

resource "passbolt_group" "example" {
  name     = "Terraform Group Example"
  managers = [var.manager_id]
  members  = [var.member_id]
}

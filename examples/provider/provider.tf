# EXAMPLE: Basic provider configuration for a self-hosted Passbolt instance.
terraform {
  required_providers {
    passbolt = {
      source  = "Bald1nh0/passbolt"
      version = "~> 1.6"
    }
  }
}

variable "passbolt_passphrase" {
  description = "Passphrase for the Passbolt PGP private key."
  type        = string
  sensitive   = true
}

# You can also supply these values through PASSBOLT_URL, PASSBOLT_KEY, and PASSBOLT_PASS.
provider "passbolt" {
  base_url    = "https://passbolt.example.com/"
  private_key = file("${path.module}/passbolt-private.asc")
  passphrase  = var.passbolt_passphrase
}

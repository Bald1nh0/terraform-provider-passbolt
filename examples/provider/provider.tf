# EXAMPLE: Provider config with inline PGP private key (basic, for test/dev only)
terraform {
  required_providers {
    passbolt = {
      source  = "Bald1nh0/passbolt"
      version = "~> 1.0.1"
    }
  }
}
provider "passbolt" {
  base_url    = "https://passbolt.example.com/"
  private_key = <<EOT
-----BEGIN PGP PRIVATE KEY BLOCK-----

hdjkahjkdhjkawhkjdhjkhjkd
-----END PGP PRIVATE KEY BLOCK-----
EOT
  passphrase  = "example_passphrase"
}

# ----------------------------------------------------------------

# RECOMMENDED: Secure provider configuration — store private key in AWS SSM Parameter Store

terraform {
  required_providers {
    passbolt = {
      source  = "Bald1nh0/passbolt"
      version = "~> 1.0.1"
    }
  }
}
data "aws_ssm_parameter" "passbolt_private_key" {
  name            = "/passbolt/private_key"
  with_decryption = true
}

data "aws_ssm_parameter" "passbolt_private_key_passphrase" {
  name            = "/passbolt/private_key_passphrase"
  with_decryption = true
}

provider "passbolt" {
  base_url    = "https://passbolt.example.com/"
  private_key = data.aws_ssm_parameter.passbolt_private_key.value
  passphrase  = data.aws_ssm_parameter.passbolt_private_key_passphrase.value
}
---
page_title: "Authenticate the Passbolt Provider"
subcategory: "Getting Started"
description: |-
  Learn the recommended ways to authenticate the Passbolt Terraform provider for local development and CI/CD workflows.
---

# Authenticate the Passbolt Provider

The provider authenticates with a Passbolt user's PGP private key, its passphrase, and the Passbolt base URL.

## Local Development

Use a local file for the PGP private key and pass the passphrase via a sensitive variable:

```terraform
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
```

## Environment Variables

You can also supply credentials through environment variables:

```bash
export PASSBOLT_URL="https://passbolt.example.com/"
export PASSBOLT_KEY="$(cat ./passbolt-private.asc)"
export PASSBOLT_PASS="<PASSPHRASE>"
```

-> Environment variables are usually the simplest option for local shells, CI runners, and one-off automation jobs.

## CI/CD Recommendations

- Store the private key and passphrase outside the repository.
- Use CI secret storage, AWS SSM Parameter Store, or AWS Secrets Manager to inject credentials at runtime.
- Avoid committing inline secrets in Terraform configuration files.

## Next Steps

- See [Sync secrets from AWS SSM](aws-ssm.md) for a production-oriented workflow.
- See [Onboard users and groups](onboarding.md) for an identity management workflow.

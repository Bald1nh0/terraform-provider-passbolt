---
page_title: "Sync Secrets from AWS SSM"
subcategory: "Workflows"
description: |-
  Load provider credentials and secret values from AWS SSM Parameter Store, then publish them into Passbolt with Terraform.
---

# Sync Secrets from AWS SSM

AWS SSM Parameter Store is a practical way to keep the Passbolt provider credentials and application secrets out of the repository.

## Load Provider Credentials from SSM

```terraform
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
```

## Publish an Application Secret into Passbolt

```terraform
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
```

-> This pattern works well when a secret is owned outside Passbolt but should be shared with teams through Passbolt folders and groups.

## Operational Notes

- Keep the PGP private key and passphrase in separate secure parameters.
- Prefer sharing secrets through `share_groups` instead of hardcoding group UUIDs.
- Use folder names, folder UUIDs, or absolute folder paths for `folder_parent`.

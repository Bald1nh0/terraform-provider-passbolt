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

~> This classic `data "aws_ssm_parameter"` flow keeps credentials out of Git, but the decrypted values are still persisted in Terraform state because the AWS data source returns them.

## Publish an Application Secret into Passbolt

Use `password_wo` when you want to avoid storing the secret value in the `passbolt_password` resource state.

```terraform
data "aws_ssm_parameter" "db_admin_password" {
  name            = "/applications/example/db_admin_password"
  with_decryption = true
}

data "passbolt_group" "platform" {
  name = "Platform Team"
}

resource "passbolt_password" "db_admin" {
  name                = "ExampleDatabaseAdmin"
  description         = "Administrative database credential synced from AWS SSM"
  username            = "db-admin"
  password_wo         = data.aws_ssm_parameter.db_admin_password.value
  password_wo_version = 1
  uri                 = "postgres://db.example.internal:5432/app"
  folder_parent       = "/Platform"
  share_groups        = [data.passbolt_group.platform.id]
}
```

-> This pattern works well when a secret is owned outside Passbolt but should be shared with teams through Passbolt folders and groups.
!> `password_wo` keeps the secret out of the `passbolt_password` state entry, but the upstream `aws_ssm_parameter` data source still stores the fetched value in Terraform state. Use an ephemeral source or runtime secret injection if the full workflow must stay out of state.

## Operational Notes

- Keep the PGP private key and passphrase in separate secure parameters.
- Use `password_wo` and bump `password_wo_version` whenever you need to rotate a Passbolt secret without persisting it in Terraform state.
- Prefer sharing secrets through `share_groups` instead of hardcoding group UUIDs.
- Use folder names, folder UUIDs, or absolute folder paths for `folder_parent`.

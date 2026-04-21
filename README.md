# terraform-provider-passbolt

Terraform provider for [Passbolt](https://www.passbolt.com). Manage users, groups, folders, passwords, and folder permissions with Terraform.

Supports Passbolt CE/PRO, self-hosted deployments, AWS SSM-backed secret workflows, and Terraform-native onboarding/offboarding.

[![Terraform Registry](https://img.shields.io/badge/Terraform%20Registry-Bald1nh0%2Fpassbolt-623CE4?logo=terraform)](https://registry.terraform.io/providers/Bald1nh0/passbolt/latest/docs)
[![Latest Release](https://img.shields.io/github/v/release/Bald1nh0/terraform-provider-passbolt?display_name=tag)](https://github.com/Bald1nh0/terraform-provider-passbolt/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](./LICENCE)

- Registry Docs: <https://registry.terraform.io/providers/Bald1nh0/passbolt/latest/docs>
- Examples: [./examples](./examples)
- Templates: [./templates](./templates)
- Changelog: [./CHANGELOG.md](./CHANGELOG.md)

## Quick start

```hcl
terraform {
  required_providers {
    passbolt = {
      source  = "Bald1nh0/passbolt"
      version = "~> 1.5"
    }
  }
}

variable "passbolt_passphrase" {
  description = "Passphrase for the Passbolt PGP private key."
  type        = string
  sensitive   = true
}

provider "passbolt" {
  base_url    = "https://passbolt.example.com/"
  private_key = file("${path.module}/passbolt-private.asc")
  passphrase  = var.passbolt_passphrase
}
```

`base_url`, `private_key`, and `passphrase` can also be supplied through the `PASSBOLT_URL`, `PASSBOLT_KEY`, and `PASSBOLT_PASS` environment variables.

## Why this provider?

- Manage Passbolt users, groups, folders, passwords, and folder permissions with Terraform.
- Support Passbolt CE and PRO deployments with the same provider.
- Resolve `folder_parent` by unique folder name, folder UUID, or absolute path such as `/application_A/prod`.
- Use exact `data.passbolt_user` lookups that return only active, non-deleted users.
- Import major resources into Terraform state for gradual adoption.
- Integrate external secrets from AWS SSM Parameter Store or Secrets Manager.

## Supported resources and data sources

### Resources

- [`passbolt_user`](./docs/resources/user.md)
- [`passbolt_group`](./docs/resources/group.md)
- [`passbolt_folder`](./docs/resources/folder.md)
- [`passbolt_password`](./docs/resources/password.md)
- [`passbolt_folder_permission`](./docs/resources/folder_permission.md)

### Data sources

- [`passbolt_user`](./docs/data-sources/user.md)
- [`passbolt_group`](./docs/data-sources/group.md)
- [`passbolt_folders`](./docs/data-sources/folders.md)
- [`passbolt_password`](./docs/data-sources/password.md)

## Common use cases

- Onboard and offboard Passbolt users, roles, and groups with Terraform.
- Sync application secrets from AWS SSM Parameter Store into Passbolt.
- Manage shared folders and group-based access for DevOps or platform teams.

## Requirements

- Terraform 0.13+ (tested with 1.3+)
- Go 1.26.2+ (for building the provider)
- Passbolt server 3.0+ (self-hosted, tested on CE/PRO)

## Local development install

```sh
make build
make install
```

`make install` uses the `PLUGIN_NAMESPACE`, `VERSION`, `OS`, and `ARCH` values from the [Makefile](./Makefile). Adjust them if you need a custom local namespace or target platform.

## Provider configuration examples

### Basic configuration (local/dev use)

Best for quick tests and ephemeral environments.

```hcl
provider "passbolt" {
  base_url    = "https://passbolt.example.com/"
  private_key = <<EOT
...
EOT
  passphrase  = "example_passphrase"
}
```

### Recommended: load provider credentials from AWS SSM Parameter Store

Store your PGP private key in AWS SSM Parameter Store as a SecureString, then load it in Terraform:

```hcl
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

> _This protects your private key from leaking in state files or configs, improving security in production and CI/CD pipelines._

---

## Resource: passbolt_folder

Creates a new folder in Passbolt.

```hcl
resource "passbolt_folder" "application_a" {
  name = "application_A"
}

resource "passbolt_folder" "application_a_prod" {
  name          = "prod"
  folder_parent = passbolt_folder.application_a.id
}

resource "passbolt_folder" "application_a_prod_sub_folder_3" {
  name          = "sub_folder_3"
  folder_parent = "/application_A/prod"
}
```

`folder_parent` accepts a unique folder name, a folder UUID, or an absolute path such as `/application_A/prod`.

---

## Resource: passbolt_password

Creates a new password in Passbolt, can share with group.

```hcl
data "passbolt_group" "devops" {
  name = "DevOps"
}

resource "passbolt_password" "example" {
  name          = "centifugo_admin"
  username      = "admin"
  password      = "MY_SECRET"
  uri           = "https://centrifugo.example.com"
  folder_parent = passbolt_folder.application_a_prod.id
  share_groups  = [data.passbolt_group.devops.id]
}
```
### Example: Using secret from AWS SSM

You can pull a secret from AWS and inject into Passbolt using Terraform:

```hcl
data "aws_ssm_parameter" "centrifugo_admin_password" {
  name            = "/centrifugo/admin_password"
  with_decryption = true
}

data "passbolt_group" "backend" {
  name = "Backend"
}

resource "passbolt_password" "centrifugo_admin_password" {
  name          = "CentrifugoAdmin"
  password      = data.aws_ssm_parameter.centrifugo_admin_password.value
  username      = "no_need"
  uri           = "https://centrifugo-dev.example.com/"
  folder_parent = "Backend"
  share_groups  = [data.passbolt_group.backend.id]
}
```

---

## Resource: passbolt_folder_permission

Share a Passbolt folder with a group, can be managed independently.

```hcl
resource "passbolt_folder" "shared" {
  name = "Critical"
}

resource "passbolt_folder_permission" "critical_devops" {
  folder_id  = passbolt_folder.shared.id
  group_name = "DevOps"
  permission = "update" # can be "read", "update", "owner"
}
```

- **permission**: `"read"` = read-only, `"update"` = edit, `"owner"` = full/admin rights  
- To revoke sharing, remove the resource from your configuration.

You can use `data.passbolt_user` + `passbolt_group` to create dynamic sharing logic:

```hcl
data "passbolt_user" "dev_manager" {
  username = "dev.lead@example.com"
}

data "passbolt_user" "dev_member" {
  username = "dev.member@example.com"
}

resource "passbolt_group" "developers" {
  name     = "Developers"
  managers = [data.passbolt_user.dev_manager.id]
  members  = [data.passbolt_user.dev_member.id]
}
```

---

## Resource: passbolt_group

```hcl
resource "passbolt_group" "example" {
  name     = "Terraform Group Example"
  managers = ["2a61bc5d-bbbb-aaaa-cccc-123456789abc"] # Must be active Passbolt user UUID
  members  = ["3b72cd6e-cccc-bbbb-dddd-23456789abcd"] # Optional regular group members
}
```

Passbolt requires at least one group manager. Regular members can be managed with `members`, and a user must not be present in both `managers` and `members`.

Group memberships require existing active Passbolt users. A user created by `passbolt_user` may not be available for `passbolt_group` membership in the same Terraform apply; create and activate the user first, then reference it from `passbolt_group` in a later apply.

You can look up user UUIDs using `data "passbolt_user"` or manually fetch them from Passbolt.

---

## Data Source: passbolt_user

```hcl
data "passbolt_user" "admin" {
  username = "admin@example.com"
}

output "user_id" {
  value = data.passbolt_user.admin.id
}
```

- Looks up users by **exact** email match
- Returns only **active, non-deleted** Passbolt users
- Returns `user id`, `role`, `first_name`, and `last_name`
- Can be used to assign managers in `passbolt_group`, or resolve dependencies

## Data Source: passbolt_group

Look up a group by name, and get its ID.

```hcl
data "passbolt_group" "devops" {
  name = "DevOps"
}

output "group_id" {
  value = data.passbolt_group.devops.id
}
```

Can be used with share_groups in passbolt_password and passbolt_folder_permission.

## Data Source: passbolt_folders

Look up all folders, including their resolved absolute paths.

```hcl
data "passbolt_folders" "all" {}

output "all_folder_paths" {
  value = [for f in data.passbolt_folders.all.folders : f.path]
}
```

## Data Source: passbolt_password

Fetch a Passbolt secret by UUID and expose its metadata and sensitive password value.

```hcl
data "passbolt_password" "db_admin" {
  id = "12345678-90ab-cdef-1234-567890abcdef"
}

output "db_admin_username" {
  value = data.passbolt_password.db_admin.username
}

output "db_admin_password" {
  sensitive = true
  value     = data.passbolt_password.db_admin.password
}
```

## Development

- Build locally with `make build` and install with `make install`.
- Register new resources in `provider.go`.
- See `internal/provider/*` for implementation examples.
- PRs (with issue or description please!) are welcome.

## Testing

- Unit tests: `go test ./...`
- Acceptance tests: `TF_ACC=1 go test ./... -count=1 -v`
- Acceptance test environment:
  - `PASSBOLT_BASE_URL`
  - `PASSBOLT_PRIVATE_KEY`
  - `PASSBOLT_PASSPHRASE`
  - `PASSBOLT_MANAGER_ID`
  - `PASSBOLT_TEST_USER_EMAIL`
  - `PASSBOLT_TEST_EMAIL_DOMAIN` (optional; defaults to `example.com` for generated acceptance-test users)
  - `PASSBOLT_MEMBER_ID` (optional; must be different from `PASSBOLT_MANAGER_ID`)
  - `PASSBOLT_DECRYPT_TEST_USER_EMAIL` (optional; dedicated user for decrypt-after-group-update acceptance coverage)
  - `PASSBOLT_DECRYPT_TEST_USER_ID` (optional; dedicated user UUID for decrypt-after-group-update acceptance coverage)
  - `PASSBOLT_DECRYPT_TEST_USER_PRIVATE_KEY` (optional; dedicated user private key for decrypt-after-group-update acceptance coverage)
  - `PASSBOLT_DECRYPT_TEST_USER_PASSPHRASE` (optional; dedicated user passphrase for decrypt-after-group-update acceptance coverage)

---

## Documentation Generation

This provider uses [terraform-plugin-docs](https://github.com/hashicorp/terraform-plugin-docs) for automatic documentation generation.

**To generate or update documentation in the `docs/` directory:**

1. Ensure you have all dependencies (see Go requirements above):

    ```sh
    go install github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@latest
    ```

2. Add or update examples in `examples/` and documentation templates in `templates/` as needed.

3. Run the following command from the project root:

    ```sh
    make docs
    ```

4. Check the generated Markdown files in the `docs/` directory before commit and push.

After pushing to the main branch and releasing a new tag, the Terraform Registry will automatically import your latest documentation files.

---

**Pro-tip:**  
All schema `Description` values, Terraform examples in `examples/`, and custom templates in `templates/` contribute to the generated Registry documentation. Keep all three in sync so the published docs stay accurate.

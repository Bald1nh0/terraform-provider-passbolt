# terraform-provider-passbolt

A **Terraform provider** for [Passbolt](https://www.passbolt.com):  
Self-hosted open-source team password manager, with folder, sharing and SSM/automation support.

---

## Features

- Create, read, and delete folders in Passbolt PRO/CE
- Create and manage secrets/passwords, including sharing with Passbolt groups
- Grant and revoke group permission to folders (`passbolt_folder_permission` resource)
- Integration-ready with AWS SSM, Secrets Manager, and other tools
- Use with your own Passbolt instance (CE or PRO)
- Manage user groups via `passbolt_group` (with manager assignment)
- Look up users by email using `data "passbolt_user"`

---

## Requirements

- Terraform 0.13+ (tested with 1.3+)
- Go 1.19+ (for building the provider)
- Passbolt server 3.0+ (self-hosted, tested on CE/PRO)

---

## Installation (local/dev)

1. **Build the provider**  
   ```sh
   go build -o terraform-provider-passbolt_v1.0.0
   ```

2. **Install it into Terraform plugins directory:**  
   (Change path, binary name and version as needed)
   ```sh
   mkdir -p ~/.terraform.d/plugins/<your_namespace>/passbolt/1.0.0/linux_amd64/
   cp terraform-provider-passbolt_v1.0.0 ~/.terraform.d/plugins/<your_namespace>/passbolt/1.0.0/linux_amd64/
   chmod +x ~/.terraform.d/plugins/<your_namespace>/passbolt/1.0.0/linux_amd64/terraform-provider-passbolt_v1.0.0
   ```

3. **Configure the provider in your Terraform project:**
   ```hcl
   terraform {
     required_providers {
       passbolt = {
         source  = "<your_namespace>/passbolt"
         version = "1.0.0"
       }
     }
   }
   ```

---

## Basic provider configuration (for local/dev use)
Best for quick tests and ephemeral environments
```hcl
provider "passbolt" {
  base_url    = "https://passbolt.example.com/"
  private_key = <<EOT
...
EOT
  passphrase  = "example_passphrase"
}
```

## Recommended: Secure provider configuration using AWS SSM Parameter Store

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
resource "passbolt_folder" "example" {
  name = "Terraform Test Folder"
}
```

---

## Resource: passbolt_password

Creates a new password in Passbolt, can share with group.

```hcl
resource "passbolt_password" "example" {
  name         = "centifugo_admin"
  username     = "admin"
  password     = "MY_SECRET"
  uri          = "https://centrifugo.example.com"
  folder_parent = passbolt_folder.example.name
  share_group   = "DevOps"
}
```
### Example: Using secret from AWS SSM

You can pull a secret from AWS and inject into Passbolt using Terraform:

```hcl
data "aws_ssm_parameter" "centrifugo_admin_password" {
  name            = "/centrifugo/admin_password"
  with_decryption = true
}

resource "passbolt_password" "centrifugo_admin_password" {
  name         = "CentrifugoAdmin"
  password     = data.aws_ssm_parameter.centrifugo_admin_password.value
  username     = "no_need"
  uri          = "https://centrifugo-dev.example.com/"
  folder_parent = "Backend"
  share_group   = "Backend"
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

resource "passbolt_group" "developers" {
  name     = "Developers"
  managers = [data.passbolt_user.dev_manager.id]
}



---

## Resource: passbolt_group

```hcl
resource "passbolt_group" "example" {
  name     = "Terraform Group Example"
  managers = ["2a61bc5d-bbbb-aaaa-cccc-123456789abc"] # Must be active Passbolt user UUID
}
```

You can look up user UUIDs using data "passbolt_user" or manually fetch from Passbolt

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

- Returns `user id`, `role`, `first_name`, and `last_name`
- Can be used to assign managers in `passbolt_group`, or resolve dependencies

## Development

- Fork, edit, and build as above.
- Register new resources in `provider.go`.
- See `internal/provider/*` for implementation examples.
- PRs (with issue or description please!) are welcome.

---

## Documentation Generation

This provider uses [terraform-plugin-docs](https://github.com/hashicorp/terraform-plugin-docs) for automatic documentation generation.

**To generate or update documentation in the `docs/` directory:**

1. Ensure you have all dependencies (see Go requirements above):

    ```sh
    go get github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs
    ```

2. Add or update examples in `examples/` as needed.

3. Run the following command from the project root:

    ```sh
    cd tools
    go generate ./...
    ```

4. Check the generated Markdown files in the `docs/` directory before commit and push.

After pushing to the main branch and releasing a new tag, the Terraform Registry will automatically import your latest documentation files.

---

**Pro-tip:**  
All schema `Description` and code examples placed in the `examples/` directory will be automatically included in the Markdown docs. Always keep your examples and schema descriptions up to date for accurate public documentation.



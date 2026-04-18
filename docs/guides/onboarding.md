---
page_title: "Onboard Users and Groups"
subcategory: "Workflows"
description: |-
  Create Passbolt users, then attach active users to groups and shared folders in a Terraform-friendly onboarding workflow.
---

# Onboard Users and Groups

Passbolt user onboarding is usually a two-step workflow:

1. Create the user and let the recipient activate the invitation in Passbolt.
2. Reference the now-active user from `data.passbolt_user` and add them to groups or shared folders.

## Step 1: Create the User

```terraform
resource "passbolt_user" "engineer" {
  username   = "platform.engineer@example.com"
  first_name = "Platform"
  last_name  = "Engineer"
  role       = "user"
}
```

~> A newly created user may not be available for `passbolt_group` membership in the same apply. Wait until the invitation is accepted and the user is active.

## Step 2: Add the Active User to a Group and Shared Folder

```terraform
data "passbolt_user" "platform_lead" {
  username = "platform.lead@example.com"
}

data "passbolt_user" "engineer" {
  username = "platform.engineer@example.com"
}

resource "passbolt_group" "platform" {
  name     = "Platform Team"
  managers = [data.passbolt_user.platform_lead.id]
  members  = [data.passbolt_user.engineer.id]
}

resource "passbolt_folder" "platform" {
  name = "Platform"
}

resource "passbolt_folder_permission" "platform_access" {
  folder_id  = passbolt_folder.platform.id
  group_name = passbolt_group.platform.name
  permission = "update"
}
```

## Why Use `data.passbolt_user`?

- It performs an exact username lookup.
- It returns only active, non-deleted users.
- It avoids hardcoding UUIDs in Terraform configuration.

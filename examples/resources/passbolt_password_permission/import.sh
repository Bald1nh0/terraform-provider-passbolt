# Password group permission can be imported using resource_id:group:group_name
terraform import passbolt_password_permission.devops_group_update 1111aaaa-2222-bbbb-3333-cccc4444dddd:group:DevOps

# Password user permission can be imported using resource_id:user:username
terraform import passbolt_password_permission.operator_read 1111aaaa-2222-bbbb-3333-cccc4444dddd:user:operator@example.com

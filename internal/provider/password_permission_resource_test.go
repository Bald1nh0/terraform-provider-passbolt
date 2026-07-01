package provider_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/passbolt/go-passbolt/api"
	"github.com/passbolt/go-passbolt/helper"
)

func TestAccPasswordPermissionResource_group(t *testing.T) {
	t.Parallel()

	requireAcceptanceEnv(t, "PASSBOLT_BASE_URL", "PASSBOLT_PRIVATE_KEY", "PASSBOLT_PASSPHRASE", "PASSBOLT_MANAGER_ID")

	baseURL := os.Getenv("PASSBOLT_BASE_URL")
	privateKey := os.Getenv("PASSBOLT_PRIVATE_KEY")
	passphrase := os.Getenv("PASSBOLT_PASSPHRASE")
	managerID := os.Getenv("PASSBOLT_MANAGER_ID")
	suffix := testAccSuffix()
	passwordName := testAccName("acc-password-permission-group", suffix)
	groupName := testAccName("acc-password-permission-group", suffix)
	configRead := testPasswordGroupPermissionConfig(
		baseURL,
		privateKey,
		passphrase,
		managerID,
		passwordName,
		groupName,
		"read",
	)
	configOwner := testPasswordGroupPermissionConfig(
		baseURL,
		privateKey,
		passphrase,
		managerID,
		passwordName,
		groupName,
		"owner",
	)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProviderFactories,
		Steps: testPasswordGroupPermissionSteps(
			baseURL,
			privateKey,
			passphrase,
			groupName,
			configRead,
			configOwner,
		),
	})
}

func TestAccPasswordPermissionResource_user(t *testing.T) {
	t.Parallel()

	requireAcceptanceEnv(
		t,
		"PASSBOLT_BASE_URL",
		"PASSBOLT_PRIVATE_KEY",
		"PASSBOLT_PASSPHRASE",
		"PASSBOLT_TEST_USER_EMAIL",
	)

	baseURL := os.Getenv("PASSBOLT_BASE_URL")
	privateKey := os.Getenv("PASSBOLT_PRIVATE_KEY")
	passphrase := os.Getenv("PASSBOLT_PASSPHRASE")
	username := os.Getenv("PASSBOLT_TEST_USER_EMAIL")
	userID := testAccPasswordPermissionUserID(t, baseURL, privateKey, passphrase, username)
	suffix := testAccSuffix()
	passwordName := testAccName("acc-password-permission-user", suffix)
	configRead := testPasswordUserPermissionConfig(baseURL, privateKey, passphrase, passwordName, username, "read")
	configUpdate := testPasswordUserPermissionConfig(baseURL, privateKey, passphrase, passwordName, username, "update")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProviderFactories,
		Steps: testPasswordUserPermissionSteps(
			baseURL,
			privateKey,
			passphrase,
			username,
			userID,
			configRead,
			configUpdate,
		),
	})
}

func TestAccPasswordPermissionResource_missingPasswordRemovesState(t *testing.T) {
	t.Parallel()

	requireAcceptanceEnv(t, "PASSBOLT_BASE_URL", "PASSBOLT_PRIVATE_KEY", "PASSBOLT_PASSPHRASE", "PASSBOLT_MANAGER_ID")

	baseURL := os.Getenv("PASSBOLT_BASE_URL")
	privateKey := os.Getenv("PASSBOLT_PRIVATE_KEY")
	passphrase := os.Getenv("PASSBOLT_PASSPHRASE")
	managerID := os.Getenv("PASSBOLT_MANAGER_ID")
	suffix := testAccSuffix()
	passwordName := testAccName("acc-password-permission-missing", suffix)
	groupName := testAccName("acc-password-permission-missing", suffix)
	resourceID := testCreatePassboltPasswordOutOfBand(t, baseURL, privateKey, passphrase, passwordName)
	config := testExternalPasswordGroupPermissionConfig(
		baseURL,
		privateKey,
		passphrase,
		managerID,
		groupName,
		resourceID,
		"read",
	)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			testStepExternalPasswordPermissionRead(
				baseURL,
				privateKey,
				passphrase,
				groupName,
				resourceID,
				config,
			),
			{
				PreConfig: func() {
					testDeletePassboltResource(t, baseURL, privateKey, passphrase, resourceID)
				},
				Config: testPassboltProviderConfig(baseURL, privateKey, passphrase),
			},
		},
	})
}

func testStepExternalPasswordPermissionRead(
	baseURL,
	privateKey,
	passphrase,
	groupName,
	resourceID,
	config string,
) resource.TestStep {
	return resource.TestStep{
		Config: config,
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_password_permission.group", "resource_id", resourceID),
			resource.TestCheckResourceAttr("passbolt_password_permission.group", "group_name", groupName),
			resource.TestCheckResourceAttr("passbolt_password_permission.group", "permission", "read"),
			testCheckPasswordPermissionTypeForResourceID(
				baseURL,
				privateKey,
				passphrase,
				resourceID,
				"passbolt_group.target",
				"Group",
				1,
			),
		),
	}
}

func testPasswordGroupPermissionSteps(
	baseURL,
	privateKey,
	passphrase,
	groupName,
	configRead,
	configOwner string,
) []resource.TestStep {
	return []resource.TestStep{
		testStepPasswordGroupPermissionRead(baseURL, privateKey, passphrase, groupName, configRead),
		testStepPasswordGroupPermissionOwner(baseURL, privateKey, passphrase, configOwner),
		testStepImportPasswordGroupPermission(),
		{
			Config:   configOwner,
			PlanOnly: true,
		},
	}
}

func testStepPasswordGroupPermissionRead(baseURL, privateKey, passphrase, groupName, config string) resource.TestStep {
	return resource.TestStep{
		Config: config,
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttrPair(
				"passbolt_password_permission.group",
				"resource_id",
				"passbolt_password.example",
				"id",
			),
			resource.TestCheckResourceAttr("passbolt_password_permission.group", "group_name", groupName),
			resource.TestCheckResourceAttr("passbolt_password_permission.group", "permission", "read"),
			testCheckPasswordPermissionTypeFromState(
				baseURL,
				privateKey,
				passphrase,
				"passbolt_password.example",
				"passbolt_group.target",
				"Group",
				1,
			),
		),
	}
}

func testStepPasswordGroupPermissionOwner(baseURL, privateKey, passphrase, config string) resource.TestStep {
	return resource.TestStep{
		Config: config,
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_password_permission.group", "permission", "owner"),
			testCheckPasswordPermissionTypeFromState(
				baseURL,
				privateKey,
				passphrase,
				"passbolt_password.example",
				"passbolt_group.target",
				"Group",
				15,
			),
		),
	}
}

func testStepImportPasswordGroupPermission() resource.TestStep {
	return resource.TestStep{
		ResourceName:      "passbolt_password_permission.group",
		ImportState:       true,
		ImportStateVerify: true,
	}
}

func testPasswordUserPermissionSteps(
	baseURL,
	privateKey,
	passphrase,
	username,
	userID,
	configRead,
	configUpdate string,
) []resource.TestStep {
	return []resource.TestStep{
		testStepPasswordUserPermissionRead(baseURL, privateKey, passphrase, username, userID, configRead),
		testStepPasswordUserPermissionUpdate(baseURL, privateKey, passphrase, userID, configUpdate),
		testStepImportPasswordUserPermission(),
		{
			Config:   configUpdate,
			PlanOnly: true,
		},
	}
}

func testStepPasswordUserPermissionRead(
	baseURL,
	privateKey,
	passphrase,
	username,
	userID,
	config string,
) resource.TestStep {
	return resource.TestStep{
		Config: config,
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttrPair(
				"passbolt_password_permission.user",
				"resource_id",
				"passbolt_password.example",
				"id",
			),
			resource.TestCheckResourceAttr("passbolt_password_permission.user", "username", username),
			resource.TestCheckResourceAttr("passbolt_password_permission.user", "permission", "read"),
			testCheckPasswordPermissionType(baseURL, privateKey, passphrase, "passbolt_password.example", userID, "User", 1),
		),
	}
}

func testStepPasswordUserPermissionUpdate(
	baseURL,
	privateKey,
	passphrase,
	userID,
	config string,
) resource.TestStep {
	return resource.TestStep{
		Config: config,
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_password_permission.user", "permission", "update"),
			testCheckPasswordPermissionType(baseURL, privateKey, passphrase, "passbolt_password.example", userID, "User", 7),
		),
	}
}

func testStepImportPasswordUserPermission() resource.TestStep {
	return resource.TestStep{
		ResourceName:      "passbolt_password_permission.user",
		ImportState:       true,
		ImportStateVerify: true,
	}
}

func testPasswordGroupPermissionConfig(
	baseURL,
	privateKey,
	passphrase,
	managerID,
	passwordName,
	groupName,
	permission string,
) string {
	return fmt.Sprintf(`
provider "passbolt" {
  base_url    = "%s"
  private_key = <<EOF
%s
EOF
  passphrase  = "%s"
}

resource "passbolt_group" "target" {
  name     = "%s"
  managers = ["%s"]
}

resource "passbolt_password" "example" {
  name     = "%s"
  username = "password-permission-group-user"
  uri      = "https://password-permission-group.example.com"
  password = "password-permission-group-secret"
}

resource "passbolt_password_permission" "group" {
  resource_id = passbolt_password.example.id
  group_name  = passbolt_group.target.name
  permission  = "%s"
}
	`, baseURL, privateKey, passphrase, groupName, managerID, passwordName, permission)
}

func testExternalPasswordGroupPermissionConfig(
	baseURL,
	privateKey,
	passphrase,
	managerID,
	groupName,
	resourceID,
	permission string,
) string {
	return fmt.Sprintf(`
provider "passbolt" {
  base_url    = "%s"
  private_key = <<EOF
%s
EOF
  passphrase  = "%s"
}

resource "passbolt_group" "target" {
  name     = "%s"
  managers = ["%s"]
}

resource "passbolt_password_permission" "group" {
  resource_id = "%s"
  group_name  = passbolt_group.target.name
  permission  = "%s"
}
`, baseURL, privateKey, passphrase, groupName, managerID, resourceID, permission)
}

func testPasswordUserPermissionConfig(
	baseURL,
	privateKey,
	passphrase,
	passwordName,
	username,
	permission string,
) string {
	return fmt.Sprintf(`
provider "passbolt" {
  base_url    = "%s"
  private_key = <<EOF
%s
EOF
  passphrase  = "%s"
}

resource "passbolt_password" "example" {
  name     = "%s"
  username = "password-permission-user"
  uri      = "https://password-permission-user.example.com"
  password = "password-permission-user-secret"
}

resource "passbolt_password_permission" "user" {
  resource_id = passbolt_password.example.id
  username    = "%s"
  permission  = "%s"
}
`, baseURL, privateKey, passphrase, passwordName, username, permission)
}

func testCheckPasswordPermissionTypeFromState(
	baseURL,
	privateKey,
	passphrase,
	passwordResourceAddress,
	targetResourceAddress,
	aro string,
	expectedType int,
) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		target, ok := state.RootModule().Resources[targetResourceAddress]
		if !ok {
			return fmt.Errorf("%s not found in Terraform state", targetResourceAddress)
		}

		return testCheckPasswordPermissionType(
			baseURL,
			privateKey,
			passphrase,
			passwordResourceAddress,
			target.Primary.ID,
			aro,
			expectedType,
		)(state)
	}
}

func testCheckPasswordPermissionTypeForResourceID(
	baseURL,
	privateKey,
	passphrase,
	resourceID,
	targetResourceAddress,
	aro string,
	expectedType int,
) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		target, ok := state.RootModule().Resources[targetResourceAddress]
		if !ok {
			return fmt.Errorf("%s not found in Terraform state", targetResourceAddress)
		}

		return checkPasswordPermissionType(
			baseURL,
			privateKey,
			passphrase,
			resourceID,
			target.Primary.ID,
			aro,
			expectedType,
		)
	}
}

func testCheckPasswordPermissionType(
	baseURL,
	privateKey,
	passphrase,
	passwordResourceAddress,
	targetID,
	aro string,
	expectedType int,
) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		password, ok := state.RootModule().Resources[passwordResourceAddress]
		if !ok {
			return fmt.Errorf("%s not found in Terraform state", passwordResourceAddress)
		}

		resourceID := password.Primary.ID
		if resourceID == "" {
			return fmt.Errorf("%s id is empty", passwordResourceAddress)
		}

		return checkPasswordPermissionType(baseURL, privateKey, passphrase, resourceID, targetID, aro, expectedType)
	}
}

func checkPasswordPermissionType(
	baseURL,
	privateKey,
	passphrase,
	resourceID,
	targetID,
	aro string,
	expectedType int,
) error {
	ctx := context.Background()
	client, err := api.NewClient(nil, "", baseURL, privateKey, passphrase)
	if err != nil {
		return fmt.Errorf("failed to create Passbolt API client: %w", err)
	}
	if err := client.Login(ctx); err != nil {
		return fmt.Errorf("failed to log in to Passbolt API: %w", err)
	}
	defer func() {
		_ = client.Logout(ctx)
	}()

	permissions, err := client.GetResourcePermissions(ctx, resourceID)
	if err != nil {
		return fmt.Errorf("failed to get resource permissions: %w", err)
	}

	for _, permission := range permissions {
		if permission.ARO != aro || permission.AROForeignKey != targetID {
			continue
		}

		if permission.Type != expectedType {
			return fmt.Errorf(
				"expected %s permission for %s to be type %d, got %d",
				aro,
				targetID,
				expectedType,
				permission.Type,
			)
		}

		return nil
	}

	return fmt.Errorf("expected %s permission for %s on resource %s", aro, targetID, resourceID)
}

func testCreatePassboltPasswordOutOfBand(t *testing.T, baseURL, privateKey, passphrase, name string) string {
	t.Helper()

	ctx := context.Background()
	client, err := api.NewClient(nil, "", baseURL, privateKey, passphrase)
	if err != nil {
		t.Fatalf("failed to create Passbolt API client: %v", err)
	}
	if err := client.Login(ctx); err != nil {
		t.Fatalf("failed to log in to Passbolt API: %v", err)
	}
	defer func() {
		_ = client.Logout(ctx)
	}()

	resourceID, err := helper.CreateResource(
		ctx,
		client,
		"",
		name,
		"out-of-band-user",
		"https://out-of-band-password.example.com",
		"out-of-band-secret",
		"",
	)
	if err != nil {
		t.Fatalf("failed to create Passbolt resource out-of-band: %v", err)
	}

	t.Cleanup(func() {
		testDeletePassboltResourceIfExists(baseURL, privateKey, passphrase, resourceID)
	})

	return resourceID
}

func testDeletePassboltResource(t *testing.T, baseURL, privateKey, passphrase, resourceID string) {
	t.Helper()

	if err := deletePassboltResource(baseURL, privateKey, passphrase, resourceID); err != nil {
		t.Fatalf("failed to delete Passbolt resource %s out-of-band: %v", resourceID, err)
	}
}

func testDeletePassboltResourceIfExists(baseURL, privateKey, passphrase, resourceID string) {
	_ = deletePassboltResource(baseURL, privateKey, passphrase, resourceID)
}

func deletePassboltResource(baseURL, privateKey, passphrase, resourceID string) error {
	ctx := context.Background()
	client, err := api.NewClient(nil, "", baseURL, privateKey, passphrase)
	if err != nil {
		return fmt.Errorf("failed to create Passbolt API client: %w", err)
	}
	if err := client.Login(ctx); err != nil {
		return fmt.Errorf("failed to log in to Passbolt API: %w", err)
	}
	defer func() {
		_ = client.Logout(ctx)
	}()

	return client.DeleteResource(ctx, resourceID)
}

func testAccPasswordPermissionUserID(t *testing.T, baseURL, privateKey, passphrase, username string) string {
	t.Helper()

	currentUserID := testAccCurrentUserID(t, baseURL, privateKey, passphrase)
	ctx := context.Background()
	client, err := api.NewClient(nil, "", baseURL, privateKey, passphrase)
	if err != nil {
		t.Fatalf("failed to create Passbolt API client: %v", err)
	}
	if err := client.Login(ctx); err != nil {
		t.Fatalf("failed to log in to Passbolt API: %v", err)
	}
	defer func() {
		_ = client.Logout(ctx)
	}()

	users, err := client.GetUsers(ctx, &api.GetUsersOptions{
		FilterSearch: username,
	})
	if err != nil {
		t.Fatalf("failed to look up password permission test user %q: %v", username, err)
	}

	for _, user := range users {
		if !strings.EqualFold(user.Username, username) {
			continue
		}
		if user.Deleted {
			t.Skipf("PASSBOLT_TEST_USER_EMAIL %q is deleted", username)
		}
		if !user.Active {
			t.Skipf("PASSBOLT_TEST_USER_EMAIL %q is not active", username)
		}
		if user.ID == currentUserID {
			t.Skip("PASSBOLT_TEST_USER_EMAIL resolves to the provider user")
		}

		return user.ID
	}

	t.Skipf("PASSBOLT_TEST_USER_EMAIL %q did not match an active user", username)

	return ""
}

package provider_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/passbolt/go-passbolt/api"
)

func TestAccPassboltGroup_fullLifecycle(t *testing.T) {
	t.Parallel()

	requireAcceptanceEnv(t, "PASSBOLT_BASE_URL", "PASSBOLT_PRIVATE_KEY", "PASSBOLT_PASSPHRASE", "PASSBOLT_MANAGER_ID")

	baseURL := os.Getenv("PASSBOLT_BASE_URL")
	privateKey := os.Getenv("PASSBOLT_PRIVATE_KEY")
	passphrase := os.Getenv("PASSBOLT_PASSPHRASE")
	managerID := os.Getenv("PASSBOLT_MANAGER_ID")
	suffix := testAccSuffix()
	groupName := testAccName("test-group", suffix)
	updatedGroupName := testAccName("renamed-group", suffix)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			testStepCreateGroup(baseURL, privateKey, passphrase, managerID, groupName),
			testStepNoDriftGroup(baseURL, privateKey, passphrase, managerID, groupName),
			testStepUpdateGroup(baseURL, privateKey, passphrase, managerID, updatedGroupName),
		},
	})
}

func TestAccPassboltGroup_withMembers(t *testing.T) {
	t.Parallel()

	requireAcceptanceEnv(
		t,
		"PASSBOLT_BASE_URL",
		"PASSBOLT_PRIVATE_KEY",
		"PASSBOLT_PASSPHRASE",
		"PASSBOLT_MANAGER_ID",
	)

	baseURL := os.Getenv("PASSBOLT_BASE_URL")
	privateKey := os.Getenv("PASSBOLT_PRIVATE_KEY")
	passphrase := os.Getenv("PASSBOLT_PASSPHRASE")
	managerID := os.Getenv("PASSBOLT_MANAGER_ID")
	memberID := testAccGroupMemberID(t, baseURL, privateKey, passphrase, managerID)
	suffix := testAccSuffix()
	groupName := testAccName("test-group-members", suffix)
	updatedGroupName := testAccName("renamed-group-members", suffix)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			testStepCreateGroupWithMembers(baseURL, privateKey, passphrase, managerID, memberID, groupName),
			testStepNoDriftGroupWithMembers(baseURL, privateKey, passphrase, managerID, memberID, groupName),
			testStepUpdateGroupWithMembers(baseURL, privateKey, passphrase, managerID, memberID, updatedGroupName),
		},
	})
}

func testAccGroupMemberID(t *testing.T, baseURL, privateKey, passphrase, managerID string) string {
	t.Helper()

	memberID := os.Getenv("PASSBOLT_MEMBER_ID")
	if memberID != "" {
		if memberID == managerID {
			t.Skip("PASSBOLT_MEMBER_ID must be different from PASSBOLT_MANAGER_ID")
		}

		return memberID
	}

	memberEmail := os.Getenv("PASSBOLT_TEST_USER_EMAIL")
	if memberEmail == "" {
		t.Skip("PASSBOLT_MEMBER_ID or PASSBOLT_TEST_USER_EMAIL is required for group member acceptance tests")
	}

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
		FilterSearch: memberEmail,
	})
	if err != nil {
		t.Fatalf("failed to look up member test user %q: %v", memberEmail, err)
	}
	if len(users) == 0 {
		t.Skipf("PASSBOLT_TEST_USER_EMAIL %q did not match any user", memberEmail)
	}

	memberID = users[0].ID
	if memberID == managerID {
		t.Skip("PASSBOLT_TEST_USER_EMAIL resolves to PASSBOLT_MANAGER_ID; skipping group member acceptance test")
	}

	return memberID
}

func testStepCreateGroup(baseURL, privateKey, passphrase, managerID, groupName string) resource.TestStep {
	return resource.TestStep{
		Config: testGroupConfig(baseURL, privateKey, passphrase, managerID, groupName),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_group.test", "name", groupName),
			resource.TestCheckResourceAttr("passbolt_group.test", "managers.#", "1"),
		),
	}
}

func testStepNoDriftGroup(baseURL, privateKey, passphrase, managerID, groupName string) resource.TestStep {
	return resource.TestStep{
		Config: testGroupConfig(baseURL, privateKey, passphrase, managerID, groupName),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_group.test", "name", groupName),
			resource.TestCheckResourceAttr("passbolt_group.test", "managers.#", "1"),
		),
	}
}

func testStepUpdateGroup(baseURL, privateKey, passphrase, managerID, groupName string) resource.TestStep {
	return resource.TestStep{
		Config: testGroupConfig(baseURL, privateKey, passphrase, managerID, groupName),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_group.test", "name", groupName),
			resource.TestCheckResourceAttr("passbolt_group.test", "managers.#", "1"),
		),
	}
}

func testStepCreateGroupWithMembers(
	baseURL,
	privateKey,
	passphrase,
	managerID,
	memberID,
	groupName string,
) resource.TestStep {
	return resource.TestStep{
		Config: testGroupWithMembersConfig(baseURL, privateKey, passphrase, managerID, memberID, groupName),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_group.test", "name", groupName),
			resource.TestCheckResourceAttr("passbolt_group.test", "managers.#", "1"),
			resource.TestCheckResourceAttr("passbolt_group.test", "members.#", "1"),
		),
	}
}

func testStepNoDriftGroupWithMembers(
	baseURL,
	privateKey,
	passphrase,
	managerID,
	memberID,
	groupName string,
) resource.TestStep {
	return resource.TestStep{
		Config: testGroupWithMembersConfig(baseURL, privateKey, passphrase, managerID, memberID, groupName),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_group.test", "name", groupName),
			resource.TestCheckResourceAttr("passbolt_group.test", "managers.#", "1"),
			resource.TestCheckResourceAttr("passbolt_group.test", "members.#", "1"),
		),
	}
}

func testStepUpdateGroupWithMembers(
	baseURL,
	privateKey,
	passphrase,
	managerID,
	memberID,
	groupName string,
) resource.TestStep {
	return resource.TestStep{
		Config: testGroupWithMembersConfig(baseURL, privateKey, passphrase, managerID, memberID, groupName),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_group.test", "name", groupName),
			resource.TestCheckResourceAttr("passbolt_group.test", "managers.#", "1"),
			resource.TestCheckResourceAttr("passbolt_group.test", "members.#", "1"),
		),
	}
}

func testGroupConfig(baseURL, privateKey, passphrase, managerID, groupName string) string {
	return fmt.Sprintf(`
provider "passbolt" {
  base_url    = "%s"
  private_key = <<EOF
%s
EOF
  passphrase  = "%s"
}

resource "passbolt_group" "test" {
  name     = "%s"
  managers = ["%s"]
}
`, baseURL, privateKey, passphrase, groupName, managerID)
}

func testGroupWithMembersConfig(baseURL, privateKey, passphrase, managerID, memberID, groupName string) string {
	return fmt.Sprintf(`
provider "passbolt" {
  base_url    = "%s"
  private_key = <<EOF
%s
EOF
  passphrase  = "%s"
}

resource "passbolt_group" "test" {
  name     = "%s"
  managers = ["%s"]
  members  = ["%s"]
}
`, baseURL, privateKey, passphrase, groupName, managerID, memberID)
}

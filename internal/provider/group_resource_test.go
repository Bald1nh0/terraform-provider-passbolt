package provider_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/passbolt/go-passbolt/api"
	"github.com/passbolt/go-passbolt/helper"
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

func TestAccPassboltGroup_addMemberToSharedGroup(t *testing.T) {
	t.Parallel()

	requireAcceptanceEnv(
		t,
		"PASSBOLT_BASE_URL",
		"PASSBOLT_PRIVATE_KEY",
		"PASSBOLT_PASSPHRASE",
	)

	baseURL := os.Getenv("PASSBOLT_BASE_URL")
	privateKey := os.Getenv("PASSBOLT_PRIVATE_KEY")
	passphrase := os.Getenv("PASSBOLT_PASSPHRASE")
	managerID := testAccCurrentUserID(t, baseURL, privateKey, passphrase)
	memberID := testAccGroupMemberID(t, baseURL, privateKey, passphrase, managerID)
	suffix := testAccSuffix()
	groupName := testAccName("test-group-shared", suffix)
	passwordName := testAccName("test-password-shared", suffix)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			testStepCreateSharedGroupPassword(baseURL, privateKey, passphrase, managerID, groupName, passwordName),
			testStepAddMemberToSharedGroup(baseURL, privateKey, passphrase, managerID, memberID, groupName, passwordName),
			testStepNoDriftSharedGroupMember(baseURL, privateKey, passphrase, managerID, memberID, groupName, passwordName),
		},
	})
}

func TestAccPassboltGroup_addMemberToSharedGroupCanDecryptSecret(t *testing.T) {
	t.Parallel()

	requireAcceptanceEnv(
		t,
		"PASSBOLT_BASE_URL",
		"PASSBOLT_PRIVATE_KEY",
		"PASSBOLT_PASSPHRASE",
	)

	baseURL := os.Getenv("PASSBOLT_BASE_URL")
	privateKey := os.Getenv("PASSBOLT_PRIVATE_KEY")
	passphrase := os.Getenv("PASSBOLT_PASSPHRASE")
	memberPrivateKey := testAccDecryptUserPrivateKey(t)
	memberPassphrase := testAccDecryptUserPassphrase(t)
	managerID := testAccCurrentUserID(t, baseURL, privateKey, passphrase)
	memberID := testAccDecryptGroupMemberID(t, baseURL, privateKey, passphrase, managerID)
	suffix := testAccSuffix()
	groupName := testAccName("test-group-shared-decrypt", suffix)
	passwordName := testAccName("test-password-shared-decrypt", suffix)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			testStepCreateSharedGroupPassword(baseURL, privateKey, passphrase, managerID, groupName, passwordName),
			testStepAddMemberToSharedGroupAndDecrypt(
				baseURL,
				privateKey,
				passphrase,
				memberPrivateKey,
				memberPassphrase,
				managerID,
				memberID,
				groupName,
				passwordName,
			),
		},
	})
}

func TestAccPassboltGroup_ignoreInactiveMembers(t *testing.T) {
	t.Parallel()

	requireAcceptanceEnv(
		t,
		"PASSBOLT_BASE_URL",
		"PASSBOLT_PRIVATE_KEY",
		"PASSBOLT_PASSPHRASE",
	)

	baseURL := os.Getenv("PASSBOLT_BASE_URL")
	privateKey := os.Getenv("PASSBOLT_PRIVATE_KEY")
	passphrase := os.Getenv("PASSBOLT_PASSPHRASE")
	managerID := testAccCurrentUserID(t, baseURL, privateKey, passphrase)
	suffix := testAccSuffix()
	groupName := testAccName("test-group-ignore-inactive", suffix)
	email := testAccEmail("inactive.member", suffix)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testGroupIgnoreInactiveMembersConfig(
					baseURL,
					privateKey,
					passphrase,
					managerID,
					email,
					groupName,
				),
				ExpectNonEmptyPlan: true,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("passbolt_group.test", "name", groupName),
					resource.TestCheckResourceAttr("passbolt_group.test", "managers.#", "1"),
					resource.TestCheckResourceAttr("passbolt_group.test", "ignore_inactive_members", "true"),
					resource.TestCheckResourceAttr("passbolt_user.member", "username", email),
					testCheckGroupDoesNotContainMember(
						baseURL,
						privateKey,
						passphrase,
						"passbolt_group.test",
						"passbolt_user.member",
					),
				),
			},
			{
				Config: testGroupIgnoreInactiveMembersConfig(
					baseURL,
					privateKey,
					passphrase,
					managerID,
					email,
					groupName,
				),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
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

func testAccDecryptGroupMemberID(t *testing.T, baseURL, privateKey, passphrase, managerID string) string {
	t.Helper()

	memberID := os.Getenv("PASSBOLT_DECRYPT_TEST_USER_ID")
	if memberID != "" {
		if memberID == managerID {
			t.Skip("PASSBOLT_DECRYPT_TEST_USER_ID must be different from PASSBOLT_MANAGER_ID")
		}

		return memberID
	}

	memberEmail := os.Getenv("PASSBOLT_DECRYPT_TEST_USER_EMAIL")
	if memberEmail == "" {
		return testAccGroupMemberID(t, baseURL, privateKey, passphrase, managerID)
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
		t.Fatalf("failed to look up decrypt test user %q: %v", memberEmail, err)
	}
	if len(users) == 0 {
		t.Skipf("PASSBOLT_DECRYPT_TEST_USER_EMAIL %q did not match any user", memberEmail)
	}

	memberID = users[0].ID
	if memberID == managerID {
		t.Skip("PASSBOLT_DECRYPT_TEST_USER_EMAIL resolves to PASSBOLT_MANAGER_ID; skipping decrypt acceptance test")
	}

	return memberID
}

func testAccDecryptUserPrivateKey(t *testing.T) string {
	t.Helper()

	privateKey := os.Getenv("PASSBOLT_DECRYPT_TEST_USER_PRIVATE_KEY")
	if privateKey != "" {
		return privateKey
	}

	privateKey = os.Getenv("PASSBOLT_MEMBER_PRIVATE_KEY")
	if privateKey == "" {
		t.Skip(
			"PASSBOLT_DECRYPT_TEST_USER_PRIVATE_KEY or PASSBOLT_MEMBER_PRIVATE_KEY is required for decrypt acceptance tests",
		)
	}

	return privateKey
}

func testAccDecryptUserPassphrase(t *testing.T) string {
	t.Helper()

	passphrase := os.Getenv("PASSBOLT_DECRYPT_TEST_USER_PASSPHRASE")
	if passphrase != "" {
		return passphrase
	}

	passphrase = os.Getenv("PASSBOLT_MEMBER_PASSPHRASE")
	if passphrase == "" {
		t.Skip("PASSBOLT_DECRYPT_TEST_USER_PASSPHRASE or PASSBOLT_MEMBER_PASSPHRASE is required for decrypt acceptance tests")
	}

	return passphrase
}

func testAccCurrentUserID(t *testing.T, baseURL, privateKey, passphrase string) string {
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

	msg, err := client.DoCustomRequest(ctx, "GET", "/users/me.json", "v2", nil, nil)
	if err != nil {
		t.Fatalf("failed to get current Passbolt user: %v", err)
	}

	var user api.User
	if err := json.Unmarshal(msg.Body, &user); err != nil {
		t.Fatalf("failed to decode current Passbolt user: %v", err)
	}

	return user.ID
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

func testStepCreateSharedGroupPassword(
	baseURL,
	privateKey,
	passphrase,
	managerID,
	groupName,
	passwordName string,
) resource.TestStep {
	return resource.TestStep{
		Config: testGroupWithSharedPasswordConfig(
			baseURL,
			privateKey,
			passphrase,
			managerID,
			"",
			groupName,
			passwordName,
		),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_group.test", "name", groupName),
			resource.TestCheckResourceAttr("passbolt_group.test", "managers.#", "1"),
			resource.TestCheckResourceAttr("passbolt_group.test", "members.#", "0"),
			resource.TestCheckResourceAttr("passbolt_password.shared", "name", passwordName),
		),
	}
}

func testStepAddMemberToSharedGroup(
	baseURL,
	privateKey,
	passphrase,
	managerID,
	memberID,
	groupName,
	passwordName string,
) resource.TestStep {
	return resource.TestStep{
		Config: testGroupWithSharedPasswordConfig(
			baseURL,
			privateKey,
			passphrase,
			managerID,
			memberID,
			groupName,
			passwordName,
		),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_group.test", "name", groupName),
			resource.TestCheckResourceAttr("passbolt_group.test", "managers.#", "1"),
			resource.TestCheckResourceAttr("passbolt_group.test", "members.#", "1"),
			resource.TestCheckResourceAttr("passbolt_password.shared", "name", passwordName),
		),
	}
}

func testStepAddMemberToSharedGroupAndDecrypt(
	baseURL,
	privateKey,
	passphrase,
	memberPrivateKey,
	memberPassphrase,
	managerID,
	memberID,
	groupName,
	passwordName string,
) resource.TestStep {
	return resource.TestStep{
		Config: testGroupWithSharedPasswordConfig(
			baseURL,
			privateKey,
			passphrase,
			managerID,
			memberID,
			groupName,
			passwordName,
		),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_group.test", "name", groupName),
			resource.TestCheckResourceAttr("passbolt_group.test", "managers.#", "1"),
			resource.TestCheckResourceAttr("passbolt_group.test", "members.#", "1"),
			resource.TestCheckResourceAttr("passbolt_password.shared", "name", passwordName),
			testCheckSharedPasswordDecryptableAsMember(baseURL, memberPrivateKey, memberPassphrase, passwordName),
		),
	}
}

func testStepNoDriftSharedGroupMember(
	baseURL,
	privateKey,
	passphrase,
	managerID,
	memberID,
	groupName,
	passwordName string,
) resource.TestStep {
	return testStepAddMemberToSharedGroup(
		baseURL,
		privateKey,
		passphrase,
		managerID,
		memberID,
		groupName,
		passwordName,
	)
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

func testGroupWithSharedPasswordConfig(
	baseURL,
	privateKey,
	passphrase,
	managerID,
	memberID,
	groupName,
	passwordName string,
) string {
	members := ""
	if memberID != "" {
		members = fmt.Sprintf(`
  members  = ["%s"]`, memberID)
	}

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
  managers = ["%s"]%s
}

resource "passbolt_password" "shared" {
  name         = "%s"
  username     = "shared-user"
  uri          = "https://shared-group.example.com"
  password     = "shared-secret"
  share_groups = [passbolt_group.test.name]
}
`, baseURL, privateKey, passphrase, groupName, managerID, members, passwordName)
}

func testGroupIgnoreInactiveMembersConfig(baseURL, privateKey, passphrase, managerID, email, groupName string) string {
	return fmt.Sprintf(`
provider "passbolt" {
  base_url    = "%s"
  private_key = <<EOF
%s
EOF
  passphrase  = "%s"
}

resource "passbolt_user" "member" {
  username   = "%s"
  first_name = "Pending"
  last_name  = "Member"
  role       = "user"
}

resource "passbolt_group" "test" {
  name                    = "%s"
  managers                = ["%s"]
  members                 = [passbolt_user.member.id]
  ignore_inactive_members = true
}
`, baseURL, privateKey, passphrase, email, groupName, managerID)
}

func testCheckSharedPasswordDecryptableAsMember(
	baseURL,
	memberPrivateKey,
	memberPassphrase,
	passwordName string,
) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		resourceState, ok := state.RootModule().Resources["passbolt_password.shared"]
		if !ok {
			return fmt.Errorf("passbolt_password.shared not found in Terraform state")
		}

		resourceID := resourceState.Primary.ID
		if resourceID == "" {
			return fmt.Errorf("passbolt_password.shared id is empty")
		}

		ctx := context.Background()
		client, err := api.NewClient(nil, "", baseURL, memberPrivateKey, memberPassphrase)
		if err != nil {
			return fmt.Errorf("failed to create Passbolt API client for member: %w", err)
		}
		if err := client.Login(ctx); err != nil {
			return fmt.Errorf("failed to log in as shared-group member: %w", err)
		}
		defer func() {
			_ = client.Logout(ctx)
		}()

		_, name, username, uri, password, _, err := helper.GetResource(ctx, client, resourceID)
		if err != nil {
			return fmt.Errorf("failed to decrypt shared resource %s as member: %w", resourceID, err)
		}

		if name != passwordName {
			return fmt.Errorf("expected shared resource name %q, got %q", passwordName, name)
		}
		if username != "shared-user" {
			return fmt.Errorf("expected shared resource username %q, got %q", "shared-user", username)
		}
		if uri != "https://shared-group.example.com" {
			return fmt.Errorf("expected shared resource uri %q, got %q", "https://shared-group.example.com", uri)
		}
		if password != "shared-secret" {
			return fmt.Errorf("expected shared resource password %q, got %q", "shared-secret", password)
		}

		return nil
	}
}

func testCheckGroupDoesNotContainMember(
	baseURL,
	privateKey,
	passphrase,
	groupResourceName,
	userResourceName string,
) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		groupResource, ok := state.RootModule().Resources[groupResourceName]
		if !ok {
			return fmt.Errorf("%s not found in Terraform state", groupResourceName)
		}

		userResource, ok := state.RootModule().Resources[userResourceName]
		if !ok {
			return fmt.Errorf("%s not found in Terraform state", userResourceName)
		}

		groupID := groupResource.Primary.ID
		userID := userResource.Primary.ID
		if groupID == "" || userID == "" {
			return fmt.Errorf("group id %q or user id %q is empty", groupID, userID)
		}

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

		_, memberships, err := helper.GetGroup(ctx, client, groupID)
		if err != nil {
			return fmt.Errorf("failed to get Passbolt group %s: %w", groupID, err)
		}

		for _, membership := range memberships {
			if membership.UserID == userID {
				return fmt.Errorf("expected inactive user %s to be skipped from group %s", userID, groupID)
			}
		}

		return nil
	}
}

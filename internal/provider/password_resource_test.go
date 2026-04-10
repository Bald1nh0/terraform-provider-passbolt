package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccPasswordResource_basic(t *testing.T) {
	t.Parallel()

	requireAcceptanceEnv(t, "PASSBOLT_BASE_URL", "PASSBOLT_PRIVATE_KEY", "PASSBOLT_PASSPHRASE", "PASSBOLT_MANAGER_ID")

	baseURL := os.Getenv("PASSBOLT_BASE_URL")
	privateKey := os.Getenv("PASSBOLT_PRIVATE_KEY")
	passphrase := os.Getenv("PASSBOLT_PASSPHRASE")
	managerID := os.Getenv("PASSBOLT_MANAGER_ID")
	suffix := testAccSuffix()
	passwordName := testAccName("acc-test", suffix)
	updatedPasswordName := testAccName("acc-test-updated", suffix)
	sharedPasswordName := testAccName("acc-test-shared", suffix)
	multiSharedPasswordName := testAccName("acc-test-multishare", suffix)
	sharedGroupName := testAccName("tf-acc-group", suffix)
	multiGroupNames := []string{
		testAccName("tf-acc-group-1", suffix),
		testAccName("tf-acc-group-2", suffix),
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			testStepCreatePassword(baseURL, privateKey, passphrase, passwordName),
			testStepCheckDriftless(baseURL, privateKey, passphrase, passwordName),
			testStepShareSameGroupTwice(baseURL, privateKey, passphrase, sharedPasswordName, sharedGroupName, managerID),
			testStepShareSameGroupTwice(baseURL, privateKey, passphrase, sharedPasswordName, sharedGroupName, managerID),
			testStepUpdatePassword(baseURL, privateKey, passphrase, updatedPasswordName),
			testStepShareMultipleGroups(baseURL, privateKey, passphrase,
				multiSharedPasswordName, multiGroupNames, managerID),
		},
	})
}

func testStepCreatePassword(baseURL, privateKey, passphrase, name string) resource.TestStep {
	return resource.TestStep{
		Config: testPasswordConfig(
			baseURL,
			privateKey,
			passphrase,
			name,
			"user",
			"https://example.com",
			"super-secret",
		),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_password.example", "name", name),
			resource.TestCheckResourceAttr("passbolt_password.example", "username", "user"),
			resource.TestCheckResourceAttr("passbolt_password.example", "uri", "https://example.com"),
		),
	}
}

func testStepCheckDriftless(baseURL, privateKey, passphrase, name string) resource.TestStep {
	return resource.TestStep{
		// same config to check drift
		Config: testPasswordConfig(
			baseURL,
			privateKey,
			passphrase,
			name,
			"user",
			"https://example.com",
			"super-secret",
		),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_password.example", "name", name),
			resource.TestCheckResourceAttr("passbolt_password.example", "username", "user"),
			resource.TestCheckResourceAttr("passbolt_password.example", "uri", "https://example.com"),
		),
	}
}

func testStepUpdatePassword(baseURL, privateKey, passphrase, name string) resource.TestStep {
	return resource.TestStep{
		Config: testPasswordConfig(
			baseURL,
			privateKey,
			passphrase,
			name,
			"updated-user",
			"https://example.org",
			"new-secret",
		),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_password.example", "name", name),
			resource.TestCheckResourceAttr("passbolt_password.example", "username", "updated-user"),
			resource.TestCheckResourceAttr("passbolt_password.example", "uri", "https://example.org"),
		),
	}
}

func testStepShareSameGroupTwice(
	baseURL,
	privateKey,
	passphrase,
	passwordName,
	groupName,
	managerID string,
) resource.TestStep {
	return resource.TestStep{
		Config: testPasswordWithShareConfig(
			baseURL,
			privateKey,
			passphrase,
			passwordName,
			managerID,
			"user",
			"https://shared.example.com",
			"shared-secret",
			groupName,
		),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_password.shared", "name", passwordName),
			resource.TestCheckResourceAttr("passbolt_password.shared", "share_group", groupName),
		),
	}
}

func testStepShareMultipleGroups(baseURL,
	privateKey, passphrase, passwordName string, groups []string, managerID string) resource.TestStep {
	return resource.TestStep{
		Config: testPasswordWithShareGroupsConfig(
			baseURL,
			privateKey,
			passphrase,
			passwordName,
			managerID,
			"user2",
			"https://multi.example.com",
			"multi-secret",
			groups,
		),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_password.shared", "name", passwordName),
			resource.TestCheckResourceAttr("passbolt_password.shared", "username", "user2"),
			resource.TestCheckResourceAttr("passbolt_password.shared", "uri", "https://multi.example.com"),
		),
	}
}

func testPasswordWithShareGroupsConfig(
	baseURL,
	privateKey,
	passphrase,
	name,
	managerID,
	username,
	uri,
	password string,
	groups []string,
) string {
	groupResources := ""
	groupNames := ""
	for i, g := range groups {
		groupResources += fmt.Sprintf(`
resource "passbolt_group" "g%d" {
  name     = "%s"
  managers = ["%s"]
}
`, i, g, managerID)
		groupNames += fmt.Sprintf(`passbolt_group.g%d.name,`, i)
	}
	groupNames = groupNames[:len(groupNames)-1] // trim last comma

	return fmt.Sprintf(`
provider "passbolt" {
  base_url    = "%s"
  private_key = <<EOF
%s
EOF
  passphrase = "%s"
}

%s

resource "passbolt_password" "shared" {
  name          = "%s"
  username      = "%s"
  uri           = "%s"
  password      = "%s"
  share_groups  = [%s]
}
`, baseURL, privateKey, passphrase, groupResources, name, username, uri, password, groupNames)
}

func testPasswordWithShareConfig(
	baseURL,
	privateKey,
	passphrase,
	name,
	managerID,
	username,
	uri,
	password,
	group string) string {
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

resource "passbolt_password" "shared" {
  name         = "%s"
  username     = "%s"
  uri          = "%s"
  password     = "%s"
  share_group  = passbolt_group.test.name
}
`, baseURL, privateKey, passphrase, group, managerID, name, username, uri, password)
}

func testPasswordConfig(baseURL, privateKey, passphrase, name, username, uri, password string) string {
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
  username = "%s"
  uri      = "%s"
  password = "%s"
}
`, baseURL, privateKey, passphrase, name, username, uri, password)
}

package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccPasswordResource_basic(t *testing.T) {
	t.Parallel()

	baseURL := os.Getenv("PASSBOLT_BASE_URL")
	privateKey := os.Getenv("PASSBOLT_PRIVATE_KEY")
	passphrase := os.Getenv("PASSBOLT_PASSPHRASE")
	managerID := os.Getenv("PASSBOLT_MANAGER_ID")

	if baseURL == "" || privateKey == "" || passphrase == "" || managerID == "" {
		t.Skip("PASSBOLT_BASE_URL, PRIVATE_KEY, PASSPHRASE, and MANAGER_ID must be set for acceptance tests")
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			testStepCreatePassword(baseURL, privateKey, passphrase),
			testStepCheckDriftless(baseURL, privateKey, passphrase),
			testStepShareSameGroupTwice(baseURL, privateKey, passphrase, "tf-acc-group", managerID),
			testStepShareSameGroupTwice(baseURL, privateKey, passphrase, "tf-acc-group", managerID),
			testStepUpdatePassword(baseURL, privateKey, passphrase),
			testStepShareMultipleGroups(baseURL, privateKey, passphrase,
				[]string{"tf-acc-group-1", "tf-acc-group-2"}, managerID),
		},
	})
}

func testStepCreatePassword(baseURL, privateKey, passphrase string) resource.TestStep {
	return resource.TestStep{
		Config: testPasswordConfig(
			baseURL,
			privateKey,
			passphrase,
			"acc-test",
			"user",
			"https://example.com",
			"super-secret",
		),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_password.example", "name", "acc-test"),
			resource.TestCheckResourceAttr("passbolt_password.example", "username", "user"),
			resource.TestCheckResourceAttr("passbolt_password.example", "uri", "https://example.com"),
		),
	}
}

func testStepCheckDriftless(baseURL, privateKey, passphrase string) resource.TestStep {
	return resource.TestStep{
		// same config to check drift
		Config: testPasswordConfig(
			baseURL,
			privateKey,
			passphrase,
			"acc-test",
			"user",
			"https://example.com",
			"super-secret",
		),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_password.example", "name", "acc-test"),
			resource.TestCheckResourceAttr("passbolt_password.example", "username", "user"),
			resource.TestCheckResourceAttr("passbolt_password.example", "uri", "https://example.com"),
		),
	}
}

func testStepUpdatePassword(baseURL, privateKey, passphrase string) resource.TestStep {
	return resource.TestStep{
		Config: testPasswordConfig(
			baseURL,
			privateKey,
			passphrase,
			"acc-test-updated",
			"updated-user",
			"https://example.org",
			"new-secret",
		),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_password.example", "name", "acc-test-updated"),
			resource.TestCheckResourceAttr("passbolt_password.example", "username", "updated-user"),
			resource.TestCheckResourceAttr("passbolt_password.example", "uri", "https://example.org"),
		),
	}
}

func testStepShareSameGroupTwice(baseURL, privateKey, passphrase, groupName, managerID string) resource.TestStep {
	return resource.TestStep{
		Config: testPasswordWithShareConfig(
			baseURL,
			privateKey,
			passphrase,
			"acc-test-shared",
			managerID,
			"user",
			"https://shared.example.com",
			"shared-secret",
			groupName,
		),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_password.shared", "name", "acc-test-shared"),
			resource.TestCheckResourceAttr("passbolt_password.shared", "share_group", groupName),
		),
	}
}

func testStepShareMultipleGroups(baseURL,
	privateKey, passphrase string, groups []string, managerID string) resource.TestStep {
	return resource.TestStep{
		Config: testPasswordWithShareGroupsConfig(
			baseURL,
			privateKey,
			passphrase,
			"acc-test-multishare",
			managerID,
			"user2",
			"https://multi.example.com",
			"multi-secret",
			groups,
		),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_password.shared", "name", "acc-test-multishare"),
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

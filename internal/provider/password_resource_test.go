package provider_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/passbolt/go-passbolt/api"
	"github.com/passbolt/go-passbolt/helper"
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

func TestAccPasswordResource_writeOnly(t *testing.T) {
	t.Parallel()

	requireAcceptanceEnv(t, "PASSBOLT_BASE_URL", "PASSBOLT_PRIVATE_KEY", "PASSBOLT_PASSPHRASE")

	baseURL := os.Getenv("PASSBOLT_BASE_URL")
	privateKey := os.Getenv("PASSBOLT_PRIVATE_KEY")
	passphrase := os.Getenv("PASSBOLT_PASSPHRASE")
	suffix := testAccSuffix()
	passwordName := testAccName("acc-test-wo", suffix)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			testStepCreatePasswordWriteOnly(baseURL, privateKey, passphrase, passwordName),
			testStepUpdatePasswordWriteOnlyWithoutRotation(baseURL, privateKey, passphrase, passwordName),
			testStepRotatePasswordWriteOnly(baseURL, privateKey, passphrase, passwordName),
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

func testStepCreatePasswordWriteOnly(baseURL, privateKey, passphrase, name string) resource.TestStep {
	return resource.TestStep{
		Config: testPasswordWriteOnlyConfig(
			baseURL,
			privateKey,
			passphrase,
			name,
			"wo-user",
			"https://write-only.example.com",
			"initial-write-only-secret",
			1,
		),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_password.example", "name", name),
			resource.TestCheckResourceAttr("passbolt_password.example", "username", "wo-user"),
			resource.TestCheckResourceAttr("passbolt_password.example", "uri", "https://write-only.example.com"),
			testCheckPasswordValue(
				baseURL,
				privateKey,
				passphrase,
				"passbolt_password.example",
				name,
				"wo-user",
				"https://write-only.example.com",
				"initial-write-only-secret",
			),
		),
		ConfigStateChecks: passwordWriteOnlyStateChecks(1),
	}
}

func testStepUpdatePasswordWriteOnlyWithoutRotation(baseURL, privateKey, passphrase, name string) resource.TestStep {
	return resource.TestStep{
		Config: testPasswordWriteOnlyConfig(
			baseURL,
			privateKey,
			passphrase,
			name,
			"wo-user-updated",
			"https://write-only-updated.example.com",
			"should-not-rotate-without-version-bump",
			1,
		),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_password.example", "name", name),
			resource.TestCheckResourceAttr("passbolt_password.example", "username", "wo-user-updated"),
			resource.TestCheckResourceAttr("passbolt_password.example", "uri", "https://write-only-updated.example.com"),
			testCheckPasswordValue(
				baseURL,
				privateKey,
				passphrase,
				"passbolt_password.example",
				name,
				"wo-user-updated",
				"https://write-only-updated.example.com",
				"initial-write-only-secret",
			),
		),
		ConfigStateChecks: passwordWriteOnlyStateChecks(1),
	}
}

func testStepRotatePasswordWriteOnly(baseURL, privateKey, passphrase, name string) resource.TestStep {
	return resource.TestStep{
		Config: testPasswordWriteOnlyConfig(
			baseURL,
			privateKey,
			passphrase,
			name,
			"wo-user-updated",
			"https://write-only-updated.example.com",
			"rotated-write-only-secret",
			2,
		),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_password.example", "name", name),
			resource.TestCheckResourceAttr("passbolt_password.example", "username", "wo-user-updated"),
			resource.TestCheckResourceAttr("passbolt_password.example", "uri", "https://write-only-updated.example.com"),
			testCheckPasswordValue(
				baseURL,
				privateKey,
				passphrase,
				"passbolt_password.example",
				name,
				"wo-user-updated",
				"https://write-only-updated.example.com",
				"rotated-write-only-secret",
			),
		),
		ConfigStateChecks: passwordWriteOnlyStateChecks(2),
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
	var groupResources strings.Builder
	groupNames := ""
	for i, g := range groups {
		_, _ = fmt.Fprintf(&groupResources, `
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
`, baseURL, privateKey, passphrase, groupResources.String(), name, username, uri, password, groupNames)
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

func testPasswordWriteOnlyConfig(
	baseURL,
	privateKey,
	passphrase,
	name,
	username,
	uri,
	password string,
	version int64,
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
  name                = "%s"
  username            = "%s"
  uri                 = "%s"
  password_wo         = "%s"
  password_wo_version = %d
}
`, baseURL, privateKey, passphrase, name, username, uri, password, version)
}

func passwordWriteOnlyStateChecks(version int64) []statecheck.StateCheck {
	return []statecheck.StateCheck{
		statecheck.ExpectKnownValue(
			"passbolt_password.example",
			tfjsonpath.New("password"),
			knownvalue.Null(),
		),
		statecheck.ExpectKnownValue(
			"passbolt_password.example",
			tfjsonpath.New("password_wo"),
			knownvalue.Null(),
		),
		statecheck.ExpectKnownValue(
			"passbolt_password.example",
			tfjsonpath.New("password_wo_version"),
			knownvalue.Int64Exact(version),
		),
	}
}

func testCheckPasswordValue(
	baseURL,
	privateKey,
	passphrase,
	resourceAddress,
	expectedName,
	expectedUsername,
	expectedURI,
	expectedPassword string,
) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		resourceState, ok := state.RootModule().Resources[resourceAddress]
		if !ok {
			return fmt.Errorf("%s not found in Terraform state", resourceAddress)
		}

		resourceID := resourceState.Primary.ID
		if resourceID == "" {
			return fmt.Errorf("%s id is empty", resourceAddress)
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

		_, name, username, uri, password, _, err := helper.GetResource(ctx, client, resourceID)
		if err != nil {
			return fmt.Errorf("failed to fetch Passbolt resource %s: %w", resourceID, err)
		}

		if name != expectedName {
			return fmt.Errorf("expected resource name %q, got %q", expectedName, name)
		}
		if username != expectedUsername {
			return fmt.Errorf("expected resource username %q, got %q", expectedUsername, username)
		}
		if uri != expectedURI {
			return fmt.Errorf("expected resource uri %q, got %q", expectedURI, uri)
		}
		if password != expectedPassword {
			return fmt.Errorf("expected resource password %q, got %q", expectedPassword, password)
		}

		return nil
	}
}

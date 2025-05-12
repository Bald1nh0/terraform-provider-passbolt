package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccFolderResource_basic(t *testing.T) {
	t.Parallel()

	baseURL := os.Getenv("PASSBOLT_BASE_URL")
	privateKey := os.Getenv("PASSBOLT_PRIVATE_KEY")
	passphrase := os.Getenv("PASSBOLT_PASSPHRASE")

	if baseURL == "" || privateKey == "" || passphrase == "" {
		t.Skip("Acceptance tests require PASSBOLT_BASE_URL, PRIVATE_KEY, and PASSPHRASE")
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			testStepCreateFolder(baseURL, privateKey, passphrase),
			testStepCheckFolderNoDrift(baseURL, privateKey, passphrase),
			testStepUpdateFolderName(baseURL, privateKey, passphrase),
		},
	})
}

func testStepCreateFolder(baseURL, privateKey, passphrase string) resource.TestStep {
	return resource.TestStep{
		Config: testFolderConfig(baseURL, privateKey, passphrase, "acc-folder-test"),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_folder.example", "name", "acc-folder-test"),
			resource.TestCheckResourceAttrSet("passbolt_folder.example", "id"),
		),
	}
}

func testStepCheckFolderNoDrift(baseURL, privateKey, passphrase string) resource.TestStep {
	return resource.TestStep{
		Config: testFolderConfig(baseURL, privateKey, passphrase, "acc-folder-test"),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_folder.example", "name", "acc-folder-test"),
		),
	}
}

func testStepUpdateFolderName(baseURL, privateKey, passphrase string) resource.TestStep {
	return resource.TestStep{
		Config: testFolderConfig(baseURL, privateKey, passphrase, "acc-folder-renamed"),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_folder.example", "name", "acc-folder-renamed"),
		),
	}
}

func testFolderConfig(baseURL, privateKey, passphrase, name string) string {
	return fmt.Sprintf(`
provider "passbolt" {
  base_url    = "%s"
  private_key = <<EOF
%s
EOF
  passphrase  = "%s"
}

resource "passbolt_folder" "example" {
  name = "%s"
}
`, baseURL, privateKey, passphrase, name)
}

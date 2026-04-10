package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccFolderDataSource_basic(t *testing.T) {
	t.Parallel()

	requireAcceptanceEnv(t, "PASSBOLT_BASE_URL", "PASSBOLT_PRIVATE_KEY", "PASSBOLT_PASSPHRASE")

	baseURL := os.Getenv("PASSBOLT_BASE_URL")
	privateKey := os.Getenv("PASSBOLT_PRIVATE_KEY")
	passphrase := os.Getenv("PASSBOLT_PASSPHRASE")
	folderName := testAccName("acc-folder-test", testAccSuffix())

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
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

data "passbolt_folders" "all" {
  depends_on = [passbolt_folder.example]
}

output "created_folder" {
  value = passbolt_folder.example.name
}
`, baseURL, privateKey, passphrase, folderName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.passbolt_folders.all", "folders.0.id"),
					resource.TestCheckResourceAttrSet("data.passbolt_folders.all", "folders.0.path"),
				),
			},
		},
	})
}

package provider_test

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccFolderResource_basic(t *testing.T) {
	t.Parallel()

	requireAcceptanceEnv(t, "PASSBOLT_BASE_URL", "PASSBOLT_PRIVATE_KEY", "PASSBOLT_PASSPHRASE")

	baseURL := os.Getenv("PASSBOLT_BASE_URL")
	privateKey := os.Getenv("PASSBOLT_PRIVATE_KEY")
	passphrase := os.Getenv("PASSBOLT_PASSPHRASE")
	suffix := testAccSuffix()
	initialName := testAccName("acc-folder-test", suffix)
	updatedName := testAccName("acc-folder-renamed", suffix)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			testStepCreateFolder(baseURL, privateKey, passphrase, initialName),
			testStepCheckFolderNoDrift(baseURL, privateKey, passphrase, initialName),
			testStepUpdateFolderName(baseURL, privateKey, passphrase, updatedName),
		},
	})
}

func TestAccFolderResource_parentReferenceVariants(t *testing.T) {
	t.Parallel()

	requireAcceptanceEnv(t, "PASSBOLT_BASE_URL", "PASSBOLT_PRIVATE_KEY", "PASSBOLT_PASSPHRASE")

	baseURL := os.Getenv("PASSBOLT_BASE_URL")
	privateKey := os.Getenv("PASSBOLT_PRIVATE_KEY")
	passphrase := os.Getenv("PASSBOLT_PASSPHRASE")
	suffix := testAccSuffix()

	applicationAName := testAccName("application-a", suffix)
	applicationBName := testAccName("application-b", suffix)
	pathNestedName := testAccName("sub-folder-path", suffix)
	dataSourceNestedName := testAccName("sub-folder-datasource", suffix)
	prodPath := fmt.Sprintf("/%s/prod", applicationAName)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			testFolderParentReferenceStep(
				baseURL,
				privateKey,
				passphrase,
				applicationAName,
				applicationBName,
				pathNestedName,
				dataSourceNestedName,
				prodPath,
			),
			testFolderAmbiguousParentStep(
				baseURL,
				privateKey,
				passphrase,
				applicationAName,
				applicationBName,
				pathNestedName,
			),
		},
	})
}

func testStepCreateFolder(baseURL, privateKey, passphrase, name string) resource.TestStep {
	return resource.TestStep{
		Config: testFolderConfig(baseURL, privateKey, passphrase, name),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_folder.example", "name", name),
			resource.TestCheckResourceAttrSet("passbolt_folder.example", "id"),
		),
	}
}

func testStepCheckFolderNoDrift(baseURL, privateKey, passphrase, name string) resource.TestStep {
	return resource.TestStep{
		Config: testFolderConfig(baseURL, privateKey, passphrase, name),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_folder.example", "name", name),
		),
	}
}

func testStepUpdateFolderName(baseURL, privateKey, passphrase, name string) resource.TestStep {
	return resource.TestStep{
		Config: testFolderConfig(baseURL, privateKey, passphrase, name),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("passbolt_folder.example", "name", name),
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

func testFolderParentReferenceConfig(
	baseURL,
	privateKey,
	passphrase,
	applicationAName,
	applicationBName,
	pathNestedName,
	dataSourceNestedName string,
) string {
	return fmt.Sprintf(`
%s

%s

data "passbolt_folders" "all" {
  depends_on = [
    passbolt_folder.application_a_prod,
    passbolt_folder.application_b_prod,
  ]
}

resource "passbolt_folder" "path_nested" {
  name          = "%s"
  folder_parent = "/%s/prod"
  depends_on = [
    passbolt_folder.application_a_prod,
  ]
}

resource "passbolt_folder" "datasource_nested" {
  name          = "%s"
  folder_parent = one([for f in data.passbolt_folders.all.folders : f.id if f.path == "/%s/prod"])
}
`,
		testPassboltProviderConfig(baseURL, privateKey, passphrase),
		testFolderParentBaseConfig(applicationAName, applicationBName),
		pathNestedName,
		applicationAName,
		dataSourceNestedName,
		applicationAName,
	)
}

func testFolderAmbiguousParentConfig(
	baseURL,
	privateKey,
	passphrase,
	applicationAName,
	applicationBName,
	ambiguousName string,
) string {
	return fmt.Sprintf(`
%s

%s

resource "passbolt_folder" "ambiguous_nested" {
  name          = "%s"
  folder_parent = "prod"
  depends_on = [
    passbolt_folder.application_a_prod,
    passbolt_folder.application_b_prod,
  ]
}
`,
		testPassboltProviderConfig(baseURL, privateKey, passphrase),
		testFolderParentBaseConfig(applicationAName, applicationBName),
		ambiguousName,
	)
}

func testFolderParentReferenceStep(
	baseURL,
	privateKey,
	passphrase,
	applicationAName,
	applicationBName,
	pathNestedName,
	dataSourceNestedName,
	prodPath string,
) resource.TestStep {
	return resource.TestStep{
		Config: testFolderParentReferenceConfig(
			baseURL,
			privateKey,
			passphrase,
			applicationAName,
			applicationBName,
			pathNestedName,
			dataSourceNestedName,
		),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttrPair(
				"passbolt_folder.application_a_prod",
				"folder_parent",
				"passbolt_folder.application_a",
				"id",
			),
			resource.TestCheckResourceAttrPair(
				"passbolt_folder.application_a_prod",
				"folder_parent_id",
				"passbolt_folder.application_a",
				"id",
			),
			resource.TestCheckResourceAttrPair(
				"passbolt_folder.application_b_prod",
				"folder_parent",
				"passbolt_folder.application_b",
				"id",
			),
			resource.TestCheckResourceAttr("passbolt_folder.path_nested", "folder_parent", prodPath),
			resource.TestCheckResourceAttrPair(
				"passbolt_folder.path_nested",
				"folder_parent_id",
				"passbolt_folder.application_a_prod",
				"id",
			),
			resource.TestCheckResourceAttrPair(
				"passbolt_folder.datasource_nested",
				"folder_parent",
				"passbolt_folder.application_a_prod",
				"id",
			),
			resource.TestCheckResourceAttrPair(
				"passbolt_folder.datasource_nested",
				"folder_parent_id",
				"passbolt_folder.application_a_prod",
				"id",
			),
		),
	}
}

func testFolderAmbiguousParentStep(
	baseURL,
	privateKey,
	passphrase,
	applicationAName,
	applicationBName,
	ambiguousName string,
) resource.TestStep {
	return resource.TestStep{
		Config: testFolderAmbiguousParentConfig(
			baseURL,
			privateKey,
			passphrase,
			applicationAName,
			applicationBName,
			ambiguousName,
		),
		ExpectError: regexp.MustCompile(`folder name "prod" is ambiguous`),
	}
}

func testPassboltProviderConfig(baseURL, privateKey, passphrase string) string {
	return fmt.Sprintf(`
provider "passbolt" {
  base_url    = "%s"
  private_key = <<EOF
%s
EOF
  passphrase  = "%s"
}
`, baseURL, privateKey, passphrase)
}

func testFolderParentBaseConfig(applicationAName, applicationBName string) string {
	return fmt.Sprintf(`
resource "passbolt_folder" "application_a" {
  name = "%s"
}

resource "passbolt_folder" "application_a_prod" {
  name          = "prod"
  folder_parent = passbolt_folder.application_a.id
}

resource "passbolt_folder" "application_b" {
  name = "%s"
}

resource "passbolt_folder" "application_b_prod" {
  name          = "prod"
  folder_parent = passbolt_folder.application_b.id
}
`, applicationAName, applicationBName)
}

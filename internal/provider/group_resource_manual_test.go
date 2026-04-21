//go:build manualgui

package provider_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestManualPassboltGroupAddMemberToSharedGroupApplyForGUI(t *testing.T) {
	t.Parallel()

	requireAcceptanceEnv(t, "PASSBOLT_BASE_URL", "PASSBOLT_PRIVATE_KEY", "PASSBOLT_PASSPHRASE")

	baseURL := os.Getenv("PASSBOLT_BASE_URL")
	privateKey := os.Getenv("PASSBOLT_PRIVATE_KEY")
	passphrase := os.Getenv("PASSBOLT_PASSPHRASE")
	managerID := testAccCurrentUserID(t, baseURL, privateKey, passphrase)
	memberID := testAccDecryptGroupMemberID(t, baseURL, privateKey, passphrase, managerID)
	runID := manualGUIRunID(t)
	workdir := manualGUIWorkdir(t, runID)
	groupName := fmt.Sprintf("manual-gui-group-%s", runID)
	passwordName := fmt.Sprintf("manual-gui-secret-%s", runID)

	if err := os.MkdirAll(workdir, 0o755); err != nil {
		t.Fatalf("failed to create manual GUI workdir: %v", err)
	}

	writeManualGUIConfig(
		t,
		workdir,
		manualGUIConfig(baseURL, privateKey, passphrase, managerID, memberID, groupName, passwordName, false),
	)
	runTerraform(t, workdir, "init", "-input=false")
	runTerraform(t, workdir, "apply", "-auto-approve", "-input=false")

	writeManualGUIConfig(
		t,
		workdir,
		manualGUIConfig(baseURL, privateKey, passphrase, managerID, memberID, groupName, passwordName, true),
	)
	runTerraform(t, workdir, "apply", "-auto-approve", "-input=false")

	output := runTerraform(t, workdir, "output")
	t.Logf("Manual GUI verification is ready in %s", workdir)
	t.Logf("Terraform outputs:\n%s", output)
	t.Logf("Open Passbolt UI and verify whether user %s can decrypt secret %q in group %q", memberID, passwordName, groupName)
	t.Log("Run the destroy test with the same PASSBOLT_MANUAL_GUI_RUN_ID after verification.")
}

func TestManualPassboltGroupAddMemberToSharedGroupDestroyForGUI(t *testing.T) {
	t.Parallel()

	workdir := manualGUIWorkdir(t, manualGUIRunID(t))
	if _, err := os.Stat(filepath.Join(workdir, "terraform.tfstate")); err != nil {
		t.Fatalf("manual GUI state not found in %s: %v", workdir, err)
	}

	runTerraform(t, workdir, "destroy", "-auto-approve", "-input=false")
	t.Logf("Manual GUI resources destroyed from %s", workdir)
}

func manualGUIRunID(t *testing.T) string {
	t.Helper()

	runID := strings.TrimSpace(os.Getenv("PASSBOLT_MANUAL_GUI_RUN_ID"))
	if runID == "" {
		t.Skip("PASSBOLT_MANUAL_GUI_RUN_ID is required for manual GUI tests")
	}

	return runID
}

func manualGUIWorkdir(t *testing.T, runID string) string {
	t.Helper()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}

	repoRoot := filepath.Clean(filepath.Join(cwd, "..", ".."))
	return filepath.Join(repoRoot, ".tmp", "manual-gui", runID)
}

func writeManualGUIConfig(t *testing.T, workdir, config string) {
	t.Helper()

	configPath := filepath.Join(workdir, "main.tf")
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatalf("failed to write manual GUI config: %v", err)
	}
}

func runTerraform(t *testing.T, workdir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("terraform", args...)
	cmd.Dir = workdir
	cmd.Env = os.Environ()

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	if err := cmd.Run(); err != nil {
		t.Fatalf("terraform %s failed:\n%s\nerror: %v", strings.Join(args, " "), output.String(), err)
	}

	return output.String()
}

func manualGUIConfig(
	baseURL,
	privateKey,
	passphrase,
	managerID,
	memberID,
	groupName,
	passwordName string,
	includeMember bool,
) string {
	members := ""
	if includeMember {
		members = fmt.Sprintf(`
  members  = ["%s"]`, memberID)
	}

	return fmt.Sprintf(`
terraform {
  required_providers {
    passbolt = {
      source = "Bald1nh0/passbolt"
    }
  }
}

provider "passbolt" {
  base_url    = "%s"
  private_key = <<EOF
%s
EOF
  passphrase  = "%s"
}

resource "passbolt_group" "manual" {
  name     = "%s"
  managers = ["%s"]%s
}

resource "passbolt_password" "manual" {
  name         = "%s"
  username     = "manual-gui-user"
  uri          = "https://manual-gui.example.com"
  password     = "manual-gui-secret"
  description  = "Manual GUI verification secret"
  share_groups = [passbolt_group.manual.name]
}

output "group_id" {
  value = passbolt_group.manual.id
}

output "group_name" {
  value = passbolt_group.manual.name
}

output "password_id" {
  value = passbolt_password.manual.id
}

output "password_name" {
  value = passbolt_password.manual.name
}

output "member_id" {
  value = "%s"
}
`, baseURL, privateKey, passphrase, groupName, managerID, members, passwordName, memberID)
}

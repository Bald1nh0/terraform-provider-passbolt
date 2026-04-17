package provider_test

import (
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

var acceptanceCounter uint64

func requireAcceptanceEnv(t *testing.T, vars ...string) {
	t.Helper()

	if os.Getenv("TF_ACC") != "1" {
		t.Skip("Acceptance tests skipped unless TF_ACC=1")
	}

	missing := make([]string, 0)
	for _, name := range vars {
		if os.Getenv(name) == "" {
			missing = append(missing, name)
		}
	}

	if len(missing) > 0 {
		t.Skipf("Acceptance tests require %s", strings.Join(missing, ", "))
	}
}

func testAccSuffix() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), atomic.AddUint64(&acceptanceCounter, 1))
}

func testAccName(prefix, suffix string) string {
	return fmt.Sprintf("%s-%s", prefix, suffix)
}

func testAccEmail(prefix, suffix string) string {
	domain := strings.TrimSpace(os.Getenv("PASSBOLT_TEST_EMAIL_DOMAIN"))
	domain = strings.TrimPrefix(domain, "@")
	if domain == "" {
		domain = "example.com"
	}

	return fmt.Sprintf("test-%s-%s@%s", prefix, suffix, domain)
}

func TestTestAccEmailUsesReservedDomainByDefault(t *testing.T) {
	got := testAccEmail("acc.user", "123")
	want := "test-acc.user-123@example.com"
	if got != want {
		t.Fatalf("expected default acceptance email %q, got %q", want, got)
	}
}

func TestTestAccEmailSupportsDomainOverride(t *testing.T) {
	t.Setenv("PASSBOLT_TEST_EMAIL_DOMAIN", "@example.test")

	got := testAccEmail("acc.user", "123")
	want := "test-acc.user-123@example.test"
	if got != want {
		t.Fatalf("expected overridden acceptance email %q, got %q", want, got)
	}
}

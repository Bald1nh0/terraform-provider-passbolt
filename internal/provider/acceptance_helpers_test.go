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
	return fmt.Sprintf("test-%s-%s@bald1nh0.net", prefix, suffix)
}

package provider

import (
	"fmt"
	"testing"

	"github.com/passbolt/go-passbolt/api"
)

func TestDecryptedGroupSecretByResourceIDPrefersCurrentUser(t *testing.T) {
	t.Parallel()

	got, err := decryptedGroupSecretByResourceID(
		[]api.Secret{
			{UserID: "other-user", ResourceID: "resource-1", Data: "other-secret"},
			{UserID: "current-user", ResourceID: "resource-1", Data: "current-secret"},
		},
		"resource-1",
		"current-user",
		testDecryptResults(map[string]string{
			"other-secret":   "other-data",
			"current-secret": "current-data",
		}),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "current-data" {
		t.Fatalf("expected decrypted secret %q, got %q", "current-data", got)
	}
}

func TestDecryptedGroupSecretByResourceIDFallsBackToDecryptableMatch(t *testing.T) {
	t.Parallel()

	got, err := decryptedGroupSecretByResourceID(
		[]api.Secret{
			{UserID: "current-user", ResourceID: "resource-1", Data: "broken-secret"},
			{UserID: "other-user", ResourceID: "resource-1", Data: "other-secret"},
		},
		"resource-1",
		"current-user",
		testDecryptResults(map[string]string{
			"other-secret": "other-data",
		}),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "other-data" {
		t.Fatalf("expected decrypted secret %q, got %q", "other-data", got)
	}
}

func TestDecryptedGroupSecretByResourceIDReturnsErrorWhenNoMatchExists(t *testing.T) {
	t.Parallel()

	_, err := decryptedGroupSecretByResourceID(
		[]api.Secret{
			{UserID: "current-user", ResourceID: "resource-1", Data: "current-secret"},
		},
		"resource-2",
		"current-user",
		testDecryptResults(map[string]string{}),
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "cannot find secret for resource ID resource-2" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDecryptedGroupSecretByResourceIDReturnsErrorWhenNothingDecrypts(t *testing.T) {
	t.Parallel()

	_, err := decryptedGroupSecretByResourceID(
		[]api.Secret{
			{UserID: "current-user", ResourceID: "resource-1", Data: "broken-secret"},
			{UserID: "other-user", ResourceID: "resource-1", Data: "other-broken-secret"},
		},
		"resource-1",
		"current-user",
		testDecryptResults(map[string]string{}),
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "cannot decrypt secret for resource ID resource-1" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func testDecryptResults(results map[string]string) func(string) (string, error) {
	return func(data string) (string, error) {
		if decrypted, ok := results[data]; ok {
			return decrypted, nil
		}

		return "", fmt.Errorf("cannot decrypt %s", data)
	}
}

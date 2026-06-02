package provider

import (
	"encoding/json"
	"testing"

	"github.com/passbolt/go-passbolt/api"
)

func TestPasswordUpdateMapsIncludeMetadataAndSecretFields(t *testing.T) {
	t.Parallel()

	metadata, secret := passwordUpdateMaps(
		"db-admin",
		"admin",
		"https://db.example.com",
		"rotated-secret",
		"production database credentials",
	)

	assertMapString(t, metadata, "name", "db-admin")
	assertMapString(t, metadata, "username", "admin")
	assertMapString(t, metadata, "uri", "https://db.example.com")
	assertMapString(t, metadata, "description", "production database credentials")
	assertMapString(t, secret, "description", "production database credentials")
	assertMapString(t, secret, "password", "rotated-secret")
}

func TestPasswordUpdateMapsOmitEmptyPassword(t *testing.T) {
	t.Parallel()

	_, secret := passwordUpdateMaps(
		"db-admin",
		"admin",
		"https://db.example.com",
		"",
		"production database credentials",
	)

	if _, ok := secret["password"]; ok {
		t.Fatal("password should be omitted when the write-only version did not change")
	}

	assertMapString(t, secret, "description", "production database credentials")
}

func TestPasswordStringUpdateRequestAllowsEmptyDescription(t *testing.T) {
	t.Parallel()

	description := ""
	payload, err := json.Marshal(passwordStringUpdateRequest{
		ID:             "resource-id",
		ResourceTypeID: "resource-type-id",
		Name:           "db-admin",
		Username:       "admin",
		URI:            "https://db.example.com",
		Description:    &description,
	})
	if err != nil {
		t.Fatalf("failed to marshal password string update request: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("failed to decode password string update request: %v", err)
	}

	if got, ok := decoded["description"].(string); !ok || got != "" {
		t.Fatalf("expected explicit empty description, got %#v", decoded["description"])
	}
}

func TestIsV4PasswordStringResource(t *testing.T) {
	t.Parallel()

	if !isV4PasswordStringResource(
		&api.Resource{Metadata: ""},
		&api.ResourceType{Slug: "password-string"},
	) {
		t.Fatal("expected legacy password-string resource to use the v4 update path")
	}

	if isV4PasswordStringResource(
		&api.Resource{Metadata: "encrypted metadata"},
		&api.ResourceType{Slug: "v5-password-string"},
	) {
		t.Fatal("expected v5 password-string resource to use the generic update path")
	}
}

func assertMapString(t *testing.T, values map[string]any, key string, want string) {
	t.Helper()

	got, ok := values[key]
	if !ok {
		t.Fatalf("expected key %q", key)
	}

	gotString, ok := got.(string)
	if !ok {
		t.Fatalf("expected key %q to be string, got %T", key, got)
	}

	if gotString != want {
		t.Fatalf("expected key %q to be %q, got %q", key, want, gotString)
	}
}

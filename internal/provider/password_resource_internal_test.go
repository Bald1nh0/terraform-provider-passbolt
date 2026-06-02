package provider

import "testing"

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

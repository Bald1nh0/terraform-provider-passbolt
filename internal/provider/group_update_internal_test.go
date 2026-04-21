package provider

import (
	"errors"
	"testing"
)

func TestCachedDecryptedSecretReturnsLoadedValueOnCacheMiss(t *testing.T) {
	t.Parallel()

	cache := map[string]string{}
	loadCalls := 0

	got, err := cachedDecryptedSecret(cache, "resource-1", func(resourceID string) (string, error) {
		loadCalls++
		if resourceID != "resource-1" {
			t.Fatalf("expected resource-1, got %s", resourceID)
		}

		return "plaintext", nil
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "plaintext" {
		t.Fatalf("expected plaintext, got %q", got)
	}
	if loadCalls != 1 {
		t.Fatalf("expected loader to be called once, got %d", loadCalls)
	}
	if cache["resource-1"] != "plaintext" {
		t.Fatalf("expected cache to store plaintext, got %q", cache["resource-1"])
	}
}

func TestCachedDecryptedSecretReturnsCachedValue(t *testing.T) {
	t.Parallel()

	cache := map[string]string{
		"resource-1": "cached-plaintext",
	}

	got, err := cachedDecryptedSecret(cache, "resource-1", func(resourceID string) (string, error) {
		t.Fatalf("loader should not be called for cached resource %s", resourceID)

		return "", nil
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != "cached-plaintext" {
		t.Fatalf("expected cached-plaintext, got %q", got)
	}
}

func TestCachedDecryptedSecretReturnsLoaderError(t *testing.T) {
	t.Parallel()

	cache := map[string]string{}
	expectedErr := errors.New("boom")

	_, err := cachedDecryptedSecret(cache, "resource-1", func(_ string) (string, error) {
		return "", expectedErr
	})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
	if _, ok := cache["resource-1"]; ok {
		t.Fatal("expected cache to stay empty on loader error")
	}
}

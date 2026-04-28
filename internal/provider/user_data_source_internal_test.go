package provider

import (
	"testing"

	"github.com/passbolt/go-passbolt/api"
)

func TestActiveUserByUsername(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		users    []api.User
		username string
		wantID   string
		wantErr  string
	}{
		"returns exact active match": {
			users: []api.User{
				{ID: "other", Username: "prefix-alexey@example.com", Active: true},
				{ID: "wanted", Username: "alexey@example.com", Active: true},
			},
			username: "alexey@example.com",
			wantID:   "wanted",
		},
		"matches username case insensitively": {
			users: []api.User{
				{ID: "wanted", Username: "Alexey@Example.com", Active: true},
			},
			username: "alexey@example.com",
			wantID:   "wanted",
		},
		"rejects inactive exact match": {
			users: []api.User{
				{ID: "wanted", Username: "alexey@example.com", Active: false},
				{ID: "other", Username: "alexey+other@example.com", Active: true},
			},
			username: "alexey@example.com",
			wantErr:  "user alexey@example.com exists but is not active in Passbolt",
		},
		"rejects deleted exact match": {
			users: []api.User{
				{ID: "wanted", Username: "alexey@example.com", Active: true, Deleted: true},
				{ID: "other", Username: "alexey+other@example.com", Active: true},
			},
			username: "alexey@example.com",
			wantErr:  "user alexey@example.com exists in Passbolt but is deleted",
		},
		"rejects missing exact match": {
			users: []api.User{
				{ID: "other", Username: "alexey+other@example.com", Active: true},
			},
			username: "alexey@example.com",
			wantErr:  "could not find active Passbolt user with username: alexey@example.com",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assertActiveUserByUsername(t, test.users, test.username, test.wantID, test.wantErr)
		})
	}
}

func TestActiveUserByUsernamePrefersUsableExactMatch(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		users    []api.User
		username string
		wantID   string
	}{
		"returns later active exact match after deleted match": {
			users: []api.User{
				{ID: "deleted", Username: "alexey@example.com", Active: true, Deleted: true},
				{ID: "wanted", Username: "alexey@example.com", Active: true},
			},
			username: "alexey@example.com",
			wantID:   "wanted",
		},
		"returns later active exact match after inactive match": {
			users: []api.User{
				{ID: "inactive", Username: "alexey@example.com", Active: false},
				{ID: "wanted", Username: "alexey@example.com", Active: true},
			},
			username: "alexey@example.com",
			wantID:   "wanted",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assertActiveUserByUsername(t, test.users, test.username, test.wantID, "")
		})
	}
}

func TestUserByUsernameIncludesInactiveWhenEnabled(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		users   []api.User
		wantID  string
		wantErr string
	}{
		"returns inactive exact match": {
			users: []api.User{
				{ID: "wanted", Username: "alexey@example.com", Active: false},
			},
			wantID: "wanted",
		},
		"prefers active exact match": {
			users: []api.User{
				{ID: "inactive", Username: "alexey@example.com", Active: false},
				{ID: "active", Username: "alexey@example.com", Active: true},
			},
			wantID: "active",
		},
		"still rejects deleted exact match": {
			users: []api.User{
				{ID: "deleted", Username: "alexey@example.com", Active: false, Deleted: true},
			},
			wantErr: "user alexey@example.com exists in Passbolt but is deleted",
		},
		"reports missing non-deleted user": {
			users: []api.User{
				{ID: "other", Username: "alexey+other@example.com", Active: true},
			},
			wantErr: "could not find non-deleted Passbolt user with username: alexey@example.com",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assertUserByUsername(t, test.users, "alexey@example.com", true, test.wantID, test.wantErr)
		})
	}
}

func assertActiveUserByUsername(
	t *testing.T,
	users []api.User,
	username string,
	wantID string,
	wantErr string,
) {
	t.Helper()

	assertUserByUsername(t, users, username, false, wantID, wantErr)
}

func assertUserByUsername(
	t *testing.T,
	users []api.User,
	username string,
	includeInactive bool,
	wantID string,
	wantErr string,
) {
	t.Helper()

	user, err := userByUsername(users, username, includeInactive)
	if wantErr != "" {
		if err == nil {
			t.Fatalf("expected error %q", wantErr)
		}
		if err.Error() != wantErr {
			t.Fatalf("expected error %q, got %q", wantErr, err.Error())
		}

		return
	}

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user == nil {
		t.Fatal("expected user, got nil")
	}
	if user.ID != wantID {
		t.Fatalf("expected user ID %q, got %q", wantID, user.ID)
	}
}

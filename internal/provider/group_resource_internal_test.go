package provider

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/passbolt/go-passbolt/api"
	"github.com/passbolt/go-passbolt/helper"
)

func TestValidateGroupMembershipConfig(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		managers []types.String
		members  []types.String
		wantErr  bool
	}{
		"valid": {
			managers: stringValues("manager-1"),
			members:  stringValues("member-1"),
		},
		"requires manager": {
			members: stringValues("member-1"),
			wantErr: true,
		},
		"rejects overlap": {
			managers: stringValues("user-1"),
			members:  stringValues("user-1"),
			wantErr:  true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := validateGroupMembershipConfig(test.managers, test.members)
			if test.wantErr && err == nil {
				t.Fatalf("expected validation error")
			}
			if !test.wantErr && err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestGroupMembershipChangeIncludesRegularMemberRole(t *testing.T) {
	t.Parallel()

	payload, err := json.Marshal(groupUpdateRequest{
		GroupChanges: []groupMembershipChange{
			{
				UserID:  "member-1",
				IsAdmin: boolPtr(false),
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to marshal group update request: %v", err)
	}

	if !strings.Contains(string(payload), `"is_admin":false`) {
		t.Fatalf("expected regular member role in payload, got %s", payload)
	}
}

func TestGroupMembershipAttributesUseSetSemantics(t *testing.T) {
	t.Parallel()

	var resp resource.SchemaResponse
	NewGroupResource().Schema(context.Background(), resource.SchemaRequest{}, &resp)

	managersAttr, ok := resp.Schema.Attributes["managers"]
	if !ok {
		t.Fatal("expected managers attribute in group schema")
	}

	if _, ok := managersAttr.(schema.SetAttribute); !ok {
		t.Fatalf("expected managers to be a set attribute, got %T", managersAttr)
	}

	attr, ok := resp.Schema.Attributes["members"]
	if !ok {
		t.Fatal("expected members attribute in group schema")
	}

	setAttr, ok := attr.(schema.SetAttribute)
	if !ok {
		t.Fatalf("expected members to be a set attribute, got %T", attr)
	}

	if len(setAttr.PlanModifiers) != 0 {
		t.Fatalf("expected members to have no plan modifiers, got %d", len(setAttr.PlanModifiers))
	}

	ignoreInactiveAttr, ok := resp.Schema.Attributes["ignore_inactive_members"]
	if !ok {
		t.Fatal("expected ignore_inactive_members attribute in group schema")
	}

	if _, ok := ignoreInactiveAttr.(schema.BoolAttribute); !ok {
		t.Fatalf("expected ignore_inactive_members to be a bool attribute, got %T", ignoreInactiveAttr)
	}
}

func TestResolveGroupMembersForUpdate(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		configMembers types.Set
		planMembers   types.Set
		stateMembers  types.Set
		want          []types.String
	}{
		"preserves state when members omitted": {
			configMembers: types.SetNull(types.StringType),
			planMembers:   types.SetUnknown(types.StringType),
			stateMembers:  setStringValue(stringValues("member-1")),
			want:          stringValues("member-1"),
		},
		"preserves state when members remain unknown": {
			configMembers: types.SetUnknown(types.StringType),
			planMembers:   types.SetUnknown(types.StringType),
			stateMembers:  setStringValue(stringValues("member-1")),
			want:          stringValues("member-1"),
		},
		"uses explicit empty members list": {
			configMembers: setStringValue([]types.String{}),
			planMembers:   setStringValue([]types.String{}),
			stateMembers:  setStringValue(stringValues("member-1")),
			want:          nil,
		},
		"uses configured members list": {
			configMembers: setStringValue(stringValues("member-2")),
			planMembers:   setStringValue(stringValues("member-2")),
			stateMembers:  setStringValue(stringValues("member-1")),
			want:          stringValues("member-2"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assertResolvedGroupMembers(t, test.configMembers, test.planMembers, test.stateMembers, test.want)
		})
	}
}

func TestFilterInactivePendingGroupMembers(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		desiredMembers []types.String
		currentMembers []types.String
		users          []api.User
		wantMembers    []types.String
		wantSkipped    []string
	}{
		"skips inactive pending members": {
			desiredMembers: stringValues("active-user", "inactive-user"),
			users: []api.User{
				{ID: "active-user", Active: true},
				{ID: "inactive-user", Active: false},
			},
			wantMembers: stringValues("active-user"),
			wantSkipped: []string{"inactive-user"},
		},
		"preserves existing inactive members": {
			desiredMembers: stringValues("inactive-user"),
			currentMembers: stringValues("inactive-user"),
			users: []api.User{
				{ID: "inactive-user", Active: false},
			},
			wantMembers: stringValues("inactive-user"),
		},
		"does not skip deleted or unknown users": {
			desiredMembers: stringValues("deleted-user", "missing-user"),
			users: []api.User{
				{ID: "deleted-user", Active: false, Deleted: true},
			},
			wantMembers: stringValues("deleted-user", "missing-user"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assertFilteredInactivePendingGroupMembers(
				t,
				test.desiredMembers,
				test.currentMembers,
				test.users,
				test.wantMembers,
				test.wantSkipped,
			)
		})
	}
}

func TestHasPendingGroupMembers(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		desiredMembers []types.String
		currentMembers []types.String
		want           bool
	}{
		"returns false when desired list is empty": {
			want: false,
		},
		"returns false when all desired members already exist": {
			desiredMembers: stringValues("member-1"),
			currentMembers: stringValues("member-1", "member-2"),
			want:           false,
		},
		"returns true when at least one desired member is missing": {
			desiredMembers: stringValues("member-1", "member-3"),
			currentMembers: stringValues("member-1", "member-2"),
			want:           true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := hasPendingGroupMembers(test.desiredMembers, test.currentMembers)
			if got != test.want {
				t.Fatalf("expected %t, got %t", test.want, got)
			}
		})
	}
}

func assertFilteredInactivePendingGroupMembers(
	t *testing.T,
	desiredMembers []types.String,
	currentMembers []types.String,
	users []api.User,
	wantMembers []types.String,
	wantSkipped []string,
) {
	t.Helper()

	gotMembers, gotSkipped := filterInactivePendingGroupMembers(
		desiredMembers,
		currentMembers,
		users,
	)

	slices.SortFunc(gotMembers, compareStringValues)
	slices.SortFunc(wantMembers, compareStringValues)

	if len(gotMembers) != len(wantMembers) {
		t.Fatalf("expected %d members, got %d: %#v", len(wantMembers), len(gotMembers), gotMembers)
	}

	for i := range wantMembers {
		if gotMembers[i] != wantMembers[i] {
			t.Fatalf("member %d: expected %#v, got %#v", i, wantMembers[i], gotMembers[i])
		}
	}

	if strings.Join(gotSkipped, ",") != strings.Join(wantSkipped, ",") {
		t.Fatalf("expected skipped %v, got %v", wantSkipped, gotSkipped)
	}
}

func assertResolvedGroupMembers(
	t *testing.T,
	configMembers types.Set,
	planMembers types.Set,
	stateMembers types.Set,
	want []types.String,
) {
	t.Helper()

	var diags diag.Diagnostics
	got := resolveGroupMembersForUpdate(
		context.Background(),
		configMembers,
		planMembers,
		stateMembers,
		&diags,
	)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	slices.SortFunc(got, compareStringValues)
	slices.SortFunc(want, compareStringValues)

	if len(got) != len(want) {
		t.Fatalf("expected %d members, got %d: %#v", len(want), len(got), got)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("member %d: expected %#v, got %#v", i, want[i], got[i])
		}
	}
}

func compareStringValues(left, right types.String) int {
	return strings.Compare(left.ValueString(), right.ValueString())
}

func TestBuildCreateGroupMembershipOps(t *testing.T) {
	t.Parallel()

	got := buildCreateGroupMembershipOps(
		stringValues("manager-1"),
		stringValues("member-1", "member-2"),
	)

	want := []helper.GroupMembershipOperation{
		{UserID: "manager-1", IsGroupManager: true},
		{UserID: "member-1", IsGroupManager: false},
		{UserID: "member-2", IsGroupManager: false},
	}

	assertGroupMembershipOps(t, got, want)
}

func TestBuildGroupMembershipOps(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		planManagers  []types.String
		planMembers   []types.String
		stateManagers []types.String
		stateMembers  []types.String
		want          []helper.GroupMembershipOperation
	}{
		"adds updates and removes memberships": {
			planManagers:  stringValues("manager-b", "member-a"),
			planMembers:   stringValues("member-b"),
			stateManagers: stringValues("manager-a"),
			stateMembers:  stringValues("member-a", "member-old"),
			want: []helper.GroupMembershipOperation{
				{UserID: "manager-b", IsGroupManager: true},
				{UserID: "member-a", IsGroupManager: true},
				{UserID: "member-b", IsGroupManager: false},
				{UserID: "manager-a", Delete: true},
				{UserID: "member-old", Delete: true},
			},
		},
		"promotes before demoting": {
			planManagers:  stringValues("manager-b"),
			planMembers:   stringValues("manager-a"),
			stateManagers: stringValues("manager-a"),
			stateMembers:  stringValues("manager-b"),
			want: []helper.GroupMembershipOperation{
				{UserID: "manager-b", IsGroupManager: true},
				{UserID: "manager-a", IsGroupManager: false},
			},
		},
		"no changes": {
			planManagers:  stringValues("manager-a"),
			planMembers:   stringValues("member-a"),
			stateManagers: stringValues("manager-a"),
			stateMembers:  stringValues("member-a"),
			want:          []helper.GroupMembershipOperation{},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := buildGroupMembershipOps(
				test.planManagers,
				test.planMembers,
				test.stateManagers,
				test.stateMembers,
			)

			assertGroupMembershipOps(t, got, test.want)
		})
	}
}

func TestShouldUpdateGroup(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		desiredName string
		currentName string
		ops         []helper.GroupMembershipOperation
		want        bool
	}{
		"returns false for local-only flag changes": {
			desiredName: "group-a",
			currentName: "group-a",
			want:        false,
		},
		"returns true when name changes": {
			desiredName: "group-b",
			currentName: "group-a",
			want:        true,
		},
		"returns true when membership ops exist": {
			desiredName: "group-a",
			currentName: "group-a",
			ops: []helper.GroupMembershipOperation{
				{UserID: "member-1", IsGroupManager: false},
			},
			want: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := shouldUpdateGroup(test.desiredName, test.currentName, test.ops)
			if got != test.want {
				t.Fatalf("expected %t, got %t", test.want, got)
			}
		})
	}
}

func stringValues(values ...string) []types.String {
	result := make([]types.String, 0, len(values))
	for _, value := range values {
		result = append(result, types.StringValue(value))
	}

	return result
}

func assertGroupMembershipOps(
	t *testing.T,
	got,
	want []helper.GroupMembershipOperation,
) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("expected %d ops, got %d: %#v", len(want), len(got), got)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("op %d: expected %#v, got %#v", i, want[i], got[i])
		}
	}
}

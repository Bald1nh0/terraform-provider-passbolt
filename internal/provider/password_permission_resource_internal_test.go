package provider

import (
	"context"
	"errors"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

func TestPasswordPermissionStringToInt(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		permission string
		want       int
		wantErr    bool
	}{
		"read": {
			permission: "read",
			want:       1,
		},
		"update": {
			permission: "update",
			want:       7,
		},
		"owner": {
			permission: "owner",
			want:       15,
		},
		"delete unsupported": {
			permission: "delete",
			wantErr:    true,
		},
		"empty unsupported": {
			wantErr: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := passwordPermissionStringToInt(test.permission)
			if test.wantErr {
				if !errors.Is(err, errInvalidPermission) {
					t.Fatalf("expected invalid permission error, got %v", err)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != test.want {
				t.Fatalf("expected %d, got %d", test.want, got)
			}
		})
	}
}

func TestPasswordPermissionIntToString(t *testing.T) {
	t.Parallel()

	tests := map[int]string{
		1:  "read",
		7:  "update",
		15: "owner",
		-1: "",
		99: "",
	}

	for permission, want := range tests {
		got := passwordPermissionIntToString(permission)
		if got != want {
			t.Fatalf("expected permission %d to map to %q, got %q", permission, want, got)
		}
	}
}

func TestParsePasswordPermissionImportID(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		value string
		want  passwordPermissionImportID
	}{
		"group": {
			value: "resource-id:group:Developers",
			want: passwordPermissionImportID{
				ResourceID: "resource-id",
				Kind:       passwordPermissionImportKindGroup,
				Name:       "Developers",
			},
		},
		"user": {
			value: "resource-id:user:dev@example.com",
			want: passwordPermissionImportID{
				ResourceID: "resource-id",
				Kind:       passwordPermissionImportKindUser,
				Name:       "dev@example.com",
			},
		},
		"group name with colon": {
			value: "resource-id:group:Dev:Ops",
			want: passwordPermissionImportID{
				ResourceID: "resource-id",
				Kind:       passwordPermissionImportKindGroup,
				Name:       "Dev:Ops",
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := parsePasswordPermissionImportID(test.value)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != test.want {
				t.Fatalf("expected %#v, got %#v", test.want, got)
			}
		})
	}
}

func TestParsePasswordPermissionImportIDRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	tests := []string{
		"",
		"resource-id",
		"resource-id:team:Developers",
		":group:Developers",
		"resource-id:group:",
	}

	for _, value := range tests {
		t.Run(value, func(t *testing.T) {
			t.Parallel()

			if _, err := parsePasswordPermissionImportID(value); err == nil {
				t.Fatalf("expected error for import ID %q", value)
			}
		})
	}
}

func TestPasswordPermissionID(t *testing.T) {
	t.Parallel()

	got := passwordPermissionID("resource-id", passwordPermissionImportKindUser, "dev@example.com")
	want := "resource-id:user:dev@example.com"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestPasswordPermissionIdentityAttributesRequireReplace(t *testing.T) {
	t.Parallel()

	var resp resource.SchemaResponse
	NewPasswordPermissionResource().Schema(context.Background(), resource.SchemaRequest{}, &resp)

	for _, name := range []string{"resource_id", "group_name", "username"} {
		attr, ok := resp.Schema.Attributes[name]
		if !ok {
			t.Fatalf("expected %s attribute in password permission schema", name)
		}

		stringAttr, ok := attr.(schema.StringAttribute)
		if !ok {
			t.Fatalf("expected %s to be a string attribute, got %T", name, attr)
		}

		if len(stringAttr.PlanModifiers) == 0 {
			t.Fatalf("expected %s to require replacement", name)
		}
	}
}

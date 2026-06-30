package provider

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestDesiredMetadataTypeDefaultsToServerDefault(t *testing.T) {
	t.Parallel()

	if got := desiredMetadataType(types.StringNull()); got != metadataTypeServerDefault {
		t.Fatalf("expected null metadata type to default to %q, got %q", metadataTypeServerDefault, got)
	}

	if got := desiredMetadataType(types.StringValue("")); got != metadataTypeServerDefault {
		t.Fatalf("expected empty metadata type to default to %q, got %q", metadataTypeServerDefault, got)
	}
}

func TestActualMetadataTypeFromEncryptedMetadata(t *testing.T) {
	t.Parallel()

	if got := actualMetadataTypeFromEncryptedMetadata(""); got != metadataTypeV4 {
		t.Fatalf("expected empty metadata to be %q, got %q", metadataTypeV4, got)
	}

	if got := actualMetadataTypeFromEncryptedMetadata("-----BEGIN PGP MESSAGE-----"); got != metadataTypeV5 {
		t.Fatalf("expected encrypted metadata to be %q, got %q", metadataTypeV5, got)
	}
}

func TestMetadataTypeActualUsesStateForUnknown(t *testing.T) {
	t.Parallel()

	resources := map[string]resource.Resource{
		"password": NewPasswordResource(),
		"folder":   NewFolderResource(),
	}

	for name, terraformResource := range resources {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var resp resource.SchemaResponse
			terraformResource.Schema(context.Background(), resource.SchemaRequest{}, &resp)

			assertMetadataTypeActualPlanModifier(t, resp.Schema.Attributes)
		})
	}
}

func assertMetadataTypeActualPlanModifier(t *testing.T, attributes map[string]schema.Attribute) {
	t.Helper()

	attr, ok := attributes["metadata_type_actual"]
	if !ok {
		t.Fatal("expected metadata_type_actual attribute")
	}

	stringAttr, ok := attr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("expected metadata_type_actual to be a string attribute, got %T", attr)
	}
	if !stringAttr.Computed {
		t.Fatal("expected metadata_type_actual to be computed")
	}
	if len(stringAttr.PlanModifiers) != 1 {
		t.Fatalf("expected one metadata_type_actual plan modifier, got %d", len(stringAttr.PlanModifiers))
	}

	wantDescription := "Once set, the value of this attribute in state will not change."
	gotDescription := stringAttr.PlanModifiers[0].Description(context.Background())
	if gotDescription != wantDescription {
		t.Fatalf("expected UseStateForUnknown plan modifier, got %q", gotDescription)
	}
}

func TestPassboltFolderMetadataJSONShape(t *testing.T) {
	t.Parallel()

	payload, err := json.Marshal(passboltFolderMetadata{
		ObjectType: passboltFolderMetadataObjectType,
		Name:       "Platform",
	})
	if err != nil {
		t.Fatalf("failed to marshal folder metadata: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("failed to decode folder metadata: %v", err)
	}

	if got := decoded["object_type"]; got != passboltFolderMetadataObjectType {
		t.Fatalf("expected object_type %q, got %#v", passboltFolderMetadataObjectType, got)
	}
	if got := decoded["name"]; got != "Platform" {
		t.Fatalf("expected name %q, got %#v", "Platform", got)
	}
	for _, key := range []string{"color", "description", "icon"} {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("expected key %q to be present", key)
		}
		if decoded[key] != nil {
			t.Fatalf("expected key %q to be null, got %#v", key, decoded[key])
		}
	}
}

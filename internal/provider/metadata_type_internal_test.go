package provider

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
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

			assertMetadataTypeActualConditionalPlanModifier(t, resp.Schema.Attributes)
		})
	}
}

func assertMetadataTypeActualConditionalPlanModifier(t *testing.T, attributes map[string]schema.Attribute) {
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

	wantDescription := "Uses the prior metadata_type_actual value unless metadata_type can change it."
	gotDescription := stringAttr.PlanModifiers[0].Description(context.Background())
	if gotDescription != wantDescription {
		t.Fatalf("expected conditional metadata_type_actual plan modifier, got %q", gotDescription)
	}
}

func TestShouldUseMetadataTypeActualState(t *testing.T) {
	t.Parallel()

	for name, test := range metadataTypeActualStateCases() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := shouldUseMetadataTypeActualState(test.metadataType, test.metadataTypeActual)
			if got != test.want {
				t.Fatalf("expected %t, got %t", test.want, got)
			}
		})
	}
}

type metadataTypeActualStateCase struct {
	metadataType       types.String
	metadataTypeActual types.String
	want               bool
}

func metadataTypeActualStateCases() map[string]metadataTypeActualStateCase {
	return map[string]metadataTypeActualStateCase{
		"unset keeps current actual": {
			metadataType:       types.StringNull(),
			metadataTypeActual: types.StringValue(metadataTypeV4),
			want:               true,
		},
		"empty keeps current actual": {
			metadataType:       types.StringValue(""),
			metadataTypeActual: types.StringValue(metadataTypeV5),
			want:               true,
		},
		"server default keeps current actual": {
			metadataType:       types.StringValue(metadataTypeServerDefault),
			metadataTypeActual: types.StringValue(metadataTypeV4),
			want:               true,
		},
		"same v4 keeps current actual": {
			metadataType:       types.StringValue(metadataTypeV4),
			metadataTypeActual: types.StringValue(metadataTypeV4),
			want:               true,
		},
		"same v5 keeps current actual": {
			metadataType:       types.StringValue(metadataTypeV5),
			metadataTypeActual: types.StringValue(metadataTypeV5),
			want:               true,
		},
		"v4 to v5 upgrade remains unknown": {
			metadataType:       types.StringValue(metadataTypeV5),
			metadataTypeActual: types.StringValue(metadataTypeV4),
			want:               false,
		},
		"v5 to v4 downgrade remains unknown": {
			metadataType:       types.StringValue(metadataTypeV4),
			metadataTypeActual: types.StringValue(metadataTypeV5),
			want:               false,
		},
		"unknown metadata type remains unknown": {
			metadataType:       types.StringUnknown(),
			metadataTypeActual: types.StringValue(metadataTypeV4),
			want:               false,
		},
		"unknown actual remains unknown": {
			metadataType:       types.StringValue(metadataTypeV5),
			metadataTypeActual: types.StringUnknown(),
			want:               false,
		},
		"null actual remains unknown": {
			metadataType:       types.StringValue(metadataTypeV5),
			metadataTypeActual: types.StringNull(),
			want:               false,
		},
	}
}

func TestMetadataTypeActualPlanModifierLeavesChangingActualUnknown(t *testing.T) {
	t.Parallel()

	req := metadataTypeActualPlanModifierRequest(
		types.StringValue(metadataTypeV5),
		types.StringValue(metadataTypeV4),
	)
	resp := &planmodifier.StringResponse{
		PlanValue: req.PlanValue,
	}

	metadataTypeActualUseStateForUnknownModifier{}.PlanModifyString(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}
	if !resp.PlanValue.IsUnknown() {
		t.Fatalf("expected changing metadata_type_actual to remain unknown, got %s", resp.PlanValue)
	}
}

func TestMetadataTypeActualPlanModifierKeepsUnchangedActualKnown(t *testing.T) {
	t.Parallel()

	req := metadataTypeActualPlanModifierRequest(
		types.StringValue(metadataTypeV5),
		types.StringValue(metadataTypeV5),
	)
	resp := &planmodifier.StringResponse{
		PlanValue: req.PlanValue,
	}

	metadataTypeActualUseStateForUnknownModifier{}.PlanModifyString(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}
	if !resp.PlanValue.Equal(types.StringValue(metadataTypeV5)) {
		t.Fatalf("expected unchanged metadata_type_actual to stay known, got %s", resp.PlanValue)
	}
}

func metadataTypeActualPlanModifierRequest(
	metadataType types.String,
	metadataTypeActual types.String,
) planmodifier.StringRequest {
	objectType := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"metadata_type":        tftypes.String,
			"metadata_type_actual": tftypes.String,
		},
	}

	return planmodifier.StringRequest{
		Plan: tfsdk.Plan{
			Raw: tftypes.NewValue(
				objectType,
				map[string]tftypes.Value{
					"metadata_type":        terraformStringValue(metadataType),
					"metadata_type_actual": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
				},
			),
			Schema: metadataTypeActualPlanModifierSchema(),
		},
		State: tfsdk.State{
			Raw: tftypes.NewValue(
				objectType,
				map[string]tftypes.Value{
					"metadata_type":        terraformStringValue(metadataType),
					"metadata_type_actual": terraformStringValue(metadataTypeActual),
				},
			),
			Schema: metadataTypeActualPlanModifierSchema(),
		},
		StateValue:  metadataTypeActual,
		PlanValue:   types.StringUnknown(),
		ConfigValue: types.StringNull(),
	}
}

func metadataTypeActualPlanModifierSchema() schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"metadata_type": schema.StringAttribute{
				Optional: true,
			},
			"metadata_type_actual": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

func terraformStringValue(value types.String) tftypes.Value {
	if value.IsUnknown() {
		return tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	}

	if value.IsNull() {
		return tftypes.NewValue(tftypes.String, nil)
	}

	return tftypes.NewValue(tftypes.String, value.ValueString())
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

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	metadataTypeServerDefault = "server_default"
	metadataTypeV4            = "v4"
	metadataTypeV5            = "v5"
)

func desiredMetadataType(value types.String) string {
	if value.IsUnknown() || value.IsNull() || value.ValueString() == "" {
		return metadataTypeServerDefault
	}

	return value.ValueString()
}

func actualMetadataTypeFromEncryptedMetadata(metadata string) string {
	if metadata != "" {
		return metadataTypeV5
	}

	return metadataTypeV4
}

func metadataTypeActualPlanModifiers() []planmodifier.String {
	return []planmodifier.String{
		metadataTypeActualUseStateForUnknownModifier{},
	}
}

type metadataTypeActualUseStateForUnknownModifier struct{}

func (m metadataTypeActualUseStateForUnknownModifier) Description(_ context.Context) string {
	return "Uses the prior metadata_type_actual value unless metadata_type can change it."
}

func (m metadataTypeActualUseStateForUnknownModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m metadataTypeActualUseStateForUnknownModifier) PlanModifyString(
	ctx context.Context,
	req planmodifier.StringRequest,
	resp *planmodifier.StringResponse,
) {
	if req.State.Raw.IsNull() {
		return
	}

	if !req.PlanValue.IsUnknown() {
		return
	}

	if req.ConfigValue.IsUnknown() {
		return
	}

	var metadataType types.String
	diags := req.Plan.GetAttribute(ctx, path.Root("metadata_type"), &metadataType)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if shouldUseMetadataTypeActualState(metadataType, req.StateValue) {
		resp.PlanValue = req.StateValue
	}
}

func shouldUseMetadataTypeActualState(metadataType types.String, metadataTypeActual types.String) bool {
	if metadataType.IsUnknown() {
		return false
	}

	if metadataType.IsNull() || metadataType.ValueString() == "" {
		return true
	}

	if metadataType.ValueString() == metadataTypeServerDefault {
		return true
	}

	if metadataTypeActual.IsUnknown() || metadataTypeActual.IsNull() {
		return false
	}

	return metadataType.ValueString() == metadataTypeActual.ValueString()
}

package provider

import "github.com/hashicorp/terraform-plugin-framework/types"

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

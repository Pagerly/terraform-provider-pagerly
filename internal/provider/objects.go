package provider

import "github.com/hashicorp/terraform-plugin-framework/types/basetypes"

// basetypesObjectAsOptions lets a null or unknown nested object decode into a zero
// model instead of failing, so Read can start from an empty prior.
var basetypesObjectAsOptions = basetypes.ObjectAsOptions{
	UnhandledNullAsEmpty:    true,
	UnhandledUnknownAsEmpty: true,
}

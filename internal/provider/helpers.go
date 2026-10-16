package provider

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/pagerly/terraform-provider-pagerly/internal/client"
)

// clientFrom pulls the configured client out of a resource Configure request.
func clientFrom(req resource.ConfigureRequest, diags *diag.Diagnostics) *client.Client {
	if req.ProviderData == nil {
		return nil
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		diags.AddError("Unexpected provider data", "The provider did not hand this resource a Pagerly client.")
		return nil
	}
	return c
}

func clientFromDataSource(req datasource.ConfigureRequest, diags *diag.Diagnostics) *client.Client {
	if req.ProviderData == nil {
		return nil
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		diags.AddError("Unexpected provider data", "The provider did not hand this data source a Pagerly client.")
		return nil
	}
	return c
}

// strValue turns an API string into a Terraform value, null when empty.
func strValue(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

// strPtr turns a Terraform string into a pointer for a JSON body: nil when unset.
func strPtr(v types.String) *string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	s := v.ValueString()
	return &s
}

// boolPtr turns a Terraform bool into a pointer for a JSON body: nil when unset.
func boolPtr(v types.Bool) *bool {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	b := v.ValueBool()
	return &b
}

// keepFold returns the prior value when it only differs from the API's by case,
// so "monday" in configuration does not show as drift against "Monday".
func keepFold(prior types.String, api string) types.String {
	if api == "" {
		return types.StringNull()
	}
	if !prior.IsNull() && !prior.IsUnknown() && strings.EqualFold(prior.ValueString(), api) {
		return prior
	}
	return types.StringValue(api)
}

// stringList reads a Terraform list of strings.
func stringList(ctx context.Context, v types.List) ([]string, diag.Diagnostics) {
	var out []string
	if v.IsNull() || v.IsUnknown() {
		return out, nil
	}
	diags := v.ElementsAs(ctx, &out, false)
	return out, diags
}

// listFold builds a list value from the API's strings, keeping each prior element
// that only differs by case (emails are returned lowercased).
func listFold(prior types.List, api []string) types.List {
	var priorValues []string
	if !prior.IsNull() && !prior.IsUnknown() {
		for _, e := range prior.Elements() {
			if s, ok := e.(types.String); ok && !s.IsNull() && !s.IsUnknown() {
				priorValues = append(priorValues, s.ValueString())
			}
		}
	}
	elements := make([]attr.Value, 0, len(api))
	for i, s := range api {
		if i < len(priorValues) && strings.EqualFold(priorValues[i], s) {
			elements = append(elements, types.StringValue(priorValues[i]))
		} else {
			elements = append(elements, types.StringValue(s))
		}
	}
	list, _ := types.ListValue(types.StringType, elements)
	return list
}

// emptyListIfNull makes an optional list attribute concrete, for computed reads.
func emptyListIfNull(api []string) []string {
	if api == nil {
		return []string{}
	}
	return api
}

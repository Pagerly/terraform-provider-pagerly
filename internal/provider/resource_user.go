package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/pagerly/terraform-provider-pagerly/internal/client"
)

var (
	_ resource.Resource                = &userResource{}
	_ resource.ResourceWithConfigure   = &userResource{}
	_ resource.ResourceWithImportState = &userResource{}
)

func newUserResource() resource.Resource { return &userResource{} }

type userResource struct {
	client *client.Client
}

type userModel struct {
	ID          types.String `tfsdk:"id"`
	Email       types.String `tfsdk:"email"`
	Name        types.String `tfsdk:"name"`
	PhoneNumber types.String `tfsdk:"phone_number"`
	CountryCode types.String `tfsdk:"country_code"`
	Role        types.String `tfsdk:"role"`
}

func (r *userResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (r *userResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A person in the Pagerly user directory: who can be paged, and the number Pagerly dials. " +
			"Creating a user that already exists adopts it.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Same as the email.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"email": schema.StringAttribute{
				Description:   "Email address, the directory key. Changing it recreates the user.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name":         optionalComputedString("Display name."),
			"phone_number": optionalComputedString("Phone number, in E.164 (`+4420…`) or bare with `country_code`."),
			"country_code": optionalComputedString("Country calling code, e.g. `+44`, for a bare phone number."),
			"role":         optionalComputedString("Directory role, e.g. `member` or `admin`."),
		},
	}
}

func (r *userResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFrom(req, &resp.Diagnostics)
}

func (r *userResource) body(plan *userModel) map[string]any {
	return map[string]any{
		"email":        plan.Email.ValueString(),
		"name":         strPtr(plan.Name),
		"phone_number": strPtr(plan.PhoneNumber),
		"country_code": strPtr(plan.CountryCode),
		"role":         strPtr(plan.Role),
	}
}

func (r *userResource) fill(m *userModel, u *client.User) {
	m.ID = keepFold(m.ID, u.Email)
	m.Email = keepFold(m.Email, u.Email)
	m.Name = strValue(u.Name)
	m.PhoneNumber = strValue(u.PhoneNumber)
	m.CountryCode = strValue(u.CountryCode)
	m.Role = strValue(u.Role)
}

func (r *userResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan userModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out client.User
	if err := r.client.Post(ctx, "/users", r.body(&plan), &out); err != nil {
		resp.Diagnostics.AddError("Could not create user", err.Error())
		return
	}
	r.fill(&plan, &out)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *userResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state userModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out client.User
	err := r.client.Get(ctx, "/users/"+client.PathEscape(state.ID.ValueString()), &out)
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Could not read user", err.Error())
		return
	}
	r.fill(&state, &out)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *userResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state userModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out client.User
	if err := r.client.Put(ctx, "/users/"+client.PathEscape(state.ID.ValueString()), r.body(&plan), &out); err != nil {
		resp.Diagnostics.AddError("Could not update user", err.Error())
		return
	}
	plan.ID = state.ID
	r.fill(&plan, &out)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *userResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state userModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, "/users/"+client.PathEscape(state.ID.ValueString())); err != nil {
		resp.Diagnostics.AddError("Could not delete user", err.Error())
	}
}

func (r *userResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

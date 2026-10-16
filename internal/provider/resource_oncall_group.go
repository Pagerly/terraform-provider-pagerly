package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/pagerly/terraform-provider-pagerly/internal/client"
)

var (
	_ resource.Resource                = &oncallGroupResource{}
	_ resource.ResourceWithConfigure   = &oncallGroupResource{}
	_ resource.ResourceWithImportState = &oncallGroupResource{}
)

func newOncallGroupResource() resource.Resource { return &oncallGroupResource{} }

type oncallGroupResource struct {
	client *client.Client
}

type oncallGroupModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	TeamIDs        types.List   `tfsdk:"team_ids"`
	UserGroup      types.String `tfsdk:"user_group"`
	PermanentUsers types.List   `tfsdk:"permanent_users"`
}

func (r *oncallGroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_oncall_group"
}

func (r *oncallGroupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A group on-call (multi-mention team): one name that stands for the on-call of several teams, " +
			"optionally keeping a Slack user group in step and always including some people.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Same as the name, so it changes when the group is renamed.",
				Computed:    true,
			},
			"name": schema.StringAttribute{
				Description: "Group name. Cannot contain `:`, `;` or `,`.",
				Required:    true,
			},
			"team_ids": schema.ListAttribute{
				Description: "Teams whose on-call the group stands for.",
				Required:    true,
				ElementType: types.StringType,
				Validators:  []validator.List{listvalidator.SizeAtLeast(1)},
			},
			"user_group": schema.StringAttribute{
				Description: "Slack user group id (`S…`) kept in step with the group's on-call.",
				Optional:    true,
			},
			"permanent_users": schema.ListAttribute{
				Description: "Emails of people always included, whoever is on call.",
				Optional:    true,
				ElementType: types.StringType,
			},
		},
	}
}

func (r *oncallGroupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFrom(req, &resp.Diagnostics)
}

func (r *oncallGroupResource) body(ctx context.Context, plan *oncallGroupModel) map[string]any {
	teams, _ := stringList(ctx, plan.TeamIDs)
	people, _ := stringList(ctx, plan.PermanentUsers)
	if people == nil {
		people = []string{}
	}
	return map[string]any{
		"name":            plan.Name.ValueString(),
		"team_ids":        teams,
		"user_group":      strPtr(plan.UserGroup),
		"permanent_users": people,
	}
}

func (r *oncallGroupResource) fill(m *oncallGroupModel, g *client.OncallGroup) {
	m.ID = types.StringValue(g.Name)
	m.Name = types.StringValue(g.Name)
	m.TeamIDs = listFold(m.TeamIDs, emptyListIfNull(g.TeamIDs))
	m.UserGroup = strValue(g.UserGroup)
	if len(g.PermanentUsers) == 0 && (m.PermanentUsers.IsNull() || m.PermanentUsers.IsUnknown()) {
		m.PermanentUsers = types.ListNull(types.StringType)
	} else {
		m.PermanentUsers = listFold(m.PermanentUsers, emptyListIfNull(g.PermanentUsers))
	}
}

func (r *oncallGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan oncallGroupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out client.OncallGroup
	if err := r.client.Post(ctx, "/oncall-groups", r.body(ctx, &plan), &out); err != nil {
		resp.Diagnostics.AddError("Could not create group on-call", err.Error())
		return
	}
	r.fill(&plan, &out)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *oncallGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state oncallGroupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out client.OncallGroup
	err := r.client.Get(ctx, "/oncall-groups/"+client.PathEscape(state.ID.ValueString()), &out)
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Could not read group on-call", err.Error())
		return
	}
	r.fill(&state, &out)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *oncallGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state oncallGroupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out client.OncallGroup
	if err := r.client.Put(ctx, "/oncall-groups/"+client.PathEscape(state.ID.ValueString()), r.body(ctx, &plan), &out); err != nil {
		resp.Diagnostics.AddError("Could not update group on-call", err.Error())
		return
	}
	r.fill(&plan, &out)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *oncallGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state oncallGroupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, "/oncall-groups/"+client.PathEscape(state.ID.ValueString())); err != nil {
		resp.Diagnostics.AddError("Could not delete group on-call", err.Error())
	}
}

func (r *oncallGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/pagerly/terraform-provider-pagerly/internal/client"
)

var (
	_ resource.Resource                = &escalationPolicyResource{}
	_ resource.ResourceWithConfigure   = &escalationPolicyResource{}
	_ resource.ResourceWithImportState = &escalationPolicyResource{}
)

func newEscalationPolicyResource() resource.Resource { return &escalationPolicyResource{} }

type escalationPolicyResource struct {
	client *client.Client
}

type escalationPolicyModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Layers       types.List   `tfsdk:"layer"`
	Repeat       types.Int64  `tfsdk:"repeat"`
	SlackChannel types.String `tfsdk:"slack_channel"`
}

type layerModel struct {
	TeamID      types.String `tfsdk:"team_id"`
	People      types.List   `tfsdk:"people"`
	WaitMinutes types.Int64  `tfsdk:"wait_minutes"`
}

var layerAttrTypes = map[string]attr.Type{
	"team_id":      types.StringType,
	"people":       types.ListType{ElemType: types.StringType},
	"wait_minutes": types.Int64Type,
}

func (r *escalationPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_escalation_policy"
}

func (r *escalationPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "An escalation policy: up to three layers, each paging a team's on-call and/or named people, " +
			"with how long to wait for an acknowledgement before the next layer.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Description: "Policy name.",
				Required:    true,
			},
			"layer": schema.ListNestedAttribute{
				Description: "Layers in the order they are paged. A layer needs a team, people, or both.",
				Required:    true,
				Validators:  []validator.List{listvalidator.SizeBetween(1, 3)},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"team_id": schema.StringAttribute{
							Description: "Page whoever is on call for this team.",
							Optional:    true,
						},
						"people": schema.ListAttribute{
							Description: "Emails of people to page directly. They must be in the Pagerly user directory.",
							Optional:    true,
							ElementType: types.StringType,
						},
						"wait_minutes": schema.Int64Attribute{
							Description: "Minutes to wait for an acknowledgement before the next layer. Layer 1 defaults to 5.",
							Optional:    true,
							Computed:    true,
							Validators:  []validator.Int64{int64validator.AtLeast(1)},
							PlanModifiers: []planmodifier.Int64{
								int64planmodifier.UseStateForUnknown(),
							},
						},
					},
				},
			},
			"repeat": schema.Int64Attribute{
				Description: "Times to start again from layer 1 when nobody acknowledges (0–9).",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(0),
				Validators:  []validator.Int64{int64validator.Between(0, 9)},
			},
			"slack_channel": schema.StringAttribute{
				Description: "Slack channel id where incidents routed to this policy by an alert integration are posted.",
				Optional:    true,
			},
		},
	}
}

func (r *escalationPolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFrom(req, &resp.Diagnostics)
}

func (r *escalationPolicyResource) body(ctx context.Context, plan *escalationPolicyModel) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	var layers []layerModel
	diags.Append(plan.Layers.ElementsAs(ctx, &layers, false)...)
	out := make([]map[string]any, 0, len(layers))
	for _, l := range layers {
		layer := map[string]any{}
		if p := strPtr(l.TeamID); p != nil {
			layer["team_id"] = *p
		}
		people, d := stringList(ctx, l.People)
		diags.Append(d...)
		if people != nil {
			layer["people"] = people
		}
		if !l.WaitMinutes.IsNull() && !l.WaitMinutes.IsUnknown() {
			layer["wait_minutes"] = l.WaitMinutes.ValueInt64()
		}
		out = append(out, layer)
	}
	body := map[string]any{
		"name":          plan.Name.ValueString(),
		"layers":        out,
		"repeat":        plan.Repeat.ValueInt64(),
		"slack_channel": strPtr(plan.SlackChannel),
	}
	return body, diags
}

func (r *escalationPolicyResource) fill(ctx context.Context, m *escalationPolicyModel, p *client.EscalationPolicy) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.StringValue(p.ID)
	m.Name = types.StringValue(p.Name)
	m.Repeat = types.Int64Value(p.Repeat)
	m.SlackChannel = strValue(p.SlackChannel)

	var prior []layerModel
	if !m.Layers.IsNull() && !m.Layers.IsUnknown() {
		diags.Append(m.Layers.ElementsAs(ctx, &prior, false)...)
	}
	values := make([]attr.Value, 0, len(p.Layers))
	for i, l := range p.Layers {
		var before layerModel
		if i < len(prior) {
			before = prior[i]
		}
		people := types.ListNull(types.StringType)
		if len(l.People) > 0 || (!before.People.IsNull() && !before.People.IsUnknown()) {
			people = listFold(before.People, emptyListIfNull(l.People))
		}
		wait := types.Int64Null()
		if l.WaitMinutes != nil {
			wait = types.Int64Value(*l.WaitMinutes)
		}
		obj, d := types.ObjectValueFrom(ctx, layerAttrTypes, layerModel{
			TeamID:      strValue(l.TeamID),
			People:      people,
			WaitMinutes: wait,
		})
		diags.Append(d...)
		values = append(values, obj)
	}
	list, d := types.ListValue(types.ObjectType{AttrTypes: layerAttrTypes}, values)
	diags.Append(d...)
	m.Layers = list
	return diags
}

func (r *escalationPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan escalationPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body, diags := r.body(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	var policy client.EscalationPolicy
	if err := r.client.Post(ctx, "/escalation-policies", body, &policy); err != nil {
		resp.Diagnostics.AddError("Could not create escalation policy", err.Error())
		return
	}
	resp.Diagnostics.Append(r.fill(ctx, &plan, &policy)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *escalationPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state escalationPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var policy client.EscalationPolicy
	err := r.client.Get(ctx, "/escalation-policies/"+client.PathEscape(state.ID.ValueString()), &policy)
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Could not read escalation policy", err.Error())
		return
	}
	resp.Diagnostics.Append(r.fill(ctx, &state, &policy)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *escalationPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state escalationPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body, diags := r.body(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	var policy client.EscalationPolicy
	if err := r.client.Put(ctx, "/escalation-policies/"+client.PathEscape(state.ID.ValueString()), body, &policy); err != nil {
		resp.Diagnostics.AddError("Could not update escalation policy", err.Error())
		return
	}
	plan.ID = state.ID
	resp.Diagnostics.Append(r.fill(ctx, &plan, &policy)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *escalationPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state escalationPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, "/escalation-policies/"+client.PathEscape(state.ID.ValueString())); err != nil {
		resp.Diagnostics.AddError("Could not delete escalation policy", err.Error())
	}
}

func (r *escalationPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

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
	_ resource.Resource                = &serviceResource{}
	_ resource.ResourceWithConfigure   = &serviceResource{}
	_ resource.ResourceWithImportState = &serviceResource{}
)

func newServiceResource() resource.Resource { return &serviceResource{} }

type serviceResource struct {
	client *client.Client
}

type serviceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	TeamID       types.String `tfsdk:"team_id"`
	EscalationID types.String `tfsdk:"escalation_id"`
	Status       types.String `tfsdk:"status"`
}

func (r *serviceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service"
}

func (r *serviceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A service (component) incidents are raised against, with the team and escalation policy that own it.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Description: "Service name.",
				Required:    true,
			},
			"team_id": schema.StringAttribute{
				Description: "Owning team.",
				Optional:    true,
			},
			"escalation_id": schema.StringAttribute{
				Description: "Escalation policy for the service's incidents.",
				Optional:    true,
			},
			"status": schema.StringAttribute{
				Description: "Free-form status label shown in the dashboard.",
				Optional:    true,
			},
		},
	}
}

func (r *serviceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFrom(req, &resp.Diagnostics)
}

func (r *serviceResource) body(plan *serviceModel) map[string]any {
	return map[string]any{
		"name":          plan.Name.ValueString(),
		"team_id":       strPtr(plan.TeamID),
		"escalation_id": strPtr(plan.EscalationID),
		"status":        strPtr(plan.Status),
	}
}

func (r *serviceResource) fill(m *serviceModel, s *client.Service) {
	m.ID = types.StringValue(s.ID)
	m.Name = types.StringValue(s.Name)
	m.TeamID = strValue(s.TeamID)
	m.EscalationID = strValue(s.EscalationID)
	m.Status = strValue(s.Status)
}

func (r *serviceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan serviceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out client.Service
	if err := r.client.Post(ctx, "/services", r.body(&plan), &out); err != nil {
		resp.Diagnostics.AddError("Could not create service", err.Error())
		return
	}
	r.fill(&plan, &out)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *serviceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state serviceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out client.Service
	err := r.client.Get(ctx, "/services/"+client.PathEscape(state.ID.ValueString()), &out)
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Could not read service", err.Error())
		return
	}
	r.fill(&state, &out)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *serviceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state serviceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out client.Service
	if err := r.client.Put(ctx, "/services/"+client.PathEscape(state.ID.ValueString()), r.body(&plan), &out); err != nil {
		resp.Diagnostics.AddError("Could not update service", err.Error())
		return
	}
	r.fill(&plan, &out)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *serviceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state serviceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, "/services/"+client.PathEscape(state.ID.ValueString())); err != nil {
		resp.Diagnostics.AddError("Could not delete service", err.Error())
	}
}

func (r *serviceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

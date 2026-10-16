package provider

import (
	"context"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/pagerly/terraform-provider-pagerly/internal/client"
)

// ── pagerly_team ───────────────────────────────────────────────────────────────

var _ datasource.DataSource = &teamDataSource{}

func newTeamDataSource() datasource.DataSource { return &teamDataSource{} }

type teamDataSource struct {
	client *client.Client
}

type teamDataModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	Timezone      types.String `tfsdk:"timezone"`
	SlackChannel  types.String `tfsdk:"slack_channel"`
	CurrentOncall types.String `tfsdk:"current_oncall"`
}

func (d *teamDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_team"
}

func (d *teamDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Look up an existing team by id or name.",
		Attributes: map[string]schema.Attribute{
			"id":             schema.StringAttribute{Description: "Team id. One of `id` or `name` is required.", Optional: true, Computed: true},
			"name":           schema.StringAttribute{Description: "Team name, case-insensitive.", Optional: true, Computed: true},
			"timezone":       schema.StringAttribute{Computed: true},
			"slack_channel":  schema.StringAttribute{Computed: true},
			"current_oncall": schema.StringAttribute{Description: "Email of whoever is on call now.", Computed: true},
		},
	}
}

func (d *teamDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, &resp.Diagnostics)
}

func (d *teamDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg teamDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var team client.Team
	switch {
	case !cfg.ID.IsNull() && cfg.ID.ValueString() != "":
		if err := d.client.Get(ctx, "/teams/"+client.PathEscape(cfg.ID.ValueString()), &team); err != nil {
			resp.Diagnostics.AddError("Could not read team", err.Error())
			return
		}
	case !cfg.Name.IsNull() && cfg.Name.ValueString() != "":
		var teams []client.Team
		q := url.Values{"name": {cfg.Name.ValueString()}}
		if err := d.client.Do(ctx, http.MethodGet, "/teams", q, nil, &teams); err != nil {
			resp.Diagnostics.AddError("Could not list teams", err.Error())
			return
		}
		if len(teams) == 0 {
			resp.Diagnostics.AddError("Team not found", "No team named "+cfg.Name.ValueString())
			return
		}
		team = teams[0]
	default:
		resp.Diagnostics.AddError("Missing lookup key", "Set id or name on data.pagerly_team.")
		return
	}
	cfg.ID = types.StringValue(team.ID)
	cfg.Name = types.StringValue(team.Name)
	cfg.Timezone = strValue(team.Timezone)
	cfg.SlackChannel = strValue(team.SlackChannel)
	cfg.CurrentOncall = strValue(team.CurrentOncall)
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}

// ── pagerly_oncall ─────────────────────────────────────────────────────────────

var _ datasource.DataSource = &oncallDataSource{}

func newOncallDataSource() datasource.DataSource { return &oncallDataSource{} }

type oncallDataSource struct {
	client *client.Client
}

type oncallEntryModel struct {
	TeamID   types.String `tfsdk:"team_id"`
	TeamName types.String `tfsdk:"team_name"`
	Email    types.String `tfsdk:"email"`
	Name     types.String `tfsdk:"name"`
}

type oncallDataModel struct {
	TeamID types.String       `tfsdk:"team_id"`
	Oncall []oncallEntryModel `tfsdk:"oncall"`
}

func (d *oncallDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_oncall"
}

func (d *oncallDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Who is on call right now, for one team or every team. Read-only; useful for wiring the current " +
			"on-call into other providers.",
		Attributes: map[string]schema.Attribute{
			"team_id": schema.StringAttribute{Description: "Limit to one team.", Optional: true},
			"oncall": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"team_id":   schema.StringAttribute{Computed: true},
						"team_name": schema.StringAttribute{Computed: true},
						"email":     schema.StringAttribute{Computed: true},
						"name":      schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *oncallDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, &resp.Diagnostics)
}

func (d *oncallDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg oncallDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	q := url.Values{}
	if !cfg.TeamID.IsNull() && cfg.TeamID.ValueString() != "" {
		q.Set("team_id", cfg.TeamID.ValueString())
	}
	var out struct {
		Oncall []struct {
			TeamID   string `json:"teamId"`
			TeamName string `json:"teamName"`
			Oncall   *struct {
				Email string `json:"email"`
				Name  string `json:"name"`
			} `json:"oncall"`
		} `json:"oncall"`
	}
	if err := d.client.Do(ctx, http.MethodGet, "/oncall", q, nil, &out); err != nil {
		resp.Diagnostics.AddError("Could not read on-call", err.Error())
		return
	}
	cfg.Oncall = make([]oncallEntryModel, 0, len(out.Oncall))
	for _, e := range out.Oncall {
		entry := oncallEntryModel{
			TeamID:   strValue(e.TeamID),
			TeamName: strValue(e.TeamName),
			Email:    types.StringNull(),
			Name:     types.StringNull(),
		}
		if e.Oncall != nil {
			entry.Email = strValue(e.Oncall.Email)
			entry.Name = strValue(e.Oncall.Name)
		}
		cfg.Oncall = append(cfg.Oncall, entry)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}

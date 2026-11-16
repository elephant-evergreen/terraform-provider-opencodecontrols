package provider

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type apiKeysDataSource struct{ api *client }

type apiKeysModel struct {
	AccountID types.String       `tfsdk:"service_account_id"`
	Keys      []keyMetadataModel `tfsdk:"keys"`
}

type keyMetadataModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Permissions types.String `tfsdk:"permissions"`
	Scopes      types.Set    `tfsdk:"scopes"`
	Status      types.String `tfsdk:"status"`
	ExpiresAt   types.String `tfsdk:"expires_at"`
}

var _ datasource.DataSourceWithConfigure = (*apiKeysDataSource)(nil)

func newAPIKeysDataSource() datasource.DataSource { return &apiKeysDataSource{} }

func (*apiKeysDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_account_api_keys"
}

func (*apiKeysDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "List the API key metadata returned by OpenCode Console for a service account.",
		Attributes: map[string]schema.Attribute{
			"service_account_id": schema.StringAttribute{Required: true, MarkdownDescription: "OpenCode Console service account ID, beginning with svcacct_."},
			"keys": schema.ListNestedAttribute{Computed: true, MarkdownDescription: "Key metadata, including statuses. Bearer tokens are never returned.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"id":          schema.StringAttribute{Computed: true, MarkdownDescription: "API key ID."},
					"name":        schema.StringAttribute{Computed: true, MarkdownDescription: "API key name."},
					"permissions": schema.StringAttribute{Computed: true, MarkdownDescription: "API key permission level."},
					"scopes":      schema.SetAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Assigned permission scopes."},
					"status":      schema.StringAttribute{Computed: true, MarkdownDescription: "API key status."},
					"expires_at":  schema.StringAttribute{Computed: true, MarkdownDescription: "API key expiration as an RFC3339 timestamp, if set."},
				}},
			},
		},
	}
}

func (d *apiKeysDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	api, ok := req.ProviderData.(*client)
	if !ok {
		resp.Diagnostics.AddError("Invalid provider configuration", "The OpenCode Console client was not configured.")
		return
	}
	d.api = api
}

func (d *apiKeysDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state apiKeysModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	keys, err := d.api.keys(ctx, state.AccountID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Could not read service account API keys", err.Error())
		return
	}
	state.Keys = make([]keyMetadataModel, 0, len(keys))
	for _, key := range keys {
		expiration := types.StringNull()
		if key.ExpiresAt != nil {
			expiration = types.StringValue(key.ExpiresAt.UTC().Format(time.RFC3339Nano))
		}
		scopes := key.Scopes
		if scopes == nil {
			scopes = []string{}
		}
		values, diags := types.SetValueFrom(ctx, types.StringType, scopes)
		resp.Diagnostics.Append(diags...)
		state.Keys = append(state.Keys, keyMetadataModel{
			ID: types.StringValue(key.ID), Name: types.StringValue(key.Name),
			Permissions: types.StringValue(key.Permissions), Scopes: values,
			Status: types.StringValue(key.Status), ExpiresAt: expiration,
		})
	}
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	}
}

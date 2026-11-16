package provider

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type apiKeyResource struct{ api *client }

type apiKeyModel struct {
	AccountID   types.String `tfsdk:"service_account_id"`
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Permissions types.String `tfsdk:"permissions"`
	Scopes      types.Set    `tfsdk:"scopes"`
	ExpiresAt   types.String `tfsdk:"expires_at"`
	Status      types.String `tfsdk:"status"`
	Value       types.String `tfsdk:"value"`
}

var (
	_ resource.ResourceWithConfigure      = (*apiKeyResource)(nil)
	_ resource.ResourceWithImportState    = (*apiKeyResource)(nil)
	_ resource.ResourceWithValidateConfig = (*apiKeyResource)(nil)
)

func newAPIKeyResource() resource.Resource { return &apiKeyResource{} }

func (*apiKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_account_api_key"
}

func (*apiKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	retain := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage one OpenCode Console service account API key. Terraform stores the one-time value for newly issued keys in state. Import brings existing key metadata under management without rotating the key.",
		Attributes: map[string]schema.Attribute{
			"service_account_id": schema.StringAttribute{Required: true, PlanModifiers: replace, MarkdownDescription: "OpenCode Console service account ID, beginning with svcacct_."},
			"id":                 schema.StringAttribute{Computed: true, PlanModifiers: retain, MarkdownDescription: "OpenCode Console API key ID, beginning with sk_."},
			"name":               schema.StringAttribute{Required: true, PlanModifiers: replace, MarkdownDescription: "Name shown in the service account's credentials list. Changing this replaces the key."},
			"permissions":        schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Permission level. New keys must be restricted (the default). Import can preserve all or read-only keys."},
			"scopes":             schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType, PlanModifiers: []planmodifier.Set{setplanmodifier.RequiresReplace(), setplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Permission scopes, such as models:read. Specify at least one scope when creating a key. Changing this replaces the key."},
			"expires_at":         schema.StringAttribute{Optional: true, PlanModifiers: replace, MarkdownDescription: "Expiration as an RFC3339 timestamp. Omit for a key without expiration. Changing this replaces the key."},
			"status":             schema.StringAttribute{Computed: true, MarkdownDescription: "Status reported by OpenCode Console."},
			"value":              schema.StringAttribute{Computed: true, Sensitive: true, PlanModifiers: retain, MarkdownDescription: "Bearer token returned at creation. Terraform stores it in state. Imported keys have no value in state."},
		},
	}
}

func (r *apiKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	api, ok := req.ProviderData.(*client)
	if !ok {
		resp.Diagnostics.AddError("Invalid provider configuration", "The OpenCode Console client was not configured.")
		return
	}
	r.api = api
}

func (*apiKeyResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config apiKeyModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !config.AccountID.IsUnknown() && !accountPattern.MatchString(config.AccountID.ValueString()) {
		resp.Diagnostics.AddAttributeError(path.Root("service_account_id"), "Invalid service account ID", "Use an OpenCode Console service account ID beginning with svcacct_.")
	}
	if !config.Name.IsUnknown() && strings.TrimSpace(config.Name.ValueString()) == "" {
		resp.Diagnostics.AddAttributeError(path.Root("name"), "Invalid API key name", "Use a nonempty name.")
	}
	if !config.ExpiresAt.IsNull() && !config.ExpiresAt.IsUnknown() {
		if _, err := time.Parse(time.RFC3339, config.ExpiresAt.ValueString()); err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("expires_at"), "Invalid expiration", "Use an RFC3339 timestamp.")
		}
	}
}

func (*apiKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	accountID, keyID, ok := strings.Cut(req.ID, "/")
	if !ok || !accountPattern.MatchString(accountID) || !keyIDPattern.MatchString(keyID) {
		resp.Diagnostics.AddError("Invalid import ID", "Use SERVICE_ACCOUNT_ID/API_KEY_ID.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("service_account_id"), accountID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), keyID)...)
}

func (r *apiKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan apiKeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if permission := plan.Permissions.ValueString(); permission != "" && permission != "restricted" {
		resp.Diagnostics.AddAttributeError(path.Root("permissions"), "Unsupported permission level", "OpenCode Console only issues restricted keys through this API. Existing keys with other permission levels can be imported.")
		return
	}
	input := createKeyRequest{Name: plan.Name.ValueString(), Permissions: "restricted"}
	resp.Diagnostics.Append(plan.Scopes.ElementsAs(ctx, &input.Scopes, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if len(input.Scopes) == 0 {
		resp.Diagnostics.AddAttributeError(path.Root("scopes"), "Missing API key scopes", "Specify the scopes for the restricted key.")
		return
	}
	for _, scope := range input.Scopes {
		if strings.TrimSpace(scope) == "" {
			resp.Diagnostics.AddAttributeError(path.Root("scopes"), "Invalid API key scope", "Each scope must be nonempty.")
			return
		}
	}
	if !plan.ExpiresAt.IsNull() {
		expiration, err := time.Parse(time.RFC3339, plan.ExpiresAt.ValueString())
		if err != nil || !expiration.After(time.Now()) {
			resp.Diagnostics.AddAttributeError(path.Root("expires_at"), "Invalid expiration", "Use a future RFC3339 timestamp.")
			return
		}
		input.ExpiresAt = &expiration
	}
	id, value, err := r.api.create(ctx, plan.AccountID.ValueString(), input)
	if err != nil {
		resp.Diagnostics.AddError("Could not issue API key", err.Error())
		return
	}
	plan.ID = types.StringValue(id)
	plan.Value = types.StringValue(value)
	plan.Permissions = types.StringValue(input.Permissions)
	plan.Status = types.StringValue("active")
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *apiKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state apiKeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	keys, err := r.api.keys(ctx, state.AccountID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Could not read service account API keys", err.Error())
		return
	}
	found := findKey(keys, state.ID.ValueString())
	if found == nil || found.Status == "revoked" {
		resp.State.RemoveResource(ctx)
		return
	}
	if r.api.isManagementKey(*found) {
		resp.Diagnostics.AddError("Cannot manage the authorization key", "Use a separate API key to manage other keys for the service account.")
		return
	}
	if err := readKeyRecord(ctx, &state, *found, &resp.Diagnostics); err != nil {
		resp.Diagnostics.AddError("Could not read API key metadata", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func readKeyRecord(ctx context.Context, state *apiKeyModel, key keyRecord, diags *diag.Diagnostics) error {
	if key.Name == "" || key.Permissions == "" || key.Status == "" {
		return fmt.Errorf("OpenCode Console did not return the key's name, permissions, and status.")
	}
	state.Name = types.StringValue(key.Name)
	state.Permissions = types.StringValue(key.Permissions)
	state.Status = types.StringValue(key.Status)
	scopes := key.Scopes
	if scopes == nil {
		scopes = []string{}
	}
	remoteScopes, scopedDiags := types.SetValueFrom(ctx, types.StringType, scopes)
	diags.Append(scopedDiags...)
	state.Scopes = remoteScopes
	if key.ExpiresAt == nil {
		state.ExpiresAt = types.StringNull()
	} else if local, err := time.Parse(time.RFC3339, state.ExpiresAt.ValueString()); err != nil || !local.Equal(*key.ExpiresAt) {
		state.ExpiresAt = types.StringValue(key.ExpiresAt.UTC().Format(time.RFC3339Nano))
	}
	return nil
}

func (*apiKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("API keys cannot be edited in place", "Review the proposed operation. Configurable fields require replacing the key.")
}

func (r *apiKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state apiKeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	keys, err := r.api.keys(ctx, state.AccountID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Could not read service account API keys", err.Error())
		return
	}
	found := findKey(keys, state.ID.ValueString())
	if found == nil || found.Status == "revoked" {
		return
	}
	if r.api.isManagementKey(*found) {
		resp.Diagnostics.AddError("Cannot revoke the authorization key", "Use a separate API key to manage other keys for the service account.")
		return
	}
	if err := r.api.revoke(ctx, found.ID); err != nil && !apiStatus(err, http.StatusNotFound) {
		resp.Diagnostics.AddError("Could not revoke API key", err.Error())
	}
}

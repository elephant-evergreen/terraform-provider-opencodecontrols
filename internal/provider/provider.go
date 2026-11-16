package provider

import (
	"context"
	"net/http"
	"os"
	"regexp"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const endpoint = "https://opencode.ai"

var tokenPattern = regexp.MustCompile("^oc_sk_[A-Za-z0-9_-]+$")

type controlsProvider struct {
	version string
	origin  string
}

type providerModel struct {
	APIKey types.String `tfsdk:"api_key"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider { return &controlsProvider{version: version, origin: endpoint} }
}

func (p *controlsProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "opencodecontrols"
	resp.Version = p.version
}

func (*controlsProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage OpenCode Console service account API keys.",
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{Optional: true, Sensitive: true, MarkdownDescription: "OpenCode Console service account management key. Defaults to the OPENCODE_CONTROLS_API_KEY environment variable."},
		},
	}
}

func (p *controlsProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if config.APIKey.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("api_key"), "Unknown API key", "The provider API key must be available when planning.")
		return
	}
	key := config.APIKey.ValueString()
	if config.APIKey.IsNull() {
		key = os.Getenv("OPENCODE_CONTROLS_API_KEY")
	}
	if !tokenPattern.MatchString(key) {
		resp.Diagnostics.AddAttributeError(path.Root("api_key"), "Missing or invalid API key", "Set api_key or OPENCODE_CONTROLS_API_KEY to an OpenCode Console service account management key beginning with oc_sk_.")
		return
	}
	c := &client{
		origin:  p.origin,
		key:     key,
		version: p.version,
		http: &http.Client{
			Timeout:       20 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
	resp.ResourceData = c
	resp.DataSourceData = c
}

func (*controlsProvider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{newAPIKeyResource}
}

func (*controlsProvider) DataSources(context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{newAPIKeysDataSource}
}

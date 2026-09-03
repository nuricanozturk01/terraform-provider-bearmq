// Package provider implements the Terraform provider for BearMQ.
package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nuricanozturk01/terraform-provider-bearmq/internal/client"
)

// Ensure the implementation satisfies the expected interface.
var _ provider.Provider = (*bearmqProvider)(nil)

const defaultEndpoint = "http://localhost:3333"

type bearmqProvider struct {
	version string
}

// New returns a provider factory for the given build version.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &bearmqProvider{version: version}
	}
}

type providerModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	APIKey   types.String `tfsdk:"api_key"`
	Token    types.String `tfsdk:"token"`
	Insecure types.Bool   `tfsdk:"insecure"`
}

func (p *bearmqProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "bearmq"
	resp.Version = p.version
}

func (p *bearmqProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage BearMQ virtual hosts, exchanges, queues and bindings as code.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Base URL of the BearMQ instance, e.g. `https://api.bearmq.com`. Falls back " +
					"to the `BEARMQ_ENDPOINT` environment variable, then a local development default.",
			},
			"api_key": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				MarkdownDescription: "BearMQ messaging API key, sent as `X-API-KEY`. Falls back to the " +
					"`BEARMQ_API_KEY` environment variable. Exactly one of `api_key` or `token` is required.",
			},
			"token": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				MarkdownDescription: "JWT bearer token, sent as `Authorization: Bearer`. Falls back to the " +
					"`BEARMQ_TOKEN` environment variable. Exactly one of `api_key` or `token` is required.",
			},
			"insecure": schema.BoolAttribute{
				Optional: true,
				MarkdownDescription: "Skip TLS certificate verification. Falls back to the `BEARMQ_INSECURE` " +
					"environment variable. Do not use outside development.",
			},
		},
	}
}

func (p *bearmqProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := firstNonEmpty(cfg.Endpoint.ValueString(), os.Getenv("BEARMQ_ENDPOINT"), defaultEndpoint)
	apiKey := firstNonEmpty(cfg.APIKey.ValueString(), os.Getenv("BEARMQ_API_KEY"))
	token := firstNonEmpty(cfg.Token.ValueString(), os.Getenv("BEARMQ_TOKEN"))

	insecure := cfg.Insecure.ValueBool()
	if cfg.Insecure.IsNull() && os.Getenv("BEARMQ_INSECURE") == "true" {
		insecure = true
	}

	if (apiKey == "") == (token == "") {
		resp.Diagnostics.AddError(
			"Ambiguous BearMQ credentials",
			"Set exactly one of `api_key` / `BEARMQ_API_KEY` or `token` / `BEARMQ_TOKEN`.",
		)
		return
	}

	c, err := client.New(client.Config{
		Endpoint:  endpoint,
		APIKey:    apiKey,
		Token:     token,
		Insecure:  insecure,
		UserAgent: "terraform-provider-bearmq/" + p.version,
	})
	if err != nil {
		resp.Diagnostics.AddError("Invalid BearMQ provider configuration", err.Error())
		return
	}

	resp.ResourceData = c
	resp.DataSourceData = c
}

func (p *bearmqProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewVHostResource,
		NewQueueResource,
		NewExchangeResource,
		NewBindingResource,
	}
}

func (p *bearmqProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewVHostDataSource,
		NewQueueDataSource,
		NewExchangeDataSource,
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// clientFromProviderData is the shared plumbing every resource/data source uses
// in its Configure to recover the *client.Client set on the provider.
func clientFromProviderData(providerData any) (*client.Client, string) {
	if providerData == nil {
		return nil, ""
	}
	c, ok := providerData.(*client.Client)
	if !ok {
		return nil, "expected *client.Client from the provider — this is a provider bug"
	}
	return c, ""
}

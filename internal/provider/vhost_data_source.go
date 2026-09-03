package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nuricanozturk01/terraform-provider-bearmq/internal/client"
)

var (
	_ datasource.DataSource                     = (*vhostDataSource)(nil)
	_ datasource.DataSourceWithConfigure        = (*vhostDataSource)(nil)
	_ datasource.DataSourceWithConfigValidators = (*vhostDataSource)(nil)
)

// NewVHostDataSource is the bearmq_vhost data source factory.
func NewVHostDataSource() datasource.DataSource { return &vhostDataSource{} }

type vhostDataSource struct {
	client *client.Client
}

type vhostDataModel struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	Username  types.String `tfsdk:"username"`
	Domain    types.String `tfsdk:"domain"`
	URL       types.String `tfsdk:"url"`
	CreatedAt types.String `tfsdk:"created_at"`
	Status    types.String `tfsdk:"status"`
}

func (d *vhostDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vhost"
}

func (d *vhostDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Look up an existing BearMQ virtual host by `id` or by `name`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Virtual host ID. Exactly one of `id` or `name` must be set.",
			},
			"name": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Virtual host name. Exactly one of `id` or `name` must be set.",
			},
			"username":   computedDataString("AMQP username."),
			"domain":     computedDataString("Host portion of the connection URI."),
			"url":        computedDataString("Full AMQP connection URI."),
			"created_at": computedDataString("RFC 3339 creation timestamp."),
			"status":     computedDataString("Lifecycle status."),
		},
	}
}

func (d *vhostDataSource) ConfigValidators(_ context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.ExactlyOneOf(path.MatchRoot("id"), path.MatchRoot("name")),
	}
}

func (d *vhostDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	c, errMsg := clientFromProviderData(req.ProviderData)
	if errMsg != "" {
		resp.Diagnostics.AddError("Provider configuration error", errMsg)
		return
	}
	d.client = c
}

func (d *vhostDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg vhostDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var (
		vh  *client.VHost
		err error
	)
	if !cfg.ID.IsNull() {
		vh, err = d.client.GetVHost(ctx, cfg.ID.ValueString())
	} else {
		vh, err = d.client.FindVHostByName(ctx, cfg.Name.ValueString())
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to look up virtual host", err.Error())
		return
	}

	cfg.ID = types.StringValue(vh.ID)
	cfg.Name = types.StringValue(vh.Name)
	cfg.Username = types.StringValue(vh.Username)
	cfg.Domain = types.StringValue(vh.Domain)
	cfg.URL = types.StringValue(vh.URL)
	cfg.CreatedAt = types.StringValue(vh.CreatedAt)
	cfg.Status = types.StringValue(vh.Status)
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}

func computedDataString(desc string) schema.StringAttribute {
	return schema.StringAttribute{Computed: true, MarkdownDescription: desc}
}

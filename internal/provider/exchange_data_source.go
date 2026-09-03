package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nuricanozturk01/terraform-provider-bearmq/internal/client"
)

var (
	_ datasource.DataSource              = (*exchangeDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*exchangeDataSource)(nil)
)

// NewExchangeDataSource is the bearmq_exchange data source factory.
func NewExchangeDataSource() datasource.DataSource { return &exchangeDataSource{} }

type exchangeDataSource struct {
	client *client.Client
}

type exchangeDataModel struct {
	ID         types.String `tfsdk:"id"`
	VHostID    types.String `tfsdk:"vhost_id"`
	Name       types.String `tfsdk:"name"`
	Type       types.String `tfsdk:"type"`
	Durable    types.Bool   `tfsdk:"durable"`
	Internal   types.Bool   `tfsdk:"internal"`
	ActualName types.String `tfsdk:"actual_name"`
	Status     types.String `tfsdk:"status"`
}

func (d *exchangeDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_exchange"
}

func (d *exchangeDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Look up an exchange by name within a virtual host.",
		Attributes: map[string]schema.Attribute{
			"vhost_id":    schema.StringAttribute{Required: true, MarkdownDescription: "Owning virtual host ID."},
			"name":        schema.StringAttribute{Required: true, MarkdownDescription: "Exchange name."},
			"id":          computedDataString("Server-assigned exchange ID."),
			"type":        computedDataString("Exchange type."),
			"durable":     schema.BoolAttribute{Computed: true},
			"internal":    schema.BoolAttribute{Computed: true},
			"actual_name": computedDataString("Internal storage name."),
			"status":      computedDataString("Lifecycle status."),
		},
	}
}

func (d *exchangeDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	c, errMsg := clientFromProviderData(req.ProviderData)
	if errMsg != "" {
		resp.Diagnostics.AddError("Provider configuration error", errMsg)
		return
	}
	d.client = c
}

func (d *exchangeDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg exchangeDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	e, err := d.client.FindExchangeByName(ctx, cfg.VHostID.ValueString(), cfg.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to look up exchange", err.Error())
		return
	}

	cfg.ID = types.StringValue(e.ID)
	cfg.Name = types.StringValue(e.Name)
	cfg.Type = types.StringValue(e.Type)
	cfg.Durable = types.BoolValue(e.Durable)
	cfg.Internal = types.BoolValue(e.Internal)
	cfg.ActualName = types.StringValue(e.ActualName)
	cfg.Status = types.StringValue(e.Status)
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}

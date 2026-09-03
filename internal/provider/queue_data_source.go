package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nuricanozturk01/terraform-provider-bearmq/internal/client"
)

var (
	_ datasource.DataSource              = (*queueDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*queueDataSource)(nil)
)

// NewQueueDataSource is the bearmq_queue data source factory.
func NewQueueDataSource() datasource.DataSource { return &queueDataSource{} }

type queueDataSource struct {
	client *client.Client
}

type queueDataModel struct {
	ID              types.String `tfsdk:"id"`
	VHostID         types.String `tfsdk:"vhost_id"`
	Name            types.String `tfsdk:"name"`
	Durable         types.Bool   `tfsdk:"durable"`
	Exclusive       types.Bool   `tfsdk:"exclusive"`
	AutoDelete      types.Bool   `tfsdk:"auto_delete"`
	ActualName      types.String `tfsdk:"actual_name"`
	Status          types.String `tfsdk:"status"`
	OverflowPolicy  types.String `tfsdk:"overflow_policy"`
	MaxMessageCount types.Int64  `tfsdk:"max_message_count"`
}

func (d *queueDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_queue"
}

func (d *queueDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Look up a queue by name within a virtual host.",
		Attributes: map[string]schema.Attribute{
			"vhost_id":          schema.StringAttribute{Required: true, MarkdownDescription: "Owning virtual host ID."},
			"name":              schema.StringAttribute{Required: true, MarkdownDescription: "Queue name."},
			"id":                computedDataString("Server-assigned queue ID."),
			"durable":           schema.BoolAttribute{Computed: true},
			"exclusive":         schema.BoolAttribute{Computed: true},
			"auto_delete":       schema.BoolAttribute{Computed: true},
			"actual_name":       computedDataString("Internal storage name."),
			"status":            computedDataString("Lifecycle status."),
			"overflow_policy":   computedDataString("Overflow policy."),
			"max_message_count": schema.Int64Attribute{Computed: true},
		},
	}
}

func (d *queueDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	c, errMsg := clientFromProviderData(req.ProviderData)
	if errMsg != "" {
		resp.Diagnostics.AddError("Provider configuration error", errMsg)
		return
	}
	d.client = c
}

func (d *queueDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg queueDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	q, err := d.client.FindQueueByName(ctx, cfg.VHostID.ValueString(), cfg.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to look up queue", err.Error())
		return
	}

	cfg.ID = types.StringValue(q.ID)
	cfg.Name = types.StringValue(q.Name)
	cfg.Durable = types.BoolValue(q.Durable)
	cfg.Exclusive = types.BoolValue(q.Exclusive)
	cfg.AutoDelete = types.BoolValue(q.AutoDelete)
	cfg.ActualName = types.StringValue(q.ActualName)
	cfg.Status = types.StringValue(q.Status)
	cfg.OverflowPolicy = types.StringValue(q.OverflowPolicy)
	cfg.MaxMessageCount = types.Int64Value(q.MaxMessageCount)
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}

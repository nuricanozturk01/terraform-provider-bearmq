package provider

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nuricanozturk01/terraform-provider-bearmq/internal/client"
)

var (
	_ resource.Resource                = (*queueResource)(nil)
	_ resource.ResourceWithConfigure   = (*queueResource)(nil)
	_ resource.ResourceWithImportState = (*queueResource)(nil)
)

// NewQueueResource is the bearmq_queue resource factory.
func NewQueueResource() resource.Resource { return &queueResource{} }

type queueResource struct {
	client *client.Client
}

type queueModel struct {
	ID              types.String `tfsdk:"id"`
	VHostID         types.String `tfsdk:"vhost_id"`
	Name            types.String `tfsdk:"name"`
	Durable         types.Bool   `tfsdk:"durable"`
	Exclusive       types.Bool   `tfsdk:"exclusive"`
	AutoDelete      types.Bool   `tfsdk:"auto_delete"`
	Arguments       types.Map    `tfsdk:"arguments"`
	DLQName         types.String `tfsdk:"dlq_name"`
	ActualName      types.String `tfsdk:"actual_name"`
	Status          types.String `tfsdk:"status"`
	OverflowPolicy  types.String `tfsdk:"overflow_policy"`
	MaxMessageCount types.Int64  `tfsdk:"max_message_count"`
}

func (r *queueResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_queue"
}

func (r *queueResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replaceStr := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	replaceBool := []planmodifier.Bool{boolplanmodifier.RequiresReplace()}
	replaceMap := []planmodifier.Map{mapplanmodifier.RequiresReplace()}

	resp.Schema = schema.Schema{
		MarkdownDescription: "A queue inside a BearMQ virtual host. Every argument forces replacement — " +
			"BearMQ queues are immutable once declared.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"vhost_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "ID of the owning virtual host.",
				PlanModifiers:       replaceStr,
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Queue name, unique within the virtual host.",
				PlanModifiers:       replaceStr,
			},
			"durable": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Survive a broker restart. Defaults to `true`.",
				PlanModifiers:       replaceBool,
			},
			"exclusive": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Restrict the queue to its declaring connection and delete it when that connection closes. Defaults to `false`.",
				PlanModifiers:       replaceBool,
			},
			"auto_delete": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Delete the queue once its last consumer disconnects. Defaults to `false`.",
				PlanModifiers:       replaceBool,
			},
			"arguments": schema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
				MarkdownDescription: "AMQP `x-` arguments. Numeric/boolean-looking values are sent as JSON " +
					"numbers/booleans (e.g. `x-message-ttl = \"60000\"`). Not refreshed from the server.",
				PlanModifiers: replaceMap,
			},
			"dlq_name": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Name of a dead-letter queue to attach.",
				PlanModifiers:       replaceStr,
			},
			"actual_name":       computedString("Internal storage name assigned by BearMQ."),
			"status":            computedString("Lifecycle status reported by BearMQ."),
			"overflow_policy":   computedString("Overflow policy reported by BearMQ."),
			"max_message_count": schema.Int64Attribute{Computed: true, MarkdownDescription: "Configured max message count."},
		},
	}
}

func (r *queueResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	c, errMsg := clientFromProviderData(req.ProviderData)
	if errMsg != "" {
		resp.Diagnostics.AddError("Provider configuration error", errMsg)
		return
	}
	r.client = c
}

func (r *queueResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan queueModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	args, diags := argsToMap(ctx, plan.Arguments)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.CreateQueue(ctx, plan.VHostID.ValueString(), client.QueueRequest{
		Name:       plan.Name.ValueString(),
		Durable:    plan.Durable.ValueBool(),
		Exclusive:  plan.Exclusive.ValueBool(),
		AutoDelete: plan.AutoDelete.ValueBool(),
		Arguments:  args,
		DLQName:    plan.DLQName.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create queue", err.Error())
		return
	}

	applyQueue(&plan, created)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *queueResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state queueModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	q, err := r.client.GetQueue(ctx, state.VHostID.ValueString(), state.ID.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read queue", err.Error())
		return
	}

	applyQueue(&state, q)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is unreachable: every settable attribute is RequiresReplace. It exists
// only to satisfy the resource.Resource interface.
func (r *queueResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan queueModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *queueResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state queueModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.DeleteQueue(ctx, state.VHostID.ValueString(), state.ID.ValueString())
	if err != nil && !errors.Is(err, client.ErrNotFound) {
		resp.Diagnostics.AddError("Unable to delete queue", err.Error())
	}
}

func (r *queueResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	vhostID, queueID, ok := splitImportID(req.ID)
	if !ok {
		resp.Diagnostics.AddError("Invalid import ID", `expected "<vhost_id>/<queue_id>"`)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("vhost_id"), vhostID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), queueID)...)
}

func applyQueue(m *queueModel, q *client.Queue) {
	m.ID = types.StringValue(q.ID)
	m.Name = types.StringValue(q.Name)
	m.Durable = types.BoolValue(q.Durable)
	m.Exclusive = types.BoolValue(q.Exclusive)
	m.AutoDelete = types.BoolValue(q.AutoDelete)
	m.ActualName = types.StringValue(q.ActualName)
	m.Status = types.StringValue(q.Status)
	m.OverflowPolicy = types.StringValue(q.OverflowPolicy)
	m.MaxMessageCount = types.Int64Value(q.MaxMessageCount)
	if q.DLQName != "" {
		m.DLQName = types.StringValue(q.DLQName)
	}
}

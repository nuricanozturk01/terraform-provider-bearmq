package provider

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nuricanozturk01/terraform-provider-bearmq/internal/client"
)

var (
	_ resource.Resource                = (*bindingResource)(nil)
	_ resource.ResourceWithConfigure   = (*bindingResource)(nil)
	_ resource.ResourceWithImportState = (*bindingResource)(nil)
)

var destinationTypes = []string{"QUEUE", "EXCHANGE"}

// NewBindingResource is the bearmq_binding resource factory.
func NewBindingResource() resource.Resource { return &bindingResource{} }

type bindingResource struct {
	client *client.Client
}

type bindingModel struct {
	ID              types.String `tfsdk:"id"`
	VHostID         types.String `tfsdk:"vhost_id"`
	Source          types.String `tfsdk:"source"`
	Destination     types.String `tfsdk:"destination"`
	DestinationType types.String `tfsdk:"destination_type"`
	RoutingKey      types.String `tfsdk:"routing_key"`
	Arguments       types.Map    `tfsdk:"arguments"`
	Status          types.String `tfsdk:"status"`
}

func (r *bindingResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_binding"
}

func (r *bindingResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replaceStr := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	replaceMap := []planmodifier.Map{mapplanmodifier.RequiresReplace()}

	resp.Schema = schema.Schema{
		MarkdownDescription: "A binding from a source exchange to a queue or exchange. Immutable — every " +
			"argument forces replacement.",
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
			"source": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the source exchange. Must already exist in the virtual host.",
				PlanModifiers:       replaceStr,
			},
			"destination": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the destination queue or exchange. Must already exist.",
				PlanModifiers:       replaceStr,
			},
			"destination_type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "`QUEUE` or `EXCHANGE` (case-insensitive).",
				Validators:          []validator.String{stringvalidator.OneOfCaseInsensitive(destinationTypes...)},
				PlanModifiers:       replaceStr,
			},
			"routing_key": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Routing key / binding pattern. Defaults to empty.",
				PlanModifiers:       replaceStr,
			},
			"arguments": schema.MapAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Binding arguments (used by headers exchanges). Not refreshed from the server.",
				PlanModifiers:       replaceMap,
			},
			"status": computedString("Lifecycle status reported by BearMQ."),
		},
	}
}

func (r *bindingResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	c, errMsg := clientFromProviderData(req.ProviderData)
	if errMsg != "" {
		resp.Diagnostics.AddError("Provider configuration error", errMsg)
		return
	}
	r.client = c
}

func (r *bindingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan bindingModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	args, diags := argsToMap(ctx, plan.Arguments)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.CreateBinding(ctx, plan.VHostID.ValueString(), client.BindRequest{
		Source:          plan.Source.ValueString(),
		Destination:     plan.Destination.ValueString(),
		DestinationType: upper(plan.DestinationType.ValueString()),
		RoutingKey:      plan.RoutingKey.ValueString(),
		Arguments:       args,
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create binding", err.Error())
		return
	}

	applyBinding(&plan, created)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *bindingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state bindingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	b, err := r.client.GetBinding(ctx, state.VHostID.ValueString(), state.ID.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read binding", err.Error())
		return
	}

	applyBinding(&state, b)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is unreachable — every settable attribute is RequiresReplace.
func (r *bindingResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan bindingModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *bindingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state bindingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.DeleteBinding(ctx, state.VHostID.ValueString(), state.ID.ValueString())
	if err != nil && !errors.Is(err, client.ErrNotFound) {
		resp.Diagnostics.AddError("Unable to delete binding", err.Error())
	}
}

func (r *bindingResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	vhostID, bindingID, ok := splitImportID(req.ID)
	if !ok {
		resp.Diagnostics.AddError("Invalid import ID", `expected "<vhost_id>/<binding_id>"`)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("vhost_id"), vhostID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), bindingID)...)
}

func applyBinding(m *bindingModel, b *client.Binding) {
	m.ID = types.StringValue(b.ID)
	if b.SourceExchangeName != "" {
		m.Source = types.StringValue(b.SourceExchangeName)
	}
	if b.DestinationName != "" {
		m.Destination = types.StringValue(b.DestinationName)
	}
	m.RoutingKey = types.StringValue(b.RoutingKey)
	m.Status = types.StringValue(b.Status)
	// destination_type is not refreshed — the server echoes an upper-cased enum
	// name that would fight a lower-case config value; it is RequiresReplace.
}

func upper(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'a' && b[i] <= 'z' {
			b[i] -= 'a' - 'A'
		}
	}
	return string(b)
}

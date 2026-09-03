package provider

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nuricanozturk01/terraform-provider-bearmq/internal/client"
)

var (
	_ resource.Resource                = (*exchangeResource)(nil)
	_ resource.ResourceWithConfigure   = (*exchangeResource)(nil)
	_ resource.ResourceWithImportState = (*exchangeResource)(nil)
)

var exchangeTypes = []string{"DIRECT", "FANOUT", "TOPIC", "HEADERS"}

// NewExchangeResource is the bearmq_exchange resource factory.
func NewExchangeResource() resource.Resource { return &exchangeResource{} }

type exchangeResource struct {
	client *client.Client
}

type exchangeModel struct {
	ID         types.String `tfsdk:"id"`
	VHostID    types.String `tfsdk:"vhost_id"`
	Name       types.String `tfsdk:"name"`
	Type       types.String `tfsdk:"type"`
	Durable    types.Bool   `tfsdk:"durable"`
	Internal   types.Bool   `tfsdk:"internal"`
	Delayed    types.Bool   `tfsdk:"delayed"`
	Args       types.Map    `tfsdk:"args"`
	ActualName types.String `tfsdk:"actual_name"`
	Status     types.String `tfsdk:"status"`
}

func (r *exchangeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_exchange"
}

func (r *exchangeResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replaceStr := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	replaceBool := []planmodifier.Bool{boolplanmodifier.RequiresReplace()}
	replaceMap := []planmodifier.Map{mapplanmodifier.RequiresReplace()}

	resp.Schema = schema.Schema{
		MarkdownDescription: "An exchange inside a BearMQ virtual host. Every argument forces replacement.",
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
				MarkdownDescription: "Exchange name, unique within the virtual host.",
				PlanModifiers:       replaceStr,
			},
			"type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Exchange type: `DIRECT`, `FANOUT`, `TOPIC` or `HEADERS` (case-insensitive).",
				Validators:          []validator.String{stringvalidator.OneOfCaseInsensitive(exchangeTypes...)},
				PlanModifiers:       replaceStr,
			},
			"durable": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Survive a broker restart. Defaults to `true`.",
				PlanModifiers:       replaceBool,
			},
			"internal": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Internal exchanges accept bindings from other exchanges but reject direct publishes. Defaults to `false`.",
				PlanModifiers:       replaceBool,
			},
			"delayed": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Enable delayed-message delivery. Not refreshed from the server.",
				PlanModifiers:       replaceBool,
			},
			"args": schema.MapAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Exchange arguments. Not refreshed from the server.",
				PlanModifiers:       replaceMap,
			},
			"actual_name": computedString("Internal storage name assigned by BearMQ."),
			"status":      computedString("Lifecycle status reported by BearMQ."),
		},
	}
}

func (r *exchangeResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	c, errMsg := clientFromProviderData(req.ProviderData)
	if errMsg != "" {
		resp.Diagnostics.AddError("Provider configuration error", errMsg)
		return
	}
	r.client = c
}

func (r *exchangeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan exchangeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	args, diags := argsToMap(ctx, plan.Args)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.CreateExchange(ctx, plan.VHostID.ValueString(), client.ExchangeRequest{
		Name:     plan.Name.ValueString(),
		Type:     plan.Type.ValueString(),
		Durable:  plan.Durable.ValueBool(),
		Internal: plan.Internal.ValueBool(),
		Delayed:  plan.Delayed.ValueBool(),
		Args:     args,
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create exchange", err.Error())
		return
	}

	applyExchange(&plan, created)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *exchangeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state exchangeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	e, err := r.client.GetExchange(ctx, state.VHostID.ValueString(), state.ID.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read exchange", err.Error())
		return
	}

	applyExchange(&state, e)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is unreachable — every settable attribute is RequiresReplace.
func (r *exchangeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan exchangeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *exchangeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state exchangeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.DeleteExchange(ctx, state.VHostID.ValueString(), state.ID.ValueString())
	if err != nil && !errors.Is(err, client.ErrNotFound) {
		resp.Diagnostics.AddError("Unable to delete exchange", err.Error())
	}
}

func (r *exchangeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	vhostID, exchangeID, ok := splitImportID(req.ID)
	if !ok {
		resp.Diagnostics.AddError("Invalid import ID", `expected "<vhost_id>/<exchange_id>"`)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("vhost_id"), vhostID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), exchangeID)...)
}

func applyExchange(m *exchangeModel, e *client.Exchange) {
	m.ID = types.StringValue(e.ID)
	m.Name = types.StringValue(e.Name)
	// type is intentionally not refreshed: the server echoes it upper-cased
	// ("TOPIC") which would fight a lower-case config value. It is RequiresReplace
	// anyway, so a manual type change on the server still forces recreation via
	// the other attributes.
	m.Durable = types.BoolValue(e.Durable)
	m.Internal = types.BoolValue(e.Internal)
	m.ActualName = types.StringValue(e.ActualName)
	m.Status = types.StringValue(e.Status)
}

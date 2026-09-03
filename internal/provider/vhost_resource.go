package provider

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nuricanozturk01/terraform-provider-bearmq/internal/client"
)

var (
	_ resource.Resource                = (*vhostResource)(nil)
	_ resource.ResourceWithConfigure   = (*vhostResource)(nil)
	_ resource.ResourceWithImportState = (*vhostResource)(nil)
)

const (
	statusActive = "ACTIVE"
	statusPaused = "PAUSED"
)

// NewVHostResource is the bearmq_vhost resource factory.
func NewVHostResource() resource.Resource { return &vhostResource{} }

type vhostResource struct {
	client *client.Client
}

type vhostModel struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	Status    types.String `tfsdk:"status"`
	Username  types.String `tfsdk:"username"`
	Password  types.String `tfsdk:"password"`
	Domain    types.String `tfsdk:"domain"`
	URL       types.String `tfsdk:"url"`
	CreatedAt types.String `tfsdk:"created_at"`
}

func (r *vhostResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vhost"
}

func (r *vhostResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A BearMQ virtual host: an isolated AMQP namespace with its own credentials.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Server-assigned virtual host identifier.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Optional: true,
				Computed: true,
				MarkdownDescription: "Virtual host name. Omit to let BearMQ generate one. Changing this " +
					"forces a new virtual host.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"status": schema.StringAttribute{
				Optional: true,
				Computed: true,
				MarkdownDescription: "Desired lifecycle status: `ACTIVE` or `PAUSED`. Updated in place via " +
					"the status endpoint.",
				Validators: []validator.String{stringvalidator.OneOf(statusActive, statusPaused)},
			},
			"username":   computedString("AMQP username for this virtual host."),
			"password":   computedSensitiveString("AMQP password. Only returned at creation time; blank after import."),
			"domain":     computedString("Host portion of the generated connection URI."),
			"url":        computedString("Full AMQP connection URI."),
			"created_at": computedString("RFC 3339 creation timestamp."),
		},
	}
}

func (r *vhostResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	c, errMsg := clientFromProviderData(req.ProviderData)
	if errMsg != "" {
		resp.Diagnostics.AddError("Provider configuration error", errMsg)
		return
	}
	r.client = c
}

func (r *vhostResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan vhostModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.CreateVHost(ctx, plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to create virtual host", err.Error())
		return
	}

	// password is only ever present in this create response.
	plan.Password = types.StringValue(created.Password)
	applyVHost(&plan, created)

	if desired := plan.Status.ValueString(); desired != "" && desired != created.Status {
		updated, statusErr := r.client.SetVHostStatus(ctx, created.ID, desired)
		if statusErr != nil {
			resp.Diagnostics.AddError("Virtual host created but status update failed", statusErr.Error())
			resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...) // persist so it can be retried/imported
			return
		}
		applyVHost(&plan, updated)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *vhostResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state vhostModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	vh, err := r.client.GetVHost(ctx, state.ID.ValueString())
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read virtual host", err.Error())
		return
	}

	applyVHost(&state, vh) // password/username kept from prior state if absent
	if vh.Username != "" {
		state.Username = types.StringValue(vh.Username)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *vhostResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state vhostModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// name is RequiresReplace; only status can reach Update.
	if plan.Status.ValueString() != state.Status.ValueString() {
		updated, err := r.client.SetVHostStatus(ctx, state.ID.ValueString(), plan.Status.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Unable to update virtual host status", err.Error())
			return
		}
		applyVHost(&state, updated)
	}
	state.Status = plan.Status
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *vhostResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state vhostModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteVHost(ctx, state.ID.ValueString()); err != nil && !errors.Is(err, client.ErrNotFound) {
		resp.Diagnostics.AddError("Unable to delete virtual host", err.Error())
	}
}

func (r *vhostResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func applyVHost(m *vhostModel, vh *client.VHost) {
	m.ID = types.StringValue(vh.ID)
	m.Name = types.StringValue(vh.Name)
	m.Domain = types.StringValue(vh.Domain)
	m.URL = types.StringValue(vh.URL)
	m.CreatedAt = types.StringValue(vh.CreatedAt)
	if vh.Status != "" {
		m.Status = types.StringValue(vh.Status)
	}
	if vh.Username != "" {
		m.Username = types.StringValue(vh.Username)
	}
	if m.Password.IsNull() || m.Password.IsUnknown() {
		m.Password = types.StringValue("")
	}
}

func computedString(desc string) schema.StringAttribute {
	return schema.StringAttribute{Computed: true, MarkdownDescription: desc}
}

func computedSensitiveString(desc string) schema.StringAttribute {
	return schema.StringAttribute{Computed: true, Sensitive: true, MarkdownDescription: desc}
}

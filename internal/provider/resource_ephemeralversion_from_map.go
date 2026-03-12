package provider

import (
	"context"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &EphemeralVersionMapResource{}
var _ resource.ResourceWithModifyPlan = &EphemeralVersionMapResource{}

type EphemeralVersionMapResource struct{}

func NewEphemeralVersionMapResource() resource.Resource {
	return &EphemeralVersionMapResource{}
}

type ephemeralVersionMapModel struct {
	ID       types.String `tfsdk:"id"`
	Values   types.Map    `tfsdk:"values"`
	Versions types.Map    `tfsdk:"versions"`
}

func (r *EphemeralVersionMapResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "ephemeralversion_from_map"
}

func (r *EphemeralVersionMapResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Derives a map of version strings from a write-only map of input values. " +
			"Each key in `values` maps to the same key in `versions`, whose value is the MD5 hex digest of the input. " +
			"`values` is write-only and is never stored in Terraform state. `versions` is recalculated on every apply.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Unique resource identifier (UUID). Generated once on creation and never changed.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"values": schema.MapAttribute{
				Required:            true,
				WriteOnly:           true,
				Sensitive:           true,
				ElementType:         types.StringType,
				MarkdownDescription: "Map of secret names to their values. Write-only: never stored in state.",
			},
			"versions": schema.MapAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Map of secret names to the MD5 hex digest of their corresponding value. Recalculated on every apply.",
			},
		},
	}
}

// hashMapValues returns a types.Map where each value is the MD5 hex digest of the corresponding input.
func hashMapValues(_ context.Context, values types.Map) (types.Map, diag.Diagnostics) {
	elems := make(map[string]attr.Value, len(values.Elements()))
	for k, v := range values.Elements() {
		s, ok := v.(types.String)
		if !ok {
			s = types.StringValue("")
		}
		elems[k] = types.StringValue(md5Hex(s.ValueString()))
	}
	return types.MapValue(types.StringType, elems)
}

// ModifyPlan computes versions during planning by reading values from config.
// id is left to UseStateForUnknown (preserved on update, unknown only on create).
func (r *EphemeralVersionMapResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}

	var config ephemeralVersionMapModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	versions, diags := hashMapValues(ctx, config.Values)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("versions"), versions)...)
}

func (r *EphemeralVersionMapResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ephemeralVersionMapModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var config ephemeralVersionMapModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	versions, diags := hashMapValues(ctx, config.Values)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.ID = types.StringValue(uuid.New().String())
	data.Versions = versions
	data.Values = types.MapNull(types.StringType)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *EphemeralVersionMapResource) Read(_ context.Context, _ resource.ReadRequest, _ *resource.ReadResponse) {
	// Nothing to refresh: versions are derived from write-only values and not stored.
}

func (r *EphemeralVersionMapResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ephemeralVersionMapModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var config ephemeralVersionMapModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	versions, diags := hashMapValues(ctx, config.Values)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// ID is preserved from state via UseStateForUnknown.
	data.Versions = versions
	data.Values = types.MapNull(types.StringType)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *EphemeralVersionMapResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
	// Nothing to do on delete.
}

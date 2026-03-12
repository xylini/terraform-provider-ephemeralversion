package provider

import (
	"context"
	"crypto/md5" //nolint:gosec // MD5 used for versioning, not security
	"fmt"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &EphemeralVersionResource{}
var _ resource.ResourceWithModifyPlan = &EphemeralVersionResource{}

type EphemeralVersionResource struct{}

func NewEphemeralVersionResource() resource.Resource {
	return &EphemeralVersionResource{}
}

// ephemeralVersionModel is the Terraform state model for the resource.
// "value" is write-only so it is never stored in state (kept as null after apply).
type ephemeralVersionModel struct {
	ID      types.String `tfsdk:"id"`
	Name    types.String `tfsdk:"name"`
	Value   types.String `tfsdk:"value"`
	Version types.String `tfsdk:"version"`
}

func (r *EphemeralVersionResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "ephemeralversion_from"
}

func (r *EphemeralVersionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Derives a version string from a write-only input value. " +
			"The `version` attribute is the MD5 hex digest of `value` and is recalculated on every apply. " +
			"`value` is write-only and is never stored in Terraform state.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Unique resource identifier (UUID). Generated once on creation and never changed.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "A human-readable name for this resource.",
			},
			"value": schema.StringAttribute{
				Required:            true,
				WriteOnly:           true,
				Sensitive:           true,
				MarkdownDescription: "The input value. Write-only: never stored in state.",
			},
			"version": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The MD5 hex digest of `value`. Recalculated on every apply.",
			},
		},
	}
}

// ModifyPlan computes version during planning by reading value from config.
// id is left to UseStateForUnknown (preserved on update, unknown only on create).
func (r *EphemeralVersionResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}

	var config ephemeralVersionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("version"), types.StringValue(md5Hex(config.Value.ValueString())))...)
}

func md5Hex(s string) string {
	sum := md5.Sum([]byte(s)) //nolint:gosec
	return fmt.Sprintf("%x", sum)
}

func (r *EphemeralVersionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ephemeralVersionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var config ephemeralVersionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.ID = types.StringValue(uuid.New().String())
	data.Version = types.StringValue(md5Hex(config.Value.ValueString()))
	data.Value = types.StringNull()

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *EphemeralVersionResource) Read(_ context.Context, _ resource.ReadRequest, _ *resource.ReadResponse) {
	// Nothing to refresh: version is derived from write-only value and not stored.
}

func (r *EphemeralVersionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ephemeralVersionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var config ephemeralVersionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// ID is preserved from state via UseStateForUnknown.
	data.Version = types.StringValue(md5Hex(config.Value.ValueString()))
	data.Value = types.StringNull()

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *EphemeralVersionResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
	// Nothing to do on delete.
}

// SPDX-License-Identifier: AGPL-3.0-or-later

package provider

import (
	"context"

	"github.com/JamesonRGrieve/tofu-nextcloud/internal/nextcloud"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// nextcloud_config manages config.php system config via `occ config:system:set`/
// `get`. Only the keys declared in `values` are managed — manage-declared-only
// diff, so unmanaged config is never clobbered. Booleans/integers are written
// with the correct occ `--type`; nested keys (memcache.local, trusted_domains.1)
// are addressed by their dotted path.
var (
	_ resource.Resource                = (*configResource)(nil)
	_ resource.ResourceWithConfigure   = (*configResource)(nil)
	_ resource.ResourceWithImportState = (*configResource)(nil)
)

// NewConfigResource constructs the nextcloud_config resource.
func NewConfigResource() resource.Resource { return &configResource{} }

type configResource struct {
	client *providerClient
}

type configModel struct {
	ID     types.String `tfsdk:"id"`
	Path   types.String `tfsdk:"path"`
	Values types.Map    `tfsdk:"values"`
}

func (r *configResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_config"
}

func (r *configResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "config.php system config managed via occ, manage-declared-only. Common keys: " +
			"trusted_domains.N, overwrite.cli.url, default_phone_region, maintenance_window_start, memcache.local, " +
			"loglevel, maintenance.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"path": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				MarkdownDescription: "Nextcloud document root (holds `occ`). Defaults to the provider `docroot`.",
			},
			"values": schema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
				MarkdownDescription: "config.php system keys as name→value. Dotted keys (memcache.local, " +
					"overwrite.cli.url, trusted_domains.1) address nested config; known integer/boolean keys " +
					"(maintenance_window_start, loglevel, maintenance, …) are written with the correct occ `--type`.",
			},
		},
	}
}

func (r *configResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req, resp)
}

func (r *configResource) docroot() string {
	if r.client == nil {
		return defaultDocroot
	}
	return r.client.Docroot
}

func (r *configResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	r.write(ctx, &req.Plan, &resp.State, &resp.Diagnostics)
}

func (r *configResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	r.write(ctx, &req.Plan, &resp.State, &resp.Diagnostics)
}

// write applies the declared config (Create/Update share it).
func (r *configResource) write(ctx context.Context, plan planGetter, state stateSetter, diags *diag.Diagnostics) {
	var m configModel
	diags.Append(plan.Get(ctx, &m)...)
	if diags.HasError() {
		return
	}
	p := resolvePath(m.Path, r.docroot())
	if r.client != nil && r.client.SSH != nil {
		occ := r.client.occ(p)
		values := mapValues(ctx, m.Values, diags)
		for _, key := range sortedKeys(values) {
			if err := occ.ConfigSystemSet(key, values[key], nextcloud.SystemConfigType(key)); err != nil {
				diags.AddError("occ config:system:set "+key+" failed", err.Error())
				return
			}
		}
	}
	m.Path = types.StringValue(p)
	m.ID = types.StringValue(p)
	diags.Append(state.Set(ctx, &m)...)
}

func (r *configResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m configModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil || r.client.SSH == nil {
		return
	}
	occ := r.client.occ(resolvePath(m.Path, r.docroot()))

	// Subset refresh: only declared keys are reconciled; equal values keep their
	// declared form (0-diff), drift surfaces the device value.
	declared := mapValues(ctx, m.Values, &resp.Diagnostics)
	if len(declared) > 0 {
		refreshed := map[string]string{}
		for _, key := range sortedKeys(declared) {
			live, err := occ.ConfigSystemGet(key)
			if err != nil {
				// Key absent on the device → surface as empty (drift).
				refreshed[key] = ""
				continue
			}
			refreshed[key] = nextcloud.ReconcileConfigValue(key, declared[key], live)
		}
		mv, d := types.MapValueFrom(ctx, types.StringType, refreshed)
		resp.Diagnostics.Append(d...)
		m.Values = mv
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

// Delete is a no-op: config.php keys persist; the resource stops managing them.
func (r *configResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

func (r *configResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("path"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

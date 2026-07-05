// SPDX-License-Identifier: AGPL-3.0-or-later

package provider

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// nextcloud_appconfig manages a single per-app config value via `occ
// config:app:set`/`get`/`delete`. Imports to 0-diff by `<app_id>/<key>`.
var (
	_ resource.Resource                = (*appConfigResource)(nil)
	_ resource.ResourceWithConfigure   = (*appConfigResource)(nil)
	_ resource.ResourceWithImportState = (*appConfigResource)(nil)
)

// NewAppConfigResource constructs the nextcloud_appconfig resource.
func NewAppConfigResource() resource.Resource { return &appConfigResource{} }

type appConfigResource struct {
	client *providerClient
}

type appConfigModel struct {
	ID    types.String `tfsdk:"id"`
	Path  types.String `tfsdk:"path"`
	AppID types.String `tfsdk:"app_id"`
	Key   types.String `tfsdk:"key"`
	Value types.String `tfsdk:"value"`
}

func (r *appConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_appconfig"
}

func (r *appConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A single per-app config value managed via `occ config:app:set/get/delete`.",
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
			"app_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The app id (e.g. `theming`).",
			},
			"key": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The app config key (e.g. `color`).",
			},
			"value": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The app config value.",
			},
		},
	}
}

func (r *appConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req, resp)
}

func (r *appConfigResource) docroot() string {
	if r.client == nil {
		return defaultDocroot
	}
	return r.client.Docroot
}

func (r *appConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	r.write(ctx, &req.Plan, &resp.State, &resp.Diagnostics)
}

func (r *appConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	r.write(ctx, &req.Plan, &resp.State, &resp.Diagnostics)
}

// write applies the declared app config value and stores state (Create/Update
// share it).
func (r *appConfigResource) write(ctx context.Context, plan planGetter, state stateSetter, diags *diag.Diagnostics) {
	var m appConfigModel
	diags.Append(plan.Get(ctx, &m)...)
	if diags.HasError() {
		return
	}
	p := resolvePath(m.Path, r.docroot())
	if r.client != nil && r.client.SSH != nil {
		occ := r.client.occ(p)
		if err := occ.ConfigAppSet(m.AppID.ValueString(), m.Key.ValueString(), m.Value.ValueString()); err != nil {
			diags.AddError("occ config:app:set failed", err.Error())
			return
		}
	}
	m.Path = types.StringValue(p)
	m.ID = types.StringValue(appConfigID(p, m.AppID.ValueString(), m.Key.ValueString()))
	diags.Append(state.Set(ctx, &m)...)
}

func (r *appConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m appConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil || r.client.SSH == nil {
		return
	}
	p := resolvePath(m.Path, r.docroot())
	occ := r.client.occ(p)
	live, err := occ.ConfigAppGet(m.AppID.ValueString(), m.Key.ValueString())
	if err != nil {
		// Key absent on the device → drop from state.
		resp.State.RemoveResource(ctx)
		return
	}
	m.Value = types.StringValue(live)
	m.Path = types.StringValue(p)
	m.ID = types.StringValue(appConfigID(p, m.AppID.ValueString(), m.Key.ValueString()))
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *appConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m appConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil || r.client.SSH == nil {
		return
	}
	occ := r.client.occ(resolvePath(m.Path, r.docroot()))
	if err := occ.ConfigAppDelete(m.AppID.ValueString(), m.Key.ValueString()); err != nil {
		resp.Diagnostics.AddError("occ config:app:delete failed", err.Error())
	}
}

func (r *appConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	app, key := parseAppConfigImportID(req.ID)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("app_id"), app)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("key"), key)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

// appConfigID is the resource id: "<path>/<app>/<key>".
func appConfigID(p, app, key string) string {
	return strings.TrimRight(p, "/") + "/" + app + "/" + key
}

// parseAppConfigImportID splits an "<app_id>/<key>" import id (path defaults to
// the provider docroot). The app id has no slash, so the first slash divides the
// two.
func parseAppConfigImportID(id string) (app, key string) {
	if i := strings.Index(id, "/"); i >= 0 {
		return id[:i], id[i+1:]
	}
	return id, ""
}

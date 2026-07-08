// SPDX-License-Identifier: AGPL-3.0-or-later

package provider

import (
	"context"

	"github.com/JamesonRGrieve/tofu-nextcloud/internal/nextcloud"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// nextcloud_app manages a Nextcloud app's install/enable state via `occ
// app:install/enable/disable/remove/update`. read = `occ app:list`. Imports to
// 0-diff by app id.
var (
	_ resource.Resource                = (*appResource)(nil)
	_ resource.ResourceWithConfigure   = (*appResource)(nil)
	_ resource.ResourceWithImportState = (*appResource)(nil)
)

// NewAppResource constructs the nextcloud_app resource.
func NewAppResource() resource.Resource { return &appResource{} }

type appResource struct {
	client *providerClient
}

type appModel struct {
	ID      types.String `tfsdk:"id"`
	Path    types.String `tfsdk:"path"`
	AppID   types.String `tfsdk:"app_id"`
	Version types.String `tfsdk:"version"`
	Enabled types.Bool   `tfsdk:"enabled"`
}

func (r *appResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app"
}

func (r *appResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A Nextcloud app managed via `occ app:install/enable/disable/remove/update`. Imports " +
			"to 0-diff by app id.",
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
				MarkdownDescription: "The app id (e.g. `calendar`).",
			},
			"version": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				MarkdownDescription: "Installed app version (reflected from `occ app:list`). Blank tracks the latest; a differing pin triggers `occ app:update`.",
			},
			"enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
				MarkdownDescription: "Whether the app is enabled (default true).",
			},
		},
	}
}

func (r *appResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req, resp)
}

func (r *appResource) docroot() string {
	if r.client == nil {
		return defaultDocroot
	}
	return r.client.Docroot
}

func (r *appResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m appModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p := resolvePath(m.Path, r.docroot())
	id := m.AppID.ValueString()
	if r.client != nil && r.client.SSH != nil {
		occ := r.client.occ(p)
		// Only fetch from the app store (`app:install`) when the app isn't already
		// present. A SHIPPED app (e.g. `federation`) is present-but-disabled — for it
		// `app:install` fails ("app is already installed"/not a store app); the right
		// action is `app:enable`. So: install only if absent, then reconcile to the
		// desired enabled state (which enables a present/shipped app or disables it).
		apps, err := occ.AppList()
		if err != nil {
			resp.Diagnostics.AddError("occ app:list failed", err.Error())
			return
		}
		if !nextcloud.AppPresent(apps, id) {
			if err := occ.AppInstall(id); err != nil {
				resp.Diagnostics.AddError("occ app:install "+id+" failed", err.Error())
				return
			}
		}
		if err := reconcileAppEnabled(occ, id, m.Enabled.ValueBool()); err != nil {
			resp.Diagnostics.AddError("occ app enable/disable "+id+" failed", err.Error())
			return
		}
		r.refresh(&m, p)
	}
	m.Path = types.StringValue(p)
	m.ID = types.StringValue(appID(p, id))
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *appResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m appModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil || r.client.SSH == nil {
		return
	}
	p := resolvePath(m.Path, r.docroot())
	occ := r.client.occ(p)
	apps, err := occ.AppList()
	if err != nil {
		resp.Diagnostics.AddError("occ app:list failed", err.Error())
		return
	}
	if _, ok := apps[m.AppID.ValueString()]; !ok {
		resp.State.RemoveResource(ctx)
		return
	}
	r.refresh(&m, p)
	m.Path = types.StringValue(p)
	m.ID = types.StringValue(appID(p, m.AppID.ValueString()))
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *appResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m, prior appModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p := resolvePath(m.Path, r.docroot())
	id := m.AppID.ValueString()
	if r.client != nil && r.client.SSH != nil {
		occ := r.client.occ(p)
		if v := m.Version.ValueString(); v != "" && v != prior.Version.ValueString() {
			if err := occ.AppUpdate(id); err != nil {
				resp.Diagnostics.AddError("occ app:update "+id+" failed", err.Error())
				return
			}
		}
		if err := reconcileAppEnabled(occ, id, m.Enabled.ValueBool()); err != nil {
			resp.Diagnostics.AddError("occ app enable/disable "+id+" failed", err.Error())
			return
		}
		r.refresh(&m, p)
	}
	m.Path = types.StringValue(p)
	m.ID = types.StringValue(appID(p, id))
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *appResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m appModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil || r.client.SSH == nil {
		return
	}
	occ := r.client.occ(resolvePath(m.Path, r.docroot()))
	if err := occ.AppRemove(m.AppID.ValueString()); err != nil {
		resp.Diagnostics.AddError("occ app:remove "+m.AppID.ValueString()+" failed", err.Error())
	}
}

func (r *appResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	p, id := parseAppImportID(req.ID)
	if p != "" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("path"), p)...)
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("app_id"), id)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

// refresh reads the live enabled/version state into the model (best-effort).
func (r *appResource) refresh(m *appModel, p string) {
	occ := r.client.occ(p)
	apps, err := occ.AppList()
	if err != nil {
		return
	}
	if s, ok := apps[m.AppID.ValueString()]; ok {
		m.Enabled = types.BoolValue(s.Enabled)
		if s.Version != "" {
			m.Version = types.StringValue(s.Version)
		}
	}
}

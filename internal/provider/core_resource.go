// SPDX-License-Identifier: AGPL-3.0-or-later

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// nextcloud_core installs and upgrades a Nextcloud instance at a docroot via occ.
// read = `occ status`; create = `occ maintenance:install`; update = `occ
// upgrade`. The admin and DB passwords are WRITE-ONLY — read from config at apply
// and never persisted to state.
var (
	_ resource.Resource                = (*coreResource)(nil)
	_ resource.ResourceWithConfigure   = (*coreResource)(nil)
	_ resource.ResourceWithImportState = (*coreResource)(nil)
)

// NewCoreResource constructs the nextcloud_core resource.
func NewCoreResource() resource.Resource { return &coreResource{} }

type coreResource struct {
	client *providerClient
}

type coreModel struct {
	ID               types.String `tfsdk:"id"`
	Path             types.String `tfsdk:"path"`
	Version          types.String `tfsdk:"version"`
	AdminUser        types.String `tfsdk:"admin_user"`
	AdminPassword    types.String `tfsdk:"admin_password"`
	DatabaseType     types.String `tfsdk:"database_type"`
	DatabaseName     types.String `tfsdk:"database_name"`
	DatabaseUser     types.String `tfsdk:"database_user"`
	DatabaseHost     types.String `tfsdk:"database_host"`
	DatabasePassword types.String `tfsdk:"database_password"`
	DataDir          types.String `tfsdk:"data_dir"`
}

func (r *coreResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_core"
}

func (r *coreResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Nextcloud core install/upgrade via occ. Imports to 0-diff from an existing install.",
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
			"version": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				MarkdownDescription: "Target Nextcloud version. When it differs from the installed version, `occ upgrade` is run (the new code must already be deployed). Omit to track the installed version.",
			},
			"admin_user": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Admin username for `occ maintenance:install` (`--admin-user`). Non-secret.",
			},
			"admin_password": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				WriteOnly: true,
				MarkdownDescription: "Admin password for `occ maintenance:install` (`--admin-pass`). WRITE-ONLY: read from " +
					"config at apply (inject from OpenBao) and NEVER stored in state.",
			},
			"database_type": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Database backend for install (`--database`): `mysql`, `pgsql`, or `sqlite`.",
			},
			"database_name": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Database name for install (`--database-name`).",
			},
			"database_user": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Database user for install (`--database-user`).",
			},
			"database_host": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Database host for install (`--database-host`).",
			},
			"database_password": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				WriteOnly: true,
				MarkdownDescription: "Database password for install (`--database-pass`). WRITE-ONLY: injected from " +
					"OpenBao at apply, never stored in state.",
			},
			"data_dir": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Data directory for install (`--data-dir`).",
			},
		},
	}
}

func (r *coreResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req, resp)
}

func (r *coreResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m coreModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p := resolvePath(m.Path, r.docroot())
	// admin_password/database_password are write-only: read from config.
	var adminPW, dbPW types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("admin_password"), &adminPW)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("database_password"), &dbPW)...)
	if r.client != nil && r.client.SSH != nil {
		occ := r.client.occ(p)
		if !occ.IsInstalled() {
			args := maintenanceInstallArgs(m, adminPW.ValueString(), dbPW.ValueString())
			if _, err := occ.Command(args...); err != nil {
				resp.Diagnostics.AddError("occ maintenance:install failed", err.Error())
				return
			}
		}
		if v, err := occ.CoreVersion(); err == nil {
			m.Version = types.StringValue(v)
		}
	}
	m.Path = types.StringValue(p)
	m.ID = types.StringValue(p)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *coreResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m coreModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil || r.client.SSH == nil {
		return
	}
	occ := r.client.occ(resolvePath(m.Path, r.docroot()))
	if !occ.IsInstalled() {
		resp.State.RemoveResource(ctx)
		return
	}
	if v, err := occ.CoreVersion(); err == nil {
		m.Version = types.StringValue(v)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *coreResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m coreModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p := resolvePath(m.Path, r.docroot())
	if r.client != nil && r.client.SSH != nil {
		occ := r.client.occ(p)
		installed, err := occ.CoreVersion()
		if err == nil && m.Version.ValueString() != "" && m.Version.ValueString() != installed {
			if _, uerr := occ.Command("upgrade"); uerr != nil {
				resp.Diagnostics.AddError("occ upgrade failed", uerr.Error())
				return
			}
		}
		if v, verr := occ.CoreVersion(); verr == nil {
			m.Version = types.StringValue(v)
		}
	}
	m.Path = types.StringValue(p)
	m.ID = types.StringValue(p)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

// Delete is a no-op: uninstalling Nextcloud is destructive and out of scope; the
// resource simply stops managing it.
func (r *coreResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

func (r *coreResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import id is the docroot path.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("path"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func (r *coreResource) docroot() string {
	if r.client == nil {
		return defaultDocroot
	}
	return r.client.Docroot
}

// maintenanceInstallArgs builds the `occ maintenance:install` arguments from the
// model and the write-only admin/DB passwords. Pure — unit-tested. An empty
// optional flag is omitted so occ applies its own default.
func maintenanceInstallArgs(m coreModel, adminPassword, dbPassword string) []string {
	args := []string{"maintenance:install"}
	appendFlag := func(name, val string) {
		if val != "" {
			args = append(args, "--"+name+"="+val)
		}
	}
	appendFlag("database", m.DatabaseType.ValueString())
	appendFlag("database-name", m.DatabaseName.ValueString())
	appendFlag("database-user", m.DatabaseUser.ValueString())
	appendFlag("database-host", m.DatabaseHost.ValueString())
	appendFlag("database-pass", dbPassword)
	appendFlag("admin-user", m.AdminUser.ValueString())
	appendFlag("admin-pass", adminPassword)
	appendFlag("data-dir", m.DataDir.ValueString())
	return args
}

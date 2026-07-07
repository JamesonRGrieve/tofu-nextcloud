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

// nextcloud_federation_config manages Nextcloud's federated-sharing feature
// toggles via `occ config:app:set files_sharing …` (plus the Global Scale lookup
// server in `config:system`). Manage-declared-only: only attributes the
// configuration sets are written and reconciled — a null attribute is left
// untouched on the device. It realizes the sharing side of a netbox-federation
// FederationRealm (protocol nextcloud_federated_sharing).
var (
	_ resource.Resource                = (*federationConfigResource)(nil)
	_ resource.ResourceWithConfigure   = (*federationConfigResource)(nil)
	_ resource.ResourceWithImportState = (*federationConfigResource)(nil)
)

// NewFederationConfigResource constructs the nextcloud_federation_config resource.
func NewFederationConfigResource() resource.Resource { return &federationConfigResource{} }

type federationConfigResource struct {
	client *providerClient
}

type federationConfigModel struct {
	ID                    types.String `tfsdk:"id"`
	Path                  types.String `tfsdk:"path"`
	OutgoingEnabled       types.Bool   `tfsdk:"outgoing_enabled"`
	IncomingEnabled       types.Bool   `tfsdk:"incoming_enabled"`
	FederatedGroupSharing types.Bool   `tfsdk:"federated_group_sharing"`
	AutoAcceptTrusted     types.Bool   `tfsdk:"auto_accept_trusted"`
	LookupServer          types.String `tfsdk:"lookup_server"`
}

// fedBoolAttr binds a files_sharing app-config key to the model field it maps
// onto, so write/read stay DRY across the four boolean toggles.
type fedBoolAttr struct {
	key string
	get func(*federationConfigModel) types.Bool
	set func(*federationConfigModel, types.Bool)
}

// fedBoolAttrs is the declared-order table of boolean federation toggles.
var fedBoolAttrs = []fedBoolAttr{
	{
		key: nextcloud.KeyOutgoingS2S,
		get: func(m *federationConfigModel) types.Bool { return m.OutgoingEnabled },
		set: func(m *federationConfigModel, v types.Bool) { m.OutgoingEnabled = v },
	},
	{
		key: nextcloud.KeyIncomingS2S,
		get: func(m *federationConfigModel) types.Bool { return m.IncomingEnabled },
		set: func(m *federationConfigModel, v types.Bool) { m.IncomingEnabled = v },
	},
	{
		key: nextcloud.KeyOutgoingGroupS2S,
		get: func(m *federationConfigModel) types.Bool { return m.FederatedGroupSharing },
		set: func(m *federationConfigModel, v types.Bool) { m.FederatedGroupSharing = v },
	},
	{
		key: nextcloud.KeyAutoAcceptTrusted,
		get: func(m *federationConfigModel) types.Bool { return m.AutoAcceptTrusted },
		set: func(m *federationConfigModel, v types.Bool) { m.AutoAcceptTrusted = v },
	},
}

func (r *federationConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_federation_config"
}

func (r *federationConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Federated-sharing settings managed via `occ config:app:set files_sharing …` " +
			"(manage-declared-only). Realizes the sharing side of a netbox-federation FederationRealm " +
			"(protocol `nextcloud_federated_sharing`).",
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
			"outgoing_enabled": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Allow this server to send federated shares (`files_sharing outgoing_server2server_share_enabled`).",
			},
			"incoming_enabled": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Allow this server to receive federated shares (`files_sharing incoming_server2server_share_enabled`).",
			},
			"federated_group_sharing": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Allow sending shares to groups on federated servers (`files_sharing outgoing_server2server_group_share_enabled`).",
			},
			"auto_accept_trusted": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Automatically accept incoming shares from trusted servers (`files_sharing federatedTrustedShareAutoAccept`).",
			},
			"lookup_server": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Global Scale lookup server base URL. Non-empty sets `config:system lookup_server` and " +
					"enables `files_sharing lookupServerUploadEnabled`; blank disables lookup-server upload.",
			},
		},
	}
}

func (r *federationConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req, resp)
}

func (r *federationConfigResource) docroot() string {
	if r.client == nil {
		return defaultDocroot
	}
	return r.client.Docroot
}

func (r *federationConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	r.write(ctx, &req.Plan, &resp.State, &resp.Diagnostics)
}

func (r *federationConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	r.write(ctx, &req.Plan, &resp.State, &resp.Diagnostics)
}

// write applies every declared (non-null) federation attribute (Create/Update
// share it). Unset attributes are left untouched on the device.
func (r *federationConfigResource) write(ctx context.Context, plan planGetter, state stateSetter, diags *diag.Diagnostics) {
	var m federationConfigModel
	diags.Append(plan.Get(ctx, &m)...)
	if diags.HasError() {
		return
	}
	p := resolvePath(m.Path, r.docroot())
	if r.client != nil && r.client.SSH != nil {
		occ := r.client.occ(p)
		for _, a := range fedBoolAttrs {
			v := a.get(&m)
			if v.IsNull() || v.IsUnknown() {
				continue
			}
			if err := occ.ConfigAppSet(nextcloud.FilesSharingApp, a.key, nextcloud.FederationBoolValue(a.key, v.ValueBool())); err != nil {
				diags.AddError("occ config:app:set files_sharing "+a.key+" failed", err.Error())
				return
			}
		}
		if !m.LookupServer.IsNull() && !m.LookupServer.IsUnknown() {
			if err := applyLookupServer(occ, m.LookupServer.ValueString()); err != nil {
				diags.AddError("occ lookup-server config failed", err.Error())
				return
			}
		}
	}
	m.Path = types.StringValue(p)
	m.ID = types.StringValue(p)
	diags.Append(state.Set(ctx, &m)...)
}

// applyLookupServer configures the Global Scale lookup server: a non-empty URL
// sets the system `lookup_server` and enables upload; a blank URL disables it.
func applyLookupServer(occ *nextcloud.OCC, url string) error {
	if u := nextcloud.NormalizeServerURL(url); u != "" {
		if err := occ.ConfigSystemSet(nextcloud.SystemKeyLookupServer, u, nextcloud.TypeString); err != nil {
			return err
		}
		return occ.ConfigAppSet(nextcloud.FilesSharingApp, nextcloud.KeyLookupServerUpload,
			nextcloud.FederationBoolValue(nextcloud.KeyLookupServerUpload, true))
	}
	return occ.ConfigAppSet(nextcloud.FilesSharingApp, nextcloud.KeyLookupServerUpload,
		nextcloud.FederationBoolValue(nextcloud.KeyLookupServerUpload, false))
}

func (r *federationConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m federationConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil || r.client.SSH == nil {
		return
	}
	p := resolvePath(m.Path, r.docroot())
	occ := r.client.occ(p)

	// Manage-declared-only: only reconcile attributes the config declares.
	for _, a := range fedBoolAttrs {
		if a.get(&m).IsNull() {
			continue
		}
		live, err := occ.ConfigAppGet(nextcloud.FilesSharingApp, a.key)
		if err != nil {
			// Absent key → treat as unset (false).
			a.set(&m, types.BoolValue(false))
			continue
		}
		a.set(&m, types.BoolValue(nextcloud.ParseFederationBool(live)))
	}
	if !m.LookupServer.IsNull() {
		m.LookupServer = types.StringValue(reconcileLookupServer(occ, m.LookupServer.ValueString()))
	}
	m.Path = types.StringValue(p)
	m.ID = types.StringValue(p)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

// reconcileLookupServer reads back the effective lookup-server URL, keeping the
// declared form when it is semantically equal (0-diff) and surfacing the live
// value on drift. An empty result means lookup-server upload is disabled.
func reconcileLookupServer(occ *nextcloud.OCC, declared string) string {
	upload, err := occ.ConfigAppGet(nextcloud.FilesSharingApp, nextcloud.KeyLookupServerUpload)
	if err != nil || !nextcloud.ParseFederationBool(upload) {
		if nextcloud.NormalizeServerURL(declared) == "" {
			return declared
		}
		return ""
	}
	live, err := occ.ConfigSystemGet(nextcloud.SystemKeyLookupServer)
	if err != nil {
		return ""
	}
	if nextcloud.NormalizeServerURL(declared) == nextcloud.NormalizeServerURL(live) {
		return declared
	}
	return nextcloud.NormalizeServerURL(live)
}

// Delete is a no-op: the federated-sharing settings persist; the resource just
// stops managing them.
func (r *federationConfigResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

func (r *federationConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("path"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

// SPDX-License-Identifier: AGPL-3.0-or-later

package provider

import (
	"context"

	"github.com/JamesonRGrieve/tofu-nextcloud/internal/nextcloud"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// nextcloud_trusted_server manages a single federated Nextcloud peer in the
// `federation` app's trusted-server list via
// `occ federation:trusted-servers:add/remove/list`. It realizes a
// netbox-federation FederationPeer (protocol nextcloud_federated_sharing) at the
// consumer layer. Imports to 0-diff by peer URL; changing the URL replaces the
// resource.
var (
	_ resource.Resource                = (*trustedServerResource)(nil)
	_ resource.ResourceWithConfigure   = (*trustedServerResource)(nil)
	_ resource.ResourceWithImportState = (*trustedServerResource)(nil)
)

// NewTrustedServerResource constructs the nextcloud_trusted_server resource.
func NewTrustedServerResource() resource.Resource { return &trustedServerResource{} }

type trustedServerResource struct {
	client *providerClient
}

type trustedServerModel struct {
	ID   types.String `tfsdk:"id"`
	Path types.String `tfsdk:"path"`
	URL  types.String `tfsdk:"url"`
}

func (r *trustedServerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_trusted_server"
}

func (r *trustedServerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A federated Nextcloud peer in the `federation` app trusted-server list, managed via " +
			"`occ federation:trusted-servers:add/remove/list`. Realizes a netbox-federation FederationPeer " +
			"(protocol `nextcloud_federated_sharing`). Imports to 0-diff by peer URL.",
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
			"url": schema.StringAttribute{
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
				MarkdownDescription: "The federated peer's base URL (e.g. `https://cloud.example.org`). A trailing slash is normalized. Changing it replaces the resource.",
			},
		},
	}
}

func (r *trustedServerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req, resp)
}

func (r *trustedServerResource) docroot() string {
	if r.client == nil {
		return defaultDocroot
	}
	return r.client.Docroot
}

func (r *trustedServerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m trustedServerModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p := resolvePath(m.Path, r.docroot())
	url := nextcloud.NormalizeServerURL(m.URL.ValueString())
	if r.client != nil && r.client.SSH != nil {
		occ := r.client.occ(p)
		// Idempotent: add only when the peer is not already trusted.
		list, err := occ.TrustedServersList()
		if err != nil {
			resp.Diagnostics.AddError("occ federation:trusted-servers:list failed", err.Error())
			return
		}
		if !nextcloud.TrustedServerPresent(list, url) {
			if err := occ.TrustedServerAdd(url); err != nil {
				resp.Diagnostics.AddError("occ federation:trusted-servers:add "+url+" failed", err.Error())
				return
			}
		}
	}
	m.Path = types.StringValue(p)
	m.URL = types.StringValue(url)
	m.ID = types.StringValue(url)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *trustedServerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m trustedServerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil || r.client.SSH == nil {
		return
	}
	p := resolvePath(m.Path, r.docroot())
	occ := r.client.occ(p)
	list, err := occ.TrustedServersList()
	if err != nil {
		resp.Diagnostics.AddError("occ federation:trusted-servers:list failed", err.Error())
		return
	}
	url := nextcloud.NormalizeServerURL(m.URL.ValueString())
	if !nextcloud.TrustedServerPresent(list, url) {
		// Peer no longer trusted on the device → drop from state.
		resp.State.RemoveResource(ctx)
		return
	}
	m.Path = types.StringValue(p)
	m.URL = types.StringValue(url)
	m.ID = types.StringValue(url)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

// Update only carries path/id/url forward: url changes force replacement, so an
// Update never mutates the peer on the device.
func (r *trustedServerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m trustedServerModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p := resolvePath(m.Path, r.docroot())
	url := nextcloud.NormalizeServerURL(m.URL.ValueString())
	m.Path = types.StringValue(p)
	m.URL = types.StringValue(url)
	m.ID = types.StringValue(url)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *trustedServerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m trustedServerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client == nil || r.client.SSH == nil {
		return
	}
	occ := r.client.occ(resolvePath(m.Path, r.docroot()))
	if err := occ.TrustedServerRemove(nextcloud.NormalizeServerURL(m.URL.ValueString())); err != nil {
		resp.Diagnostics.AddError("occ federation:trusted-servers:remove failed", err.Error())
	}
}

func (r *trustedServerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import id is the peer URL; path defaults to the provider docroot on Read.
	url := nextcloud.NormalizeServerURL(req.ID)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("url"), url)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), url)...)
}

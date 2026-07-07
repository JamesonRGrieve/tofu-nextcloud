// SPDX-License-Identifier: AGPL-3.0-or-later

// Package provider implements the nextcloud OpenTofu/Terraform provider — a
// native manager for Nextcloud installed state (occ core install/upgrade),
// config.php system config (config:system), app install/enable state (app:*),
// and per-app config (config:app), driven over an SSH + occ transport. Secrets
// (Nextcloud admin password, DB password) are injected at apply from the secret
// store as write-only values and are NEVER stored in state.
package provider

import (
	"context"
	"time"

	"github.com/JamesonRGrieve/tofu-nextcloud/internal/nextcloud"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ provider.Provider = (*nextcloudProvider)(nil)

// defaultDocroot is the conventional Nextcloud document root (holds occ);
// resources that omit `path` inherit it.
const defaultDocroot = "/var/www/nextcloud"

// New returns the provider factory for a given version.
func New(version string) func() provider.Provider {
	return func() provider.Provider { return &nextcloudProvider{version: version} }
}

type nextcloudProvider struct {
	version string
}

// providerClient is the shared per-provider state handed to every resource: the
// SSH transport, the default docroot, and the web-server user occ runs as.
type providerClient struct {
	SSH     *nextcloud.SSHClient
	Docroot string
	WebUser string
}

type providerModel struct {
	Host       types.String `tfsdk:"host"`
	Port       types.Int64  `tfsdk:"port"`
	User       types.String `tfsdk:"user"`
	SSHKeyFile types.String `tfsdk:"ssh_key_file"`
	SSHKeyPEM  types.String `tfsdk:"ssh_key_pem"`
	Docroot    types.String `tfsdk:"docroot"`
	WebUser    types.String `tfsdk:"web_user"`
	Timeout    types.Int64  `tfsdk:"timeout"`
}

func (p *nextcloudProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	// Single-token type name → resources are `nextcloud_*` (the source address is
	// jamesonrgrieve/nextcloud).
	resp.TypeName = "nextcloud"
	resp.Version = p.version
}

func (p *nextcloudProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Native provider for Nextcloud installed state, config.php system config, app " +
			"install/enable state, and per-app config, driven over SSH + occ. Reads its source of truth from " +
			"netbox-services ServiceInstance params at the consumer layer (no dedicated NetBox plugin); DB and " +
			"admin secrets are injected at apply from OpenBao and never stored in state.",
		Attributes: map[string]schema.Attribute{
			"host": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Nextcloud host address (host or host:port), no scheme, reached over SSH.",
			},
			"port": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "SSH port (default: the port in `host`, else 22).",
			},
			"user": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "SSH user (default `root`). occ is run as `web_user` via sudo.",
			},
			"ssh_key_file": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Path to an SSH identity file (`ssh -i`). When empty, ssh_config/agent is used.",
			},
			"ssh_key_pem": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				MarkdownDescription: "SSH private-key material (e.g. an OpenBao-signed key). Materialized to a temp " +
					"0600 file per call; available at plan time, unlike a Terraform-written key file. Auth is key/cert only.",
			},
			"docroot": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Default Nextcloud document root (holds `occ`) inherited by any resource that " +
					"omits `path`. Default `" + defaultDocroot + "`.",
			},
			"web_user": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Web-server user occ is run as via `sudo -u` (Nextcloud data/config must be owned " +
					"by it). Default `" + nextcloud.DefaultWebUser + "`.",
			},
			"timeout": schema.Int64Attribute{
				Optional: true,
				MarkdownDescription: "Per-command SSH timeout in seconds (default 120). Raise it for slow operations " +
					"such as an `occ upgrade` on a large instance.",
			},
		},
	}
}

func (p *nextcloudProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var timeout time.Duration
	if !cfg.Timeout.IsNull() && !cfg.Timeout.IsUnknown() && cfg.Timeout.ValueInt64() > 0 {
		timeout = time.Duration(cfg.Timeout.ValueInt64()) * time.Second
	}
	ssh := nextcloud.NewSSHClient(nextcloud.SSHConfig{
		Host:    cfg.Host.ValueString(),
		Port:    int(cfg.Port.ValueInt64()),
		User:    cfg.User.ValueString(),
		KeyFile: cfg.SSHKeyFile.ValueString(),
		KeyPEM:  cfg.SSHKeyPEM.ValueString(),
		Timeout: timeout,
	})
	docroot := defaultDocroot
	if !cfg.Docroot.IsNull() && cfg.Docroot.ValueString() != "" {
		docroot = cfg.Docroot.ValueString()
	}
	webUser := nextcloud.DefaultWebUser
	if !cfg.WebUser.IsNull() && cfg.WebUser.ValueString() != "" {
		webUser = cfg.WebUser.ValueString()
	}
	client := &providerClient{SSH: ssh, Docroot: docroot, WebUser: webUser}
	resp.ResourceData = client
	resp.DataSourceData = client
}

func (p *nextcloudProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewCoreResource,
		NewConfigResource,
		NewAppResource,
		NewAppConfigResource,
		NewTrustedServerResource,
		NewFederationConfigResource,
	}
}

func (p *nextcloudProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}

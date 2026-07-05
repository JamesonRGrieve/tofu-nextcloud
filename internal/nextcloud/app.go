// SPDX-License-Identifier: AGPL-3.0-or-later
//
// App install/enable-state helpers. Nextcloud apps are managed with `occ
// app:install/enable/disable/remove/update` and inspected with `occ app:list
// --output=json`. These pure helpers build the occ argument vectors and parse
// the app:list JSON into a per-app enabled/version view — so the nextcloud_app
// resource can drive install-state and detect drift. All pure — unit-tested.
package nextcloud

import (
	"encoding/json"
	"fmt"
	"strings"
)

// AppState is the enabled flag and version of a single app.
type AppState struct {
	Enabled bool
	Version string
}

// appInstallArgs installs (and enables) an app from the app store (`occ
// app:install`).
func appInstallArgs(id string) []string {
	return []string{"app:install", id}
}

// appEnableArgs enables an already-present app (`occ app:enable`).
func appEnableArgs(id string) []string {
	return []string{"app:enable", id}
}

// appDisableArgs disables an app (`occ app:disable`).
func appDisableArgs(id string) []string {
	return []string{"app:disable", id}
}

// appRemoveArgs uninstalls an app (`occ app:remove`).
func appRemoveArgs(id string) []string {
	return []string{"app:remove", id}
}

// appUpdateArgs updates an app to the latest available version (`occ
// app:update`).
func appUpdateArgs(id string) []string {
	return []string{"app:update", id}
}

// appListArgs lists all apps as JSON (`occ app:list --output=json`).
func appListArgs() []string {
	return []string{"app:list", "--output=json"}
}

// parseAppList parses `occ app:list --output=json`. The JSON has two objects,
// `enabled` and `disabled`, each mapping app id → version (a disabled app's value
// may be the literal `false` instead of a version string). Every app is returned
// keyed by id with its enabled flag and best-known version.
func parseAppList(b []byte) (map[string]AppState, error) {
	var raw struct {
		Enabled  map[string]json.RawMessage `json:"enabled"`
		Disabled map[string]json.RawMessage `json:"disabled"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("parse occ app:list json: %w", err)
	}
	out := make(map[string]AppState, len(raw.Enabled)+len(raw.Disabled))
	for id, v := range raw.Enabled {
		out[id] = AppState{Enabled: true, Version: rawVersion(v)}
	}
	for id, v := range raw.Disabled {
		out[id] = AppState{Enabled: false, Version: rawVersion(v)}
	}
	return out, nil
}

// rawVersion extracts a version string from an app:list value. A JSON string is
// unquoted; a non-string (e.g. the boolean `false`) yields an empty version.
func rawVersion(v json.RawMessage) string {
	var s string
	if err := json.Unmarshal(v, &s); err == nil {
		return strings.TrimSpace(s)
	}
	return ""
}

// AppInstall installs (and enables) an app from the app store (`occ
// app:install`).
func (o *OCC) AppInstall(id string) error {
	_, err := o.run(appInstallArgs(id)...)
	return err
}

// AppEnable enables an already-present app (`occ app:enable`).
func (o *OCC) AppEnable(id string) error {
	_, err := o.run(appEnableArgs(id)...)
	return err
}

// AppDisable disables an app (`occ app:disable`).
func (o *OCC) AppDisable(id string) error {
	_, err := o.run(appDisableArgs(id)...)
	return err
}

// AppRemove uninstalls an app (`occ app:remove`).
func (o *OCC) AppRemove(id string) error {
	_, err := o.run(appRemoveArgs(id)...)
	return err
}

// AppUpdate updates an app to the latest available version (`occ app:update`).
func (o *OCC) AppUpdate(id string) error {
	_, err := o.run(appUpdateArgs(id)...)
	return err
}

// AppEnabled reports whether the named app is present and enabled in a parsed
// app list.
func AppEnabled(apps map[string]AppState, id string) bool {
	s, ok := apps[id]
	return ok && s.Enabled
}

// AppPresent reports whether the named app is installed (enabled or disabled).
func AppPresent(apps map[string]AppState, id string) bool {
	_, ok := apps[id]
	return ok
}

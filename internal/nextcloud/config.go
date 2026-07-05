// SPDX-License-Identifier: AGPL-3.0-or-later
//
// config.php (config:system) rendering helpers. Nextcloud system config values
// are typed — most are strings, some are integers or booleans, and nested keys
// (e.g. memcache.local, overwrite.cli.url, trusted_domains.1) are addressed by a
// dotted path split into positional occ arguments. These pure helpers classify a
// key's type, split its name path, build the occ argument vectors, and implement
// the manage-declared-only reconcile — so the nextcloud_config resource issues
// the correct `occ config:system:*` for each key. All pure — unit-tested.
package nextcloud

import "strings"

// Config value types accepted by `occ config:system:set --type`.
const (
	TypeString  = "string"
	TypeInteger = "integer"
	TypeBoolean = "boolean"
)

// typedSystemKeys maps the well-known non-string system config keys to their occ
// `--type`. Anything not listed is written as a string. Nested keys use their
// full dotted name (matching the declared map key).
var typedSystemKeys = map[string]string{
	// integers
	"maintenance_window_start": TypeInteger,
	"loglevel":                 TypeInteger,
	"log_rotate_size":          TypeInteger,
	"redis.port":               TypeInteger,
	"redis.dbindex":            TypeInteger,
	"redis.timeout":            TypeInteger,
	// booleans
	"maintenance":                        TypeBoolean,
	"debug":                              TypeBoolean,
	"updatechecker":                      TypeBoolean,
	"filelocking.enabled":                TypeBoolean,
	"mysql.utf8mb4":                      TypeBoolean,
	"check_for_working_wellknown":        TypeBoolean,
	"auth.bruteforce.protection.enabled": TypeBoolean,
}

// SystemConfigType returns the occ `--type` for a system config key. Unknown
// keys default to string.
func SystemConfigType(key string) string {
	if t, ok := typedSystemKeys[key]; ok {
		return t
	}
	return TypeString
}

// splitConfigName splits a dotted config key into the positional name segments
// occ expects (`memcache.local` → ["memcache", "local"], `trusted_domains.1` →
// ["trusted_domains", "1"]). A key with no dot yields a single-element slice.
func splitConfigName(key string) []string {
	return strings.Split(key, ".")
}

// configSystemSetArgs builds the `occ config:system:set` argument vector for a
// key/value. A non-string type adds `--type=`; a string type omits it (occ's
// default). Value goes through `--value=` so it is never confused with a name
// segment.
func configSystemSetArgs(key, value, valueType string) []string {
	args := append([]string{"config:system:set"}, splitConfigName(key)...)
	args = append(args, "--value="+value)
	if valueType != "" && valueType != TypeString {
		args = append(args, "--type="+valueType)
	}
	return args
}

// configSystemGetArgs builds the `occ config:system:get` argument vector.
func configSystemGetArgs(key string) []string {
	return append([]string{"config:system:get"}, splitConfigName(key)...)
}

// configSystemDeleteArgs builds the `occ config:system:delete` argument vector.
func configSystemDeleteArgs(key string) []string {
	return append([]string{"config:system:delete"}, splitConfigName(key)...)
}

// configAppSetArgs builds the `occ config:app:set` argument vector.
func configAppSetArgs(app, key, value string) []string {
	return []string{"config:app:set", app, key, "--value=" + value}
}

// configAppGetArgs builds the `occ config:app:get` argument vector.
func configAppGetArgs(app, key string) []string {
	return []string{"config:app:get", app, key}
}

// configAppDeleteArgs builds the `occ config:app:delete` argument vector.
func configAppDeleteArgs(app, key string) []string {
	return []string{"config:app:delete", app, key}
}

// NormalizeConfigValue canonicalizes a value for drift comparison so that state
// (what `occ config:system:get` returns) and config match. Booleans are folded
// to "true"/"false" (occ prints them that way); integers/strings are trimmed.
func NormalizeConfigValue(key, value string) string {
	v := strings.TrimSpace(value)
	if SystemConfigType(key) != TypeBoolean {
		return v
	}
	switch strings.ToLower(v) {
	case "true", "1":
		return "true"
	case "false", "0", "":
		return "false"
	default:
		return strings.ToLower(v)
	}
}

// ReconcileConfigValue implements manage-declared-only diff for a single key:
// given the declared value and the value read back from the device, it returns
// the value to store in state. When the two are semantically equal it keeps the
// declared form (no spurious diff), otherwise it surfaces the device value so
// drift is visible.
func ReconcileConfigValue(key, declared, live string) string {
	if NormalizeConfigValue(key, declared) == NormalizeConfigValue(key, live) {
		return declared
	}
	return live
}

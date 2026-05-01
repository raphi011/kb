package views

import "sync/atomic"

// assetMap holds logical → fingerprinted asset names installed by SetAssets.
// nil means "no manifest available" — Asset falls back to the un-fingerprinted
// /static/<name> URL, which is the right behavior for dev mode where bundles
// are written under their canonical names without hashing.
var assetMap atomic.Pointer[map[string]string]

// SetAssets installs the logical → fingerprinted asset name map. The server
// calls this at startup with the contents of static/dist/manifest.json (when
// present). m is taken as immutable; callers should not mutate it after the
// call.
func SetAssets(m map[string]string) {
	assetMap.Store(&m)
}

// Asset returns the URL for the asset with the given logical name (e.g.
// "app.min.js"). When a fingerprinted variant is registered the URL points
// inside /static/dist/ with a 1-year-immutable cache. Otherwise it falls
// through to /static/<name>, which is served with no-cache headers.
func Asset(name string) string {
	if m := assetMap.Load(); m != nil {
		if hashed, ok := (*m)[name]; ok {
			return "/static/dist/" + hashed
		}
	}
	return "/static/" + name
}

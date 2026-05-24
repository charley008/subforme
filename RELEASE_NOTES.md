# v1.1.1

## New

- Added third-party Provider conversion. SubForMe can fetch an external subscription, extract `proxies`, store it under `proxy_providers`, refresh it on a schedule, and expose it through the local Provider endpoint.
- Added a dedicated "Third-party Providers" page for creating, refreshing, editing, and deleting converted Providers.
- User subscriptions can now include selected third-party Providers. Generated configs add matching `url-test` groups and `proxy-providers` entries automatically.
- Added admin operation logs for login and panel actions, making Docker console logs more useful.

## Fixed

- Cleaned stale `app.yaml` references when users, nodes, groups, or third-party Providers are deleted or changed.
- Prevented deleted/unselected Provider names from remaining in generated proxy groups.
- Protected Provider file serving against path traversal.
- Improved 3x-ui API compatibility by trying common API base path variants.
- Kept the last valid Provider YAML file when a refresh fails, so existing subscriptions do not break because of temporary upstream errors.

## Changed

- Default release config is now sanitized for public use: no test panel URL, test API key, test nodes, or bundled third-party Provider.
- Server management now states that only 3x-ui is currently supported.
- `monaco-editor` updated to `0.53.0`.

## Verification

- `go test ./...`
- `npm.cmd run build`

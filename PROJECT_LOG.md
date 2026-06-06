## [2026-06-03 11:40]

### Goal
Fix the dashboard traffic view so a server traffic reset does not bounce back to stale values after refresh.

### Findings
- `ResetServerUserTraffic` only reset counters in 3xui and did not update the local `user_traffic` cache.
- The dashboard triggered `/api/traffic/refresh` immediately after reset, which could pull stale stats back from 3xui before its counters fully settled.

### Actions
- Added a DB helper to zero cached traffic rows for a specific server and set of emails.
- Cleared local cached traffic after a successful server reset.
- Changed the dashboard reset flow to reload stored dashboard traffic instead of forcing an immediate remote refresh.
- Added DB coverage for server-scoped traffic zeroing.

### Modified Files
- backend/internal/db/traffic.go: add server-scoped cache zeroing.
- backend/internal/db/traffic_test.go: cover scoped traffic zeroing.
- backend/internal/app/service.go: clear cached traffic after reset.
- frontend/src/pages/DashboardPage.tsx: reload local dashboard traffic after reset.

### Result
The dashboard now reflects reset traffic immediately from local state and no longer jumps back to stale counters on the next page refresh.

### Next
Verify whether 3xui still returns stale traffic from its query endpoints after manual refresh.

## [2026-06-03 12:35]

### Goal
Redesign traffic collection around panel-level client stats and add per-server timed refresh plus monthly automatic traffic reset.

### Findings
- The latest 3xui `GET /panel/api/clients/list` response includes per-client `traffic`, which matches the current "single user, multiple inbounds" model better than inbound `clientStats`.
- For this product, traffic reset should happen at the panel level for all users, not by looping one email at a time.
- Automatic reset and automatic refresh can collide unless reset also advances the local sync marker.

### Actions
- Switched traffic refresh to read `clients/list` and store `traffic.up/down` per client.
- Switched reset to `POST /panel/api/clients/resetAllTraffics`, then zeroed the local cache for that server.
- Added server schedule fields for traffic refresh interval, monthly reset enable/day/time/timezone, last sync timestamp, and last reset month key.
- Reworked the traffic background job into a per-minute scheduler that decides per server whether to refresh traffic or run the monthly reset.
- Added server form controls for the new traffic scheduling settings.
- Added tests for schema persistence, scoped traffic replacement, scheduler timing helpers, and the new xui reset-all endpoint.

### Modified Files
- backend/internal/app/service.go
- backend/internal/db/db.go
- backend/internal/db/db_test.go
- backend/internal/db/models.go
- backend/internal/db/servers.go
- backend/internal/db/traffic.go
- backend/internal/db/traffic_test.go
- backend/internal/xui/client.go
- backend/internal/xui/client_test.go
- backend/internal/xui/models.go
- frontend/src/pages/DashboardPage.tsx
- frontend/src/pages/ServersPage.tsx

### Result
Traffic collection now follows 3xui client-level counters, manual reset operates on the full panel, and each server can independently refresh traffic on a configured interval and reset traffic monthly.

### Next
Run one end-to-end validation on a real panel: manual reset, delayed dashboard refresh, and one scheduled monthly reset window.

## [2026-06-03 12:55]

### Goal
Speed up manual traffic refresh when one or more 3xui panels are slow or offline.

### Findings
- Traffic refresh was fetching every enabled server serially.
- Each xui client request allowed up to 15 seconds, so one dead panel could stall the whole dashboard refresh.

### Actions
- Switched `RefreshTraffic` to fetch enabled servers concurrently.
- Added a dedicated 5-second timeout for per-server traffic refresh requests.
- Added per-server refresh duration logs to make slow panels visible in backend logs.
- Rebuilt the local `release/subforme.exe` test binary.

### Modified Files
- backend/internal/app/service.go
- release/subforme.exe

### Result
Dashboard traffic refresh now waits roughly for the slowest healthy panel instead of the sum of all panels, and offline panels time out much faster.

### Next
Verify refresh latency against a mix of healthy and offline panels, then tune the 5-second timeout if needed.

## [2026-06-06 12:10]

### Goal
Add an iOS/mobile subscription template variant for memory-constrained clients.

### Findings
- The existing subscription generator selected only the user's normal mode template (`whitelist` or `blacklist`).
- The template editor supported only `base`, `whitelist`, and `blacklist`.
- The public subscription URL can support a template variant without changing existing links.

### Actions
- Added an `ios.yaml` template, initially copied from the default direct template.
- Added `ios` as a supported template editor section.
- Added support for `/api/sub?user=<name>&type=ios`.
- Added an iOS subscription option in the user page share controls.
- Rebuilt `release/subforme.exe` and refreshed `release/web` for local testing.

### Modified Files
- backend/config/templates/ios.yaml: new iOS/mobile template seed.
- backend/internal/config/template_sections.go: manage the `ios` template section.
- backend/internal/app/service.go: select the iOS template for subscription variants.
- backend/internal/web/public.go: read the `type` subscription variant parameter.
- frontend/src/pages/TemplatesPage.tsx: expose the iOS/mobile template tab.
- frontend/src/pages/UserPreviewPage.tsx: add the iOS subscription copy option.

### Result
Existing subscription links are unchanged, while iOS/mobile clients can use `type=ios` to receive the dedicated template.

### Next
Tune `ios.yaml` rules later to reduce memory usage for iOS clients with strict memory limits.

## [2026-06-06 20:52]

### Goal
Publish v1.3.6 with the iOS subscription template and user share-link selector.

### Findings
- Version metadata still pointed at v1.3.5.
- The repository did not yet include a license file.
- Release notes should stay focused on user-facing subscription changes.

### Actions
- Bumped the app version to v1.3.6.
- Added a concise v1.3.6 release note.
- Added an MIT license file.
- Rebuilt the frontend and local release executable.

### Modified Files
- backend/internal/config/types.go: update displayed API version.
- RELEASE_NOTES.md: add concise v1.3.6 release notes.
- LICENSE: add MIT license.
- release/subforme.exe and release/web: refreshed local test artifacts.

### Result
v1.3.6 is ready to commit, tag, push, and publish on GitHub.

### Next
Create the git commit, tag v1.3.6, push to GitHub, and create the GitHub release.

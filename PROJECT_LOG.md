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

## [2026-06-08 15:03]

### Goal
Keep admin login active after closing and reopening the browser.

### Findings
- The backend was issuing `subforme_session` as a session cookie.
- Session cookies are normally removed when the browser is closed, which forced a new login.

### Actions
- Added a 30-day `MaxAge` and `Expires` to the session cookie.
- Added a test to ensure successful login returns a persistent cookie.
- Rebuilt the local `release/subforme.exe`.

### Modified Files
- backend/internal/auth/session.go: make the admin session cookie persistent.
- backend/internal/web/auth_test.go: cover persistent login cookie behavior.
- release/subforme.exe: refreshed local test executable.

### Result
Closing and reopening the browser should keep the admin logged in until the cookie expires or the user logs out.

### Next
If a configurable duration is needed later, expose the session lifetime in runtime config.

## [2026-06-08 15:14]

### Goal
Improve mobile usability, add backup/restore, and update default subscription templates.

### Findings
- The users page was desktop-table oriented and awkward on small screens.
- A safe restore flow should avoid overwriting SQLite while the process is running.
- The default direct, proxy, and iOS templates needed to be replaced with the user's tuned rules.

### Actions
- Added a mobile card layout for the users page while keeping the desktop table layout.
- Added authenticated backup export and restore upload endpoints.
- Implemented a pending restore archive that is applied automatically before database startup, then wired restore upload to restart the app automatically.
- Added backup/restore controls to the settings page.
- Replaced `whitelist.yaml`, `blacklist.yaml`, and `ios.yaml` defaults.
- Rebuilt the local release executable and static web assets.

### Modified Files
- backend/internal/web/backup.go: add backup export and restore upload endpoints.
- backend/internal/config/restore.go: apply pending restore archives safely at startup.
- backend/cmd/subforme/main.go: apply pending restore before opening the database.
- frontend/src/pages/UserPreviewPage.tsx: add mobile card layout.
- frontend/src/pages/XuiSettingsPage.tsx: add backup and restore controls.
- frontend/src/styles/theme.css: add responsive user card and modal styling.
- backend/config/templates/*.yaml: update default direct, proxy, and iOS templates.

### Result
The users page is usable on phones, backups can be exported/restored with an automatic restart, and new installs/release templates use the updated rule sets.

### Next
Validate the restore flow with a real backup before relying on it for production recovery.

## [2026-06-08 15:17]

### Goal
Record the user's manual update to the default proxy template.

### Findings
- The user manually edited `backend/config/templates/blacklist.yaml`.
- The template now keeps the large Loyalsoldier providers commented out.
- Active rule providers are `lancidr` and `xiaoshuo`.
- The default proxy template routes unmatched traffic with `MATCH,PROXY`.

### Actions
- Reviewed the current `blacklist.yaml` diff and content.
- Recorded the manual template change without modifying the file.

### Modified Files
- PROJECT_LOG.md: document the manual default proxy template edit.

### Result
The manual default proxy template change is now captured in project memory.

### Next
Preserve this edited `blacklist.yaml` when continuing template or release work.

## [2026-06-08 15:21]

### Goal
Fix the mobile users page layout where the left sidebar consumed too much screen width.

### Findings
- The 1024px responsive layout collapsed the sidebar to a 60px fixed left rail.
- On phones, that left rail still consumed horizontal space and made user cards clip on the right.
- The desktop layout should remain unchanged.

### Actions
- Added a phone-only `max-width: 768px` layout that turns the sidebar into a bottom horizontal navigation bar.
- Removed the mobile left margin from the main content and added bottom padding for the nav bar.
- Kept the desktop table and tablet collapsed-sidebar behavior unchanged above the phone breakpoint.
- Rebuilt the local release executable and static web assets.

### Modified Files
- frontend/src/styles/theme.css: adjust phone-only sidebar and main content layout.
- release/subforme.exe and release/web: refreshed local test artifacts.

### Result
Phone screens now get full-width content with a bottom navigation bar, while PC layout remains the existing table/sidebar layout.

### Next
Validate on a real phone browser and tune the bottom nav label density if needed.

## [2026-06-08 15:23]

### Goal
Restore visible sidebar text on narrow desktop/tablet widths.

### Findings
- The 1024px responsive rule hid `.nav-label`, but the sidebar has no separate icons.
- This made navigation items appear as blank buttons before the phone bottom-nav breakpoint.

### Actions
- Changed the 1024px breakpoint to keep a readable 168px sidebar with labels visible.
- Kept the 768px phone breakpoint as the bottom navigation layout.
- Rebuilt the local release executable and static web assets.

### Modified Files
- frontend/src/styles/theme.css: keep sidebar labels visible on narrow non-phone layouts.
- release/subforme.exe and release/web: refreshed local test artifacts.

### Result
Sidebar text remains visible on PC/tablet widths, while phones still use the bottom navigation layout.

### Next
Check the phone browser after cache refresh to confirm the bottom nav labels are visible.

## [2026-06-08 15:25]

### Goal
Make mobile navigation visible after the bottom nav was obscured on iPhone Safari.

### Findings
- The phone layout moved navigation to the bottom.
- On iPhone Safari, the browser bottom toolbar can obscure that area, making the navigation look missing.

### Actions
- Changed the phone navigation from bottom fixed layout to a top sticky horizontal navigation bar.
- Kept main content full-width on phone screens.
- Rebuilt the local release executable and static web assets.

### Modified Files
- frontend/src/styles/theme.css: move phone navigation to the top.
- release/subforme.exe and release/web: refreshed local test artifacts.

### Result
Phone navigation should now be visible at the top of the app instead of being hidden behind Safari controls.

### Next
Validate on the iPhone screenshot scenario and adjust nav item spacing if needed.

## [2026-06-08 15:27]

### Goal
Make all mobile navigation items visible without horizontal swiping.

### Findings
- The top mobile navigation used horizontal scrolling.
- On phones, users may not notice that more navigation items are available off-screen.

### Actions
- Changed the phone navigation to a 4-column grid.
- Allowed longer labels to wrap instead of requiring horizontal scrolling.
- Rebuilt the local release executable and static web assets.

### Modified Files
- frontend/src/styles/theme.css: show all phone navigation items as a compact grid.
- release/subforme.exe and release/web: refreshed local test artifacts.

### Result
All main navigation entries should now be visible at once on mobile screens.

### Next
Validate on iPhone and reduce to 3 columns if labels feel too cramped.

## [2026-06-08 15:30]

### Goal
Restore theme and logout controls on the mobile navigation layout.

### Findings
- The phone layout hid the sidebar footer.
- That also hid the light/dark theme toggle and logout button.

### Actions
- Kept the version label hidden on phone screens.
- Displayed the sidebar footer as a compact 2-column grid containing theme toggle and logout.
- Rebuilt the local release executable and static web assets.

### Modified Files
- frontend/src/styles/theme.css: show mobile theme/logout controls.
- release/subforme.exe and release/web: refreshed local test artifacts.

### Result
Mobile users can now switch light/dark mode and log out without leaving the phone layout.

### Next
Validate on iPhone that the mobile navigation grid plus footer controls are readable.

## [2026-06-08 15:36]

### Goal
Use fixed frontend asset filenames in the release web build.

### Findings
- Vite's default hashed asset names are useful for cache busting but made the local release directory hard to read.
- Old hashed files accumulated because release web assets were copied without clearing the target directory first.

### Actions
- Configured Vite/Rollup output to emit `assets/index.js` and `assets/index.css`.
- Added `Cache-Control: no-cache` for served frontend assets and `index.html` to reduce stale-cache issues with fixed filenames.
- Cleared `release/web` before copying the latest frontend build.
- Rebuilt the local release executable and static web assets.

### Modified Files
- frontend/vite.config.ts: emit fixed frontend asset filenames.
- backend/internal/web/public.go: set no-cache headers for frontend assets.
- release/subforme.exe and release/web: refreshed local test artifacts with only fixed asset files.

### Result
The release web directory now contains only `index.html`, `assets/index.js`, and `assets/index.css`.

### Next
Keep clearing `release/web` before copying frontend builds so removed files do not linger.

## [2026-06-08 15:38]

### Goal
Prepare and publish v1.3.7.

### Findings
- Local validation completed successfully after the mobile layout, backup/restore, template, login-session, and fixed-asset changes.
- App version still needed to be bumped from v1.3.6 to v1.3.7.

### Actions
- Updated the app version constant to v1.3.7.
- Added concise v1.3.7 release notes.
- Rebuilt frontend assets and local release executable.

### Modified Files
- backend/internal/config/types.go: bump version to v1.3.7.
- RELEASE_NOTES.md: add v1.3.7 notes.
- release/subforme.exe and release/web: refreshed local test artifacts.

### Result
v1.3.7 is ready to commit, tag, push, and publish on GitHub.

### Next
Commit the release, push `main`, push tag `v1.3.7`, and create the GitHub release.

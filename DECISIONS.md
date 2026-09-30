# Architecture decisions

## 2026-09-30: Server-side administrator sessions

Use independent 256-bit random tokens issued by `crypto/rand`. Keep only their SHA-256 hashes and expiry times in a mutex-protected store owned by the HTTP router. The server enforces the existing 30-day lifetime. Logout revokes the current token; password changes revoke all old tokens and issue a replacement to the current browser. Serialize login and password changes so an old password cannot issue a session after revocation.

The public subscription URL and user/node synchronization behavior are intentionally unchanged for personal use.

Rejected alternative: retain signed stateless cookies and generate a signing secret. This would still require a revocation store for logout and password changes. Opaque tokens remove the dependency on user-supplied signing secrets. Legacy `session_secret` settings remain readable but are not used for authentication.

Tradeoff: sessions are in memory. Restarting or upgrading requires signing in again, and independent server instances do not share sessions. Database-backed sessions can be added later if persistence is needed.

## 2026-09-30: Preserve SQLite DELETE journaling

Set `busy_timeout(5000)` and `cache_size(-20000)` using the `_pragma` DSN parameter supported by the pinned modernc.org/sqlite version. Explicitly preserve the actual pre-change `DELETE` journal mode. Regression tests query these values on both the initial connection and a reopened connection.

Rejected alternative: activate the previously intended WAL mode in this patch. Existing backup/export directly copies database files and restore overwrites files; switching to WAL also requires consistent database snapshots and handling stale WAL files. That should be a coordinated backup/restore change.

Tradeoff: this patch does not gain WAL's reader/writer concurrency. The application retains its single-connection configuration. Existing live-file backups still lack a consistent SQLite snapshot; this decision does not resolve that pre-existing issue.

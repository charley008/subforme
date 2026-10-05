# Release version

- The single source of truth for the application release version is `backend/internal/config/VERSION` (format: `vX.Y.Z`).
- When the user requests a version change, update this file. The backend embeds it, and `/api/version` supplies the sidebar display. Docker and ordinary Go builds require no version injection.
- Update the corresponding release notes; create the GitHub tag using the exact value from VERSION. CI rejects tags that do not match this file.
- `frontend/package.json` is the private frontend package version, not the application release version. Historical release notes keep their original version headings.
- For same-version republication, retain the existing version, add the fix as a new commit, and move the release tag only when authorized by the user.

# FabricWorld Maintenance Entry Point

This project is for personal use: validate each change locally before release; there is no full-regression gate, and permanent tests are neither added nor retained by default. For UI changes, check the computer and phone scenarios that are actually used. Validate data migrations, bulk writes/deletes, and backup recovery with an isolated copy first.

GitHub backs up source code and configuration only; pushing does not trigger tests or deployment. See [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) for the local-machine entry points and rollback procedure. Shared procedures are maintained by `server-operations`.

Read [docs/README.md](docs/README.md) first, then read the architecture, API, operations, and local-release documentation as needed.

Agent instructions, maintenance documentation, technical plans, handoffs, and similar agent-authored operational documentation default to English unless the user explicitly requests another language. Product and user-facing output follows the requested language. Preserve exact non-English UI labels, matching literals, and quotations when translation would change meaning or behavior.

- The shared fabric and finished-work libraries use centralized device authentication. Authorized devices can view, edit, delete, and export. The initial version is online-only, and the server is the authoritative data source. A finished work only links to its fabric sources; update remnants manually from the fabric details page.
- Go + Vue + SQLite + libvips. Back up photos together with the database. Do not copy an active WAL database's main file, and do not overwrite saved photos in place.
- Store dimensions as integer millimeters and monetary amounts as integer cents. Unknown dimensions must not be written as 0. Remnant status, dimension groups, image references, and `revision` must remain consistent.
- Use the shared custom component for simple dropdown options and input suggestions. Keep the system picker for complex controls such as dates and times. Switch between the exact UI sections `布料库` and `成品集` only in the top bar; do not duplicate these entries in the More menu. See [docs/CONTROLS.md](docs/CONTROLS.md) for interaction and animation conventions.
- Writes must support idempotency and version-conflict handling. Tests use isolated synthetic data; production acceptance is read-only by default.
- Explicitly record when the real phone camera has not been verified.
- Use `server-operations` before deployment, and explicitly read `ssh ali 'cat /opt/AGENTS.md'`. This project maintains only its own location, units, data, and deployment identity.
- Update the corresponding `docs` and `deploy` materials. Add new shared state to `agent-config`. After pushing, run `agent-config`'s `skills/server-operations/scripts/sync-docs.sh FabricWorld` to synchronize the server copy.
- Follow only the authorization table in `server-operations`'s `SKILL.md` to decide which tasks to finish and report afterward and which require confirmation first. Documentation maintenance and synchronization do not require advance confirmation.
- The GitHub repository is public. Production addresses, photos, databases, backups, and credentials must not enter Git.

## Portal Resource Synchronization

- When changing the site icon or authentication, also verify the iPhone `apple-touch-icon`, post-build paths, and anonymous GET/HEAD behavior. Only explicit brand-icon and public-manifest exceptions are allowed; pages, APIs, and user media still require authentication. See `server-operations`'s `common-issues` for shared troubleshooting and acceptance criteria; `current-state` is authoritative for what is actually enabled.
- This project's `deploy/portal.json` is the maintenance source for ServerPortal's resource declaration. It records directory purposes, databases, runtime user, port, units, access paths, APIs, and backup types. When adding, moving, or deleting a data root, endpoint, or runtime artifact, update the declaration, the corresponding documentation, and the shared application inventory together.
- The declaration is deployed to `/opt/fabricworld/config/portal.json` and managed by root. Local-machine releases use the shared `server-operations` validator: preflight before release, then synchronize over restricted SSH and verify the portal's loaded hash afterward. `registry.d/fabricworld.json` registers the link automatically; declaration updates do not require a portal restart.
- Shared Nginx and the portal provide centralized authentication; do not maintain another device allowlist in this project. Preserve the shared conventions for local calls and release health checks. Device authentication is enabled in production; after a change, update `server-operations`'s `current-state`.
- The portal's read-only display does not replace this project's native consistent backups. When the backup format or media lifecycle changes, validate the affected backup and restore contracts in isolation. Real business data, credentials, and backups must still never enter Git.

Local-machine deployment automatically preflights and synchronizes the committed `deploy/portal.json`. To update only the declaration, run `make portal`; this preserves the business-program version.

By default, the web application blocks ordinary Tab / Shift+Tab control traversal and removes focus highlights; preserve only user-requested exceptions. Maintain and verify this behavior according to the [server-operations web keyboard and focus conventions](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/conventions.md#网页键盘与焦点).

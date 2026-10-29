# Installation, Operation, and Recovery

The public entry point uses HTTPS with a trusted IP certificate and preserves the existing `/fabricworld/` path. Public HTTP returns 308; API clients should use HTTPS directly. Nginx overwrites `X-Forwarded-Proto`. Write-origin validation trusts proxy headers only from loopback addresses and continues to reject cross-site origins. See [shared HTTPS operations](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/https.md) (server copy: `/opt/server-context/references/https.md`) for certificates, renewal, rollback, and whole-machine acceptance checks. This project's backend and release checks retain local HTTP. The `127.0.0.1:80` proxy-check entry point is not publicly reachable. The public entry point uses ServerPortal's centralized device authentication: first enter the passphrase at `/portal/login` to authorize the device, then use the same-origin Secure/HttpOnly Cookie. Unauthorized API requests return 401. Local release checks and service-to-service calls remain available.

FabricWorld's application target is `nas`; `vps` exposes public HTTPS port 8443 and temporary `ali` access forwards to the same instance. NAS provides local-CA HTTPS for home devices. Use `server-operations`, read the target's remote `/opt/AGENTS.md`, and consult current-state for activation. Shared Nginx and `server-context` do not belong to this project; addresses remain in trusted SSH configuration.

Image-size boundaries: Nginx accepts requests up to 26 MiB, and the program accepts individual images up to 25 MiB; the entry-point margin covers multipart overhead. If an image larger than 1 MiB returns 500 before reaching the program, see [centralized authentication incorrectly blocking large requests](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/common-issues.md#统一认证误拦大请求). The shared correction must preserve this application's upload limit.

## Runtime Layout

- `/opt/fabricworld/bin/fabricworld`: Linux amd64 program with the web application embedded.
- `/opt/fabricworld/config/`: application/maintenance units, preserved legacy backup timer, and Nginx location; managed by root.
- `/opt/fabricworld/data/fabricworld.db`: SQLite database; `media/`, `tmp/`, and `maintenance.lock` are in the same directory.
- `/opt/fabricworld/backups/`: preserved legacy database/media snapshots; no routine NAS backup or upgrade protection is created here.
- `/opt/fabricworld/docs/`, `AGENTS.md`, and `docs/SOURCE`: copies of committed documentation and its source marker.
- `/opt/fabricworld/releases/` and `current-commit`: release artifacts and the running commit.

The runtime user is `fabricworld`, and the release user is `fabricworld-deploy`. `data`/`backups` are mode 0700 and their files are 0600; root manages the other directories and the program. The service listens on `127.0.0.1:18082` and is accessed through `/fabricworld/`. No cloud-security-group port is added.

## Application Cleanup

The NAS migration separates existing application cleanup from the disabled native daily backup job. Its dedicated units are `fabricworld-cleanup.service` and `fabricworld-cleanup.timer`; scheduling is enabled only on a host marked `nas`. The timer retains 03:30 Asia/Shanghai with up to five minutes of randomized delay and persistent catch-up. The [shared inventory](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/current-state.md) records actual installation and activation.

The cleanup service runs `fabricworld cleanup --data /opt/fabricworld/data` as `fabricworld`, with writes limited to `data`, a 256 MiB memory limit, 50% CPU quota, Nice 10, and a 15-minute timeout. It takes the shared maintenance lock, then the exclusive application deployment lock before applying the existing trash/photo/operation cleanup rules. Shared `/etc/tmpfiles.d/server-maintenance.conf` prepares these root-owned 0644 lock files before the units run. Portal registers these maintenance units so full cold backups coordinate their writer pause with the same locks. This operation generates no snapshot and prunes no backup or release material; native backup timers remain disabled on NAS.

## First Installation

Run this only in an authorized new empty directory. First inspect directories, identities, port 18082, the health of all existing applications, and Nginx configuration. Follow `software-installation` for software installation. Dependencies come from the official Ubuntu repositories:

```sh
apt-get --simulate install --no-install-recommends libvips-tools libheif-plugin-libde265
DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=l apt-get install -y --no-install-recommends libvips-tools libheif-plugin-libde265
```

Install only after confirming the package plan. Preserve existing host tools and do not upgrade or remove unrelated services. Build locally with `make linux`, verify the artifact's SHA-256, upload it to an isolated staging directory, and have an administrator run:

```sh
bash deploy/install.sh /path/to/fabricworld-linux-amd64
ln -s /opt/fabricworld/config/nginx-location.conf /etc/nginx/app-locations/fabricworld.conf
nginx -t
systemctl reload nginx
```

`install` rejects an existing installation and explicitly initializes an empty database. Before integration, complete photo and backup-recovery verification with an isolated test database. Do not import sample data into the production database. The new location must not change old application prefixes. Check local and proxied health, the `/fabricworld/new` deep link, and assets; recheck Ledger and FeeTable.

## Local Development

Go uses the toolchain declared in `go.mod`. If the local goenv wrapper checks for a specified version that is not installed, use the installed Go's `GOTOOLCHAIN=auto` to download or select the project toolchain without changing the system default. In the recorded environment, commands used `GOENV_VERSION=1.24.0 go ...`, which actually ran Go 1.26.8.

Install libvips (Homebrew `vips` is recommended on macOS), then run:

```sh
npm --prefix web run build
go build -o bin/fabricworld ./cmd/fabricworld
bin/fabricworld init --data .local/dev-data
bin/fabricworld serve --data .local/dev-data --with-prefix
```

Before upgrading an existing local data directory, back it up and then run `bin/fabricworld migrate --data .local/dev-data`. If the material-catalog or finished-work extensions are missing, `check`/`serve` refuse to start; do not overwrite existing data with `init`.

The local URL is `http://127.0.0.1:18082/fabricworld/`. For routine frontend hot reload, run the web project's `npm run dev` and proxy the API to a local backend started without `--with-prefix`. All test data remains under `.local`, which Git ignores.

Test browsers may be installed in a project-ignored directory:

## Backups

NAS routine backups are manual complete compressed archives to an explicitly identified external volume through Portal or the agent. Pause business/background services, cold-copy all data/config/program state and media/work files to external staging, resume previously running services, then compress and verify that immutable copy. NAS has no routine internal staging/archive, native daily schedule, or automatic backup deletion.

Native packages remain a compatibility format: consistent SQLite plus image hard links and a SHA-256 manifest. The disabled legacy daily job combines cleanup and backup; it is not the NAS full-backup path. Existing hard-linked snapshots must not be edited because they share immutable production image inodes. Only completed native snapshots have `manifest.json`. Run native compatibility checks on an isolated data copy:

```sh
systemctl status fabricworld-backup.timer
journalctl -u fabricworld-backup.service -n 50 --no-pager
runuser -u fabricworld -- /opt/fabricworld/bin/fabricworld backup --data /path/to/isolated-data --out /path/to/new-native-snapshot
```

Native output directories must not exist; a lock conflict is failure, not a completed backup. The verified 2026-09-27 maintenance-computer archive and independent ALI Cloud Backup remain preserved in shared current-state. NAS upgrades instead use the root-private complete cold protection in [DEPLOYMENT.md](DEPLOYMENT.md), removed only after runtime and previous-row/file preservation pass.

## Isolated Recovery and Production Recovery

First confirm that isolated port 19082 is free. Port 18083 is assigned to RecipeBox and must not be used for a recovery rehearsal.

```sh
fabricworld restore --source /path/to/backup --out /path/to/new-restore-directory
fabricworld migrate --data /path/to/new-restore-directory
fabricworld check --data /path/to/new-restore-directory
fabricworld serve --data /path/to/new-restore-directory --listen 127.0.0.1:19082 --with-prefix
```

The native `restore` validates its manifest, restricts relative paths, copies files independently, and rejects an existing target. Full NAS cold archives are instead extracted into an isolated directory and verified before the agent restores the complete application subtree. Confirm the recovery point and later data loss; pause writes, preserve current state, verify photos/covers/recycle-bin records/dimensions, set correct ownership, and restart for read-only acceptance. Keep NAS native scheduling disabled. Program rollback is not data restoration.

## Diagnostics and Verification

```sh
systemctl status fabricworld fabricworld-backup.timer
journalctl -u fabricworld -n 100 --no-pager
curl --fail http://127.0.0.1:18082/healthz
curl --fail http://127.0.0.1/fabricworld/healthz
systemctl show fabricworld -p MemoryCurrent -p MemoryPeak -p NRestarts
df -h /opt/fabricworld
du -sh /opt/fabricworld/data /opt/fabricworld/backups
vips --version
```

Common failures: 415 unsupported file type; 422 HEIC decoding failure or limit exceeded; 429 image-processing/backup-lock/rate-limit contention; 507 disk or media quota; 409 multi-device version conflict. When the database is missing, inspect directories, permissions, and backups; do not run `init`.

Image processing supports the verified libvips 8.15 and 8.18. Color conversion uses the shared `--export-profile=srgb` option. Version 8.15 does not support the newer `--output-profile` name. On conversion failure, the journal records the subprocess error and the page returns a readable message.

## Ledger Integration Operating Contract

Ledger calls `/api/integrations/ledger` over local HTTP, targeting `127.0.0.1:18082` by default. Ledger owns the `LEDGER_FABRICWORLD_URL` configuration. FabricWorld adds no configuration, shared database, or service dependency. Release the FabricWorld endpoint before the Ledger dialog. Nginx, runtime identities, and permissions remain unchanged.

Daily cleanup indefinitely retains `operations` whose fingerprint begins with `ledger:`—the source and first result—so cross-day retries do not create duplicate fabrics. These records are backed up and restored with the database and are not stored in the browser or Ledger database. Old cleanup code does not recognize this retention rule; before a long-term rollback to an old version, disable the integration and inspect source records. When restoring an old backup, reconcile synchronization records created after the backup; do not treat program rollback as database recovery.

## Material-Field Compatibility

Material percentages are stored as an optional JSON field in existing schema v1 and require no table migration. The new service remains compatible with old clients whose save requests omit the percentage field. The old service does not recognize the field; editing a record after rollback loses its percentages. During rollback, pause details editing and preserve the pre-release backup.

The independent material catalog uses the added `material_catalog(name,id)` table, allowing zero-use names to persist. Fabric counts are still calculated from current records. The catalog extension creates this compatible table. A complete `migrate` also adds finished-work structures while preserving `user_version=1` and all fabric/photo data. First installation creates the table directly. When releasing against an old database on NAS, follow [DEPLOYMENT.md](DEPLOYMENT.md): protect complete cold state, rehearse candidate `migrate` and `check` on a disposable copy, then apply them to the protected live data. Removal also updates recycle-bin records; restoring a fabric does not bring back a removed material. Operation results are retained for seven days. For an unknown result, first query or retry with the original key. If the set version is stale, recheck the current catalog. The preceding program version can read the extended v1 database while ignoring the independent catalog table. During rollback, zero-use names are not displayed, but their persistent data is not cleared. Rollback code does not undo completed bulk removal, and history retains before/after snapshots. After restoring a pre-upgrade backup, explicitly run `migrate` before starting the new version. Production acceptance checks only the catalog, pages, and health in read-only mode; bulk-write regression uses an isolated synthetic database.

## Finished-Work Module Upgrade

Finished works use the same database and `media` directory. They add no runtime, Nginx configuration, unit, port, permission, or backup task. The added `works`/`work_changes`, `media.work_id`, and compatibility protections require explicit migration. After production authorization, use the NAS release flow's complete cold protection, independent candidate rehearsal, protected live `migrate`/`check`, startup, and runtime/data acceptance. Cleanup also covers the finished-work recycle bin and photos. During rollback, the old program can still read and write fabrics while ignoring finished works. Photo-protection triggers prevent old cleanup from deleting finished-work photos, and backups still cover all media. See [WORKS.md](WORKS.md) for the complete procedure and limitations.

## Documentation Synchronization

After pushing managed documentation, run `SYNC_HOST=nas ~/agent-config/skills/server-operations/scripts/sync-docs.sh FabricWorld`; it verifies drift/files and records `docs/SOURCE`. Shared gateway context synchronizes to its own hosts.

## ServerPortal Integration Materials

`deploy/portal.json` is this application's maintenance source for its resource declaration. Local-machine release uses the `server-operations` validator and saves the same declaration and binary in the same local version directory. It runs `portal-check` before release and `portal` afterward, installs the declaration at `/opt/fabricworld/config/portal.json` through existing restricted SSH, and verifies the SHA-256 actually loaded by the collector. `config/portal-source.json` records the declaration's source commit; it and the program's `current-commit` represent versions of different artifacts.

The portal discovers this application through the controlled link `/opt/serverportal/registry.d/fabricworld.json`. It automatically loads a successfully updated declaration without a restart. The first normal local-machine release also creates the link, so the portal's central application list does not need another edit. Ordinary releases may update this application's declaration; administrators still install other units, environment settings, Nginx configuration, and release scripts.

Source-code or database-behavior changes cannot use this option instead of a program release.

When a data root, media behavior, backup format, unit, port, or access path changes, maintain the declaration and corresponding documentation in the same commit, update the shared inventory, and verify resource coverage. The portal automatically reads files, media, database tables, and systemd state. The maintenance agent must explicitly update directory purposes, API descriptions, and permission boundaries. See [portal maintenance](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/portal.md) for the shared protocol, failure handling, and new-application integration.

Portal protects both HTTPS entrances; loopback release checks remain private and public unauthorized APIs return 401. Device grants persist until revocation; preserve shared cookie-renewal headers. NAS Portal backups cold-copy complete state to external staging rather than relying on the native media snapshot. Recovery remains agent-operated after confirming the recovery point.

## Phone Home-Screen Icon

The existing `/fabricworld/apple-touch-icon.png` is 180×180. Nginx allows anonymous GET/HEAD only for it, `favicon.ico`, and `fabricworld.svg`, preserving version query parameters. Pages, APIs, and user media continue to use device authentication. See the [shared troubleshooting record](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/common-issues.md#统一认证后-iphone-桌面图标缺失) for the common reason, deployment state, and phone acceptance checks.

## Current Release Entry Point

This project is for personal use: validate each change locally before release; there is no full-regression gate, and permanent tests are neither added nor retained by default. For UI changes, check the computer and phone scenarios that are actually used. Validate data migrations, bulk writes/deletes, and backup recovery with an isolated copy first.

See [Local-Machine Release and Rollback](DEPLOYMENT.md) for the complete flow. GitHub stores source code only. Locally, `make release` builds and `make deploy` updates production; documentation synchronizes separately.

## Release-Material Cleanup

NAS has no daily backup/release pruning. Successful upgrade acceptance removes only its private `/var/lib/server-upgrades/fabricworld/<release>/` and candidate work copy; failed or uncertain copies remain for the agent. Existing ALI and maintenance-computer backups are untouched. [Legacy retention](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/retention.md) is not enabled for NAS upgrades.

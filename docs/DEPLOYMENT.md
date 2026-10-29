# Local-Machine Release and Rollback

This project is for personal use: validate each change locally before release; there is no full-regression gate, and permanent tests are neither added nor retained by default. For UI changes, check the computer and phone scenarios that are actually used. Validate data migrations, bulk writes/deletes, and backup recovery with an isolated copy first.

GitHub is only a source-code and configuration backup; Actions are disabled. Committing and pushing do not update production. See the [server-operations local-machine release guide](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/release.md) for shared implementation and environment preparation. Project-specific parameters come from `deploy/release.json`.

## Routine Use

`make deploy` targets `nas` and its dedicated deployment key. `vps` provides public HTTPS on port 8443; temporary `ali` access forwards to the same NAS application. Public-check overrides and host-specific keys are maintained in the shared release guide. Explicit legacy ALI deployment must not create a second writer after migration.

```sh
# Exercise this change locally, then commit and push the source.
git add <files-changed-in-this-task>
git commit -m "Describe the change"
git push origin main
make release   # Build from the committed snapshot; do not run the full test suite.
make deploy    # Reuse the artifact from the same commit and release it over SSH.
```

Local artifacts live under `.local/releases/<full-commit>/` and include the program, `manifest.json`, and `portal.json` from the same commit. Deployment verifies hashes and confirms that the commit is backed up on `origin/main`; it reuses artifacts when source has not changed. Untracked files are not included in the build. `make releases` lists local versions. These artifacts are not committed to Git.

Use `make portal` to update only portal information. It preflights and synchronizes the declaration from the same commit without building or restarting the business program. A declaration-synchronization failure does not automatically revert a program update that already succeeded; fix the issue and retry the declaration separately.

## Release Boundaries

Use the restricted `fabricworld-deploy` identity; its private key exists only on the maintenance computer. To create the account for the first time, use `deploy/setup-deploy.sh <local-release-public-key>`; do not reinitialize an existing account. The forced command accepts only `deploy`, `portal-check`, and `portal`, and `sudo` permits only this application's fixed release script.

Switching begins only after upload and verification succeed. On `nas`, the shared helper cold-copies complete data/config/program state, including media and retained work, to `/var/lib/server-upgrades/fabricworld/<release>/`. The root-private recovery copy is separate from the candidate's independent work copy. Runtime and complete previous SQLite rows/files must pass verification before only this upgrade's copy is deleted; failure or uncertainty retains it. `previous` remains the program rollback artifact. Normal releases do not transfer production data. Administrators install units, Nginx, environment, authorization, and scripts from source; documentation uses `SYNC_HOST=nas` with the shared `sync-docs.sh`.

After cold protection, the NAS script rehearses `migrate` and `check` against the independent copy, then applies the explicit migration to stopped live data. Verify authorized schema transformations and compatibility first; strict row/file preservation rejects unexplained changes and retains the recovery copy. For Ledger-integration updates, release FabricWorld before Ledger.

`fabricworld-cleanup.service`/`.timer` are separately installed runtime configuration and must be registered in `deploy/portal.json`. Only a host marked `nas` enables cleanup scheduling; the native backup timer remains disabled. A program-only deployment does not install or change these units. During existing-data migration, install the reviewed units without running `init`, and verify their lock/sandbox contract before authorized activation; see [OPERATIONS.md](OPERATIONS.md).

## Rollback and Troubleshooting

```sh
make releases
make rollback COMMIT=<explicitly-selected-full-old-commit>
```

Rollback reuses a verified artifact retained on the local machine and still performs a backup and health check. First confirm that the old program is compatible with current data. Rolling back the program does not restore the database, photos, or device state. To restore data, first choose a restore point and preserve the current state, then follow the restore procedure in [OPERATIONS.md](OPERATIONS.md).

On NAS, `/opt/fabricworld/releases/` retains the program, `previous`, metadata, result/recovery, and upgrade-acceptance evidence; `current-commit` records the actual running source. `.local/releases/<commit>/last-deployment.json` records the local result. After a failed switch, inspect the result, journal, and retained private snapshot. NAS does not automatically prune uncertain upgrade copies. [Legacy automatic retention](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/retention.md) applies to `ali`, not the NAS recovery namespace.

# Local-Machine Release and Rollback

This project is for personal use: validate each change locally before release; there is no full-regression gate, and permanent tests are neither added nor retained by default. For UI changes, check the computer and phone scenarios that are actually used. Validate data migrations, bulk writes/deletes, and backup recovery with an isolated copy first.

GitHub is only a source-code and configuration backup; Actions are disabled. Committing and pushing do not update production. See the [server-operations local-machine release guide](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/release.md) for shared implementation and environment preparation. Project-specific parameters come from `deploy/release.json`.

## Routine Use

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

Switching begins only after upload and verification succeed. Preserve the pre-upgrade backup and `previous`, and restore the old program if the health check fails. A routine release does not carry production data. An administrator explicitly synchronizes units, Nginx, environment configuration, authorization, and release scripts from source; documentation uses the shared `sync-docs.sh`.

After backup, the release script runs the candidate's `migrate` and `check` commands. For schema-related updates, verify compatibility first. For Ledger-integration updates, release FabricWorld before Ledger.

## Rollback and Troubleshooting

```sh
make releases
make rollback COMMIT=<explicitly-selected-full-old-commit>
```

Rollback reuses a verified artifact retained on the local machine and still performs a backup and health check. First confirm that the old program is compatible with current data. Rolling back the program does not restore the database, photos, or device state. To restore data, first choose a restore point and preserve the current state, then follow the restore procedure in [OPERATIONS.md](OPERATIONS.md).

On the server, `/opt/fabricworld/releases/` retains the program, `previous`, `metadata`, `result`, and `recovery`; `current-commit` records the actual running source. `.local/releases/<commit>/last-deployment.json` records the local release result. Upload failures usually occur before the service has stopped; for failures after a switch, inspect `result` and the journal first. Server release history and routine pre-release snapshots rotate according to [automatic release-artifact retention](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/retention.md); local builds, manual backups, migrations, and portal export packages are excluded.

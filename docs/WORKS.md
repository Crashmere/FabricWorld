# Finished Sewing Works Module

## Goals and Scope

Record completed sewing projects so their photos, patterns, adjustment notes, and source fabrics can be found together. Preserve the shared-library model and centralized device authentication for public access. Finished works and fabrics share the existing service, database, image processing, and backup system; no runtime dependency, port, or shared configuration is added.

| Information | Rule |
| --- | --- |
| Work name | Required, 1–200 characters |
| Photos | May be added later; up to 10 photos, with the first used as the cover; support reordering, removal, and zoomed viewing; no more than 25 MiB each |
| Category | Optional, up to 40 characters; offer common choices and allow custom values |
| Completion date | Optional; valid date from 1900 through 2200; leave empty when unknown |
| Pattern / tutorial | Optional, 300 characters, for a name, number, or source; displayed as plain text |
| Size and recipient | Up to 80 characters each |
| Tags | Up to 20, each no more than 40 characters; deduplicated |
| Making notes | 8,000 characters; preserve line breaks; may record techniques, adjustments, and retrospectives |
| Linked fabrics | Up to 20; fabrics may be in stock or used up; each link may have a 300-character usage note |

Links record provenance only. One finished work may use multiple fabrics, and one fabric may be used by multiple works. Cutting shape, grain direction, and remnant combinations cannot be derived reliably from “how many meters were used,” so saving a work does not change a fabric's `revision`, status, dimensions, or purchase amount. The details page links to the fabric; the user records its actual remaining dimensions through the existing remnant flow. Fabric-usage notes are textual records and do not participate in numeric inventory calculations. The current scope excludes works in progress, automatic material deductions, cost allocation, tutorial collection, and progress management.

## Pages and Operations

- Every page's top bar switches between the exact UI sections `布料库` and `成品集`, on the same row as the application icon. The menu does not duplicate the `成品集` entry.
- `/works`: photo cards with server-side search across name, category, pattern, size, recipient, tags, notes, and linked fabrics; filter by category or fabric. It shows 24 works per page and supports loading more. The default is most recently completed, using the entry date when the completion date is empty; users can instead sort by most recently modified or by name. Filters are stored in the URL.
- `/works/new` and `/works/:id/edit`: name is the only required field, photos upload independently, and the form groups work details, fabrics used, and making notes. `?fabric=:id` preselects a source. The fabric details action `记录成品` uses this entry point.
- `/works/:id`: complete photos, pattern and size, fabrics used, and notes, with edit, change-history, and delete entry points. Related finished works can be viewed in reverse from fabric details.
- `/works/trash`: 30-day soft deletion and restoration, switched alongside the fabric recycle bin through category entries. Deleting a work does not delete source fabrics; restoring it preserves the original photo order and links.
- CSV and ZIP exports follow the current search and filters and exclude the recycle bin. ZIP includes `works.json` and the finished works' main images. It is not a backup-and-restore format.

On desktop, the editor places photos and fields in columns. On phones, it lays them out sequentially and fixes the Save button at the bottom. Dates retain the system picker and `date-input` container. Category entry and filtering use `ChoiceField`. Long names, tags, and notes wrap at 320 px. Photos continue to use `PhotoViewer`. Real-phone camera and system photo-library behavior still require device verification.

## Data and Consistency

Add `works` and `work_changes`, represented as bounded JSON plus `revision`, timestamps, and soft-delete columns. Add a nullable `work_id` foreign key to `media`, mutually exclusive with `fabric_id`. Finished works and fabrics upload their own photos; an image already bound to one record cannot be reused by another. Upload behavior, image quotas, locking, thumbnail conversion, and the retention period after removal are shared with fabrics.

Fabric links are stored in the work's JSON. The server generates the name snapshot and does not accept a client-forged name. When the linked fabric is accessible, reads display its current name; after deletion or permanent cleanup, reads still display the historical name and mark the source unavailable. Existing invalid links may be kept or removed. New links accept only non-deleted fabrics; used-up fabrics are allowed. Deleting a fabric does not cascade to finished works or their history.

Every create, edit, delete, and restore transaction writes the record, photo ownership, history snapshot, and `operations` result atomically. All writes use the existing 32-character idempotency key; edit/delete/restore operations check `revision`. History queries return at most the latest 100 changes. All user data is rendered as plain text, and the existing origin checks and rate limits apply.

Finished-work drafts and fabric drafts are stored independently in `sessionStorage`. Ordinary drafts remain for 24 hours; an unuploaded `File` must be selected again after refresh. Before submission, store the request body, idempotency key, and send time. After refresh, query `operations` before comparing the edit `revision`. If the result is unknown, only query it or retry the identical body with the original key. Do not resend an unknown submission after seven days; require manual review first. On conflict, preserve the input and show the latest record. Only after the user reviews it may the current `revision` be used for another submission. Unknown delete/restore submissions also retain the original key and support lookup after refresh.

## Migration, Release, and Rollback

On 2026-09-27, with user authorization, the finished-work module completed its production backup, migration, and release. The running commit was `ae936800`. Future production migrations involving database structure still follow the `server-operations` authorization table.

1. Complete Go/race/vet checks, frontend type checking and build, and Chromium/WebKit business-flow tests. Tests use only an isolated synthetic database.
2. On NAS, the current root-managed release script takes the shared maintenance and application deployment locks, stops this application, and protects complete cold data/config/program state under `/var/lib/server-upgrades/fabricworld/<release>/`. It runs the candidate's `migrate` and `check` on a disposable copy before applying them to the protected live data, atomically replaces the program, and validates runtime and affected data. Successful acceptance immediately removes only that upgrade's protection; failed or uncertain copies remain. The finished-work schema itself adds no unit or Nginx requirement; see [DEPLOYMENT.md](DEPLOYMENT.md).
3. Migration is explicit, transactional, and repeatable: create the two tables; add `media.work_id` according to column existence; and create the indexes, mutual-exclusion constraint, and photo-protection triggers. Preserve `application_id` and `user_version=1`; do not change fabric bodies or photo files. `check` and `serve` refuse to start when tables or columns are missing; they do not migrate implicitly.
4. In production, perform read-only checks of `/works`, create/recycle-bin deep links, the API, and the original fabric library. After pushing managed documentation, run `SYNC_HOST=nas ~/agent-config/skills/server-operations/scripts/sync-docs.sh FabricWorld`.

If the candidate fails, the original release script restores the old program but does not automatically roll back the database. The old program can read the compatibly extended fabric data while ignoring finished-work pages and records. Old backups enumerate all `media` and therefore still include finished-work photos. `protect_work_media` prevents old cleanup logic from mistaking a bound finished-work photo with a null `fabric_id` for an expired upload. The photo-owner mutual-exclusion constraint prevents the old program from assigning a finished-work photo to a fabric. Rollback does not undo committed finished-work data; finished-work management and expired-work cleanup resume only after the new version is restored. A long-term rollback to a version predating other existing features must also follow the material and Ledger compatibility limits in [OPERATIONS.md](OPERATIONS.md).

New-version backups automatically cover finished works, history, and photos; they do not add a separate backup task. After restoring a pre-upgrade backup, run `migrate` and then `check`. Restoring production to a selected point in time still requires explicit authorization.

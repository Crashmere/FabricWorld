# HTTP API

The external prefix is `/fabricworld/`; Nginx strips the prefix. The paths below are internal Go paths, and `{id}` and `{key}` are path parameters. ServerPortal's centralized device authentication protects the public API and media; unauthorized requests return 401. Local loopback calls remain available. JSON uses camelCase. Except for the Ledger integration's `transactionId`, writes require an `Idempotency-Key` containing 32 random lowercase hexadecimal characters. Fabric and finished-work updates, deletions, and restorations submit the current `revision`; material removal uses the catalog `version`.

| Method and path | Description |
| --- | --- |
| GET /healthz | Returns `status=ok` when the database is reachable |
| GET /api/fabrics | `items`/`total`/`offset`/`stats`, 24 items per page; query parameters: `q`, `status` (`stock`/`unused`/`using`/`used`/`all`), `material`, `color`, `location`, `tag`, `min_width`/`min_length` (cm), `sort` (`purchase`/`updated`/`name`), `offset`, and `trash=1` |
| POST /api/fabrics | Creates a fabric from the complete form and returns the `Fabric` |
| POST /api/integrations/ledger | Idempotently creates a fabric by Ledger `transactionId` and returns the current `Fabric` |
| GET /api/fabrics/{id} | Returns details including `pieces`, `photos`, `photoIds`, `revision`, and `deletedAt` |
| PUT /api/fabrics/{id} | Saves the complete form by default; `action=remnant` updates only `pieces`/`status` and requires `revision` |
| DELETE /api/fabrics/{id} | Body contains `revision`; soft-deletes and returns the new version |
| POST /api/fabrics/{id}/restore | Body contains `revision`; restores a record deleted within the last 30 days |
| GET /api/fabrics/{id}/changes | Returns the latest 100 changes, including before/after business snapshots |
| POST /api/uploads | `multipart/form-data`, with `file` as the first part; maximum 25 MiB per image; success returns 200 and `Media` |
| GET /api/operations/{key} | Returns the original write result; 404 means it has not been found and must not be treated directly as failure |
| GET /api/suggestions | `materials` merges the independent catalog with materials from non-deleted fabrics; `tags`/`location`/`color` return at most 200 entries each |
| GET /api/materials | Returns the saved material array; each item has `name`/`count`/`trashCount`/`version`, including recycle-bin usage |
| POST /api/materials | Body contains `name` (1–40 characters after trimming); persistently adds a zero-use material; returns `name` and requires `Idempotency-Key` |
| POST /api/materials/remove | Body contains `name`/`version`; atomically removes that material from all related fabrics; returns `name`/`affected` |
| GET /api/export | Query parameter `format=csv`/`zip`, with the same filters as the list; `status` defaults to `stock`; ignores pagination and `trash`; always excludes the recycle bin |
| GET /media/{id}/main | Standard JPEG; available only for valid staged images or images belonging to non-deleted records |
| GET /media/{id}/thumb | WebP thumbnail |

When the fabric list's `status` is omitted or is `all`, it includes all non-deleted states. `sort` defaults to `purchase`, and `offset` defaults to 0. `trash=1` switches the query to the recycle bin. `total` is the number of records under the current filters. `stats.total`/`stock`/`incomplete` always summarize all non-deleted fabrics and do not vary with the filters.

## Finished-Work Endpoints

| Method and path | Description |
| --- | --- |
| GET /api/works | `items`/`total`/`offset`/`categories`; `q`, `category`, `fabric` (source ID), `sort` (`completed`/`updated`/`name`), `offset`, `trash=1`; 24 works per page |
| POST /api/works | Creates a complete `Work`; name is required; returns the `Work` |
| GET /api/works/{id} | Returns work details, including photos and source-fabric availability |
| PUT /api/works/{id} | Complete edit, submitting the current `revision` |
| DELETE /api/works/{id} | Body contains `revision`; soft-deletes and returns the new version |
| POST /api/works/{id}/restore | Body contains `revision`; restores a finished work deleted within the last 30 days |
| GET /api/works/{id}/changes | Returns the latest 100 changes, including before/after finished-work snapshots |
| GET /api/works/export | `format=csv`/`zip`; uses the same search/category/fabric filters, ignores pagination, and excludes the recycle bin |

The two tables above cover the current business, health, and media endpoints. Keep the portal's `deploy/portal.json.apis` consistent with both tables. Register every method and path separately; query parameters and the body's `action` are not part of the path. The finished-work list's `sort` defaults to `completed`, and `offset` defaults to 0. `categories` contains categories from every non-deleted finished work and is not restricted by the current filters. See the [shared troubleshooting guide](https://github.com/Crashmere/agent-config/blob/main/skills/server-operations/references/common-issues.md#门户-api-列表与服务不一致) for the verification and synchronization workflow.

`Work` contains `name`, `category`, `completedDate`, `pattern`, `size`, `recipient`, `tags`, `notes`, `fabrics`, `photoIds`, and the server fields `id`/`revision`/`photos`/`createdAt`/`updatedAt`/`deletedAt`. Name is required, the date may be empty, and `fabrics` is an array of `{fabricId,name,note,available}`. Input uses only `fabricId` and `note`; the server generates the `name` snapshot from the source, and `available` reflects live read availability. Used-up fabrics may be linked. A new link cannot use a deleted or nonexistent fabric, while an existing invalid link may remain. A source change does not automatically update the finished work's `revision`, and saving a finished work does not change inventory.

Writes use `Idempotency-Key` and the existing operations lookup. `revision_conflict`, `key_reused`, `deleted`, `expired`, and `validation` retain the existing error format. Finished-work photos use the existing uploads endpoint. Media can be bound to only one finished work or fabric; while the owning record is deleted, the media endpoint cannot read it, and it becomes available again after restoration. See [WORKS.md](WORKS.md) for field limits, pages, and migration details.

The same idempotency key for a form write must retain the exact HTTP method, internal path, and body bytes. Upload replay is identified by the original file bytes and does not require the same multipart boundary. Maintenance cleanup retains ordinary operation results for approximately seven days; Ledger source records are retained indefinitely. `GET /api/operations/{key}` returns the original submission result, which may be a `Fabric`, `Work`, `Media`, or material-operation result; it does not represent the resource's latest state. Uploads accept JPEG, PNG, WebP, HEIC, and HEIF. A staged photo must be bound within 24 hours. Media accepts only `main` and `thumb`; it returns 404 when the photo has been removed, its owning record is deleted, or staging has expired.

## Fabric Fields and Integrations

`Fabric` contains `name`, `materials`, `materialPercentages`, `composition`, `color`, `tags`, `status`, `location`, `purchaseDate`, `shop`, `price`, `notes`, `pieces`, and `photoIds`. `price` is a decimal string; empty means unknown. `Piece` contains decimal-string `width`/`length`, `unit` (`cm`/`m`), `count`, `irregular`, and `note`; the server recalculates `widthMM`/`lengthMM`. The server overwrites read-only fields such as IDs, timestamps, and integer-cent amounts; they cannot be used to bypass validation.

`materialPercentages` is an optional map from material name to decimal string, for example `{"cotton":"75.1250","linen":"50.5"}`. Store percentages only for selected materials. A single selection is fixed at 100; multiple selections may be empty, and neither individual values nor their sum is required to equal 100. Input is not rounded. Each value is at most 20 characters and accepts only a nonnegative decimal number or an empty string. `composition` remains independent explanatory text. Old records may lack the field. When an old client omits the field during modification, percentages for materials that remain selected are preserved; an empty object or empty string explicitly clears multi-material percentages. The CSV material column includes percentages, and ZIP JSON preserves the independent map.

A material catalog entry's `count` is the total number of fabrics using that name, including used-up and recycle-bin records; `trashCount` is the recycle-bin subset. `version` is a SHA-256 digest—64 hexadecimal characters—of the independent catalog identity, if any, and the ID/`revision` pairs of related fabrics sorted by ID. The client submits the catalog value unchanged. Removal requires `Idempotency-Key` and matches the name exactly. If the related set changes, the server returns 409 / `material_changed`; refresh the catalog before confirming again. Removal deletes the material and its percentage. If one material remains, it becomes 100; all other details, photos, and independent composition text remain unchanged. Related fabric revisions increase, and the history action is `remove_material`. There is no partial success, and the idempotent result is available through the operations endpoint. The catalog merges current fabrics with `material_catalog`; it does not include names found only in historical snapshots. An independently added name remains when usage reaches zero; removal also deletes its catalog entry. Adding the same name repeatedly does not create duplicates. The operation result and catalog write commit in the same transaction; a replay returns the old result and does not recreate a name removed later. Re-adding creates a new catalog identity, so a stale removal version cannot delete the new entry.

Ledger integration input is `{transactionId,name,purchaseDate,price}`. `transactionId` is a canonical lowercase UUID; date and total price are required. This endpoint uses the source ID instead of `Idempotency-Key`. The same ID forever identifies the first fabric created: later calls only return the existing record and do not update fields. Source records are retained indefinitely. A deleted/cleaned fabric returns 409/404 and is not recreated. The existing origin checks and write rate limits apply; no new identity authentication is introduced.

Names are at most 500 characters to remain compatible with Ledger titles. An empty name from the integration uses the exact fallback `未命名布料`. The date maps verbatim, and the amount is a decimal value in yuan. Status is unused, with one default piece whose width and length are unknown and whose count is 1; all other details and photos are empty. The client redirects to `/fabricworld/fabrics/:id/edit` for completion.

A remnant request needs only `{revision, action: "remnant", status: "using" | "used", pieces}` and returns the complete `Fabric`. `used` requires empty `pieces`; other valid states remain compatible with old clients and are normalized to `using`. The action applies only to an existing fabric and cannot create a record. Even if old form fields such as `name` or `photoIds` are included, it does not change details or photo ownership. A stale version still returns 409.

Application errors are JSON objects containing `code`, `message`, and an optional `field`. HTTP statuses are: 400 malformed request, 403 origin rejected, 404 not found, 409 conflict, 410 restore window expired, 413 too large, 415 unsupported format, 422 invalid field, 429 concurrency/rate/maintenance busy, 500 internal error, and 507 insufficient disk or quota. The shared public entry point returns 401 for authentication failure. After a network interruption or unknown 5xx result on mobile, query the original idempotency key first; do not generate a new key and blindly retry creation.

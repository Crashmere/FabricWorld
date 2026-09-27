package app

import "context"

// This additive extension preserves the v1 fabric format and remains readable
// by the previous program, so a program rollback does not require data restore.
const materialCatalogSchema = `CREATE TABLE IF NOT EXISTS material_catalog(
name TEXT PRIMARY KEY, id TEXT NOT NULL UNIQUE);`

const worksSchema = `CREATE TABLE IF NOT EXISTS works(
id TEXT PRIMARY KEY, revision INTEGER NOT NULL, body TEXT NOT NULL CHECK(json_valid(body)),
created_at TEXT NOT NULL, updated_at TEXT NOT NULL, deleted_at TEXT);
CREATE INDEX IF NOT EXISTS work_updated ON works(deleted_at,updated_at DESC,id DESC);
CREATE TABLE IF NOT EXISTS work_changes(work_id TEXT NOT NULL REFERENCES works(id) ON DELETE CASCADE,
revision INTEGER NOT NULL,body TEXT NOT NULL,PRIMARY KEY(work_id,revision));`

const workMediaColumn = `ALTER TABLE media ADD COLUMN work_id TEXT REFERENCES works(id) ON DELETE CASCADE
CHECK(work_id IS NULL OR fabric_id IS NULL);`

// Older v1 cleanup treats fabric_id=NULL as an unbound upload. Keep work photos
// safe during a program rollback; backups already enumerate every media row.
const workMediaGuards = `CREATE INDEX IF NOT EXISTS media_work ON media(work_id);
CREATE TRIGGER IF NOT EXISTS protect_work_media BEFORE DELETE ON media
WHEN OLD.work_id IS NOT NULL
AND EXISTS(SELECT 1 FROM works WHERE id=OLD.work_id)
AND (OLD.removed_at IS NULL OR julianday(OLD.removed_at)>julianday('now','-30 days'))
BEGIN SELECT RAISE(IGNORE); END;`

// Migrate is explicit: check/serve never create missing schema or an empty DB.
func Migrate(ctx context.Context, dir string) error {
	s, e := Open(dir, false)
	if e != nil {
		return e
	}
	defer s.DB.Close()
	if e = s.checkIntegrity(ctx); e != nil {
		return e
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, materialCatalogSchema); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, worksSchema); e != nil {
		return e
	}
	var columns int
	if e = tx.QueryRowContext(ctx, "SELECT count(*) FROM pragma_table_info('media') WHERE name='work_id'").Scan(&columns); e != nil {
		return e
	}
	if columns == 0 {
		if _, e = tx.ExecContext(ctx, workMediaColumn); e != nil {
			return e
		}
	}
	if _, e = tx.ExecContext(ctx, workMediaGuards); e != nil {
		return e
	}
	return tx.Commit()
}

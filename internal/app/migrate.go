package app

import "context"

// This additive extension preserves the v1 fabric format and remains readable
// by the previous program, so a program rollback does not require data restore.
const materialCatalogSchema = `CREATE TABLE IF NOT EXISTS material_catalog(
name TEXT PRIMARY KEY, id TEXT NOT NULL UNIQUE);`

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
	return tx.Commit()
}

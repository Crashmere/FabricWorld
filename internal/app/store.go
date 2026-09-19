package app

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	_ "modernc.org/sqlite"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const applicationID = 1178681932
const schema = `PRAGMA application_id=1178681932; PRAGMA user_version=1;
CREATE TABLE fabrics(id TEXT PRIMARY KEY, revision INTEGER NOT NULL, body TEXT NOT NULL CHECK(json_valid(body)),created_at TEXT NOT NULL,updated_at TEXT NOT NULL,deleted_at TEXT);
CREATE INDEX fabric_updated ON fabrics(deleted_at,updated_at DESC,id DESC);
CREATE INDEX fabric_purchase ON fabrics(deleted_at,coalesce(nullif(json_extract(body,'$.purchaseDate'),''),substr(created_at,1,10)) DESC,id DESC);
CREATE TABLE media(id TEXT PRIMARY KEY,fabric_id TEXT REFERENCES fabrics(id) ON DELETE CASCADE, body TEXT NOT NULL CHECK(json_valid(body)),created_at TEXT NOT NULL,removed_at TEXT);
CREATE INDEX media_fabric ON media(fabric_id);
CREATE TABLE changes(fabric_id TEXT NOT NULL REFERENCES fabrics(id) ON DELETE CASCADE,revision INTEGER NOT NULL,body TEXT NOT NULL,PRIMARY KEY(fabric_id,revision));
CREATE TABLE operations(key TEXT PRIMARY KEY,fingerprint TEXT NOT NULL,result TEXT NOT NULL,created_at TEXT NOT NULL);
` + materialCatalogSchema

type Store struct {
	DB          *sql.DB
	Dir         string
	UploadSlot  chan struct{}
	ExportSlot  chan struct{}
	MediaLimit  int64
	FreeReserve uint64
}

func Open(dir string, init bool) (*Store, error) {
	dir, e := filepath.Abs(dir)
	if e != nil {
		return nil, e
	}
	dbPath := filepath.Join(dir, "fabricworld.db")
	if init {
		if e = os.MkdirAll(dir, 0700); e != nil {
			return nil, e
		}
		f, e := os.OpenFile(dbPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return nil, e
		}
		f.Close()
	} else {
		if _, e = os.Stat(dbPath); e != nil {
			return nil, fmt.Errorf("database must exist; use init only for a new installation: %w", e)
		}
	}
	db, e := sql.Open("sqlite", "file:"+filepath.ToSlash(dbPath)+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=synchronous(FULL)")
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	good := false
	defer func() {
		if !good {
			db.Close()
		}
	}()
	if _, e = db.Exec("PRAGMA journal_mode=WAL"); e != nil {
		return nil, e
	}
	if init {
		if _, e = db.Exec(schema); e != nil {
			return nil, e
		}
	}
	var appID, version int
	if e = db.QueryRow("PRAGMA application_id").Scan(&appID); e != nil {
		return nil, e
	}
	if e = db.QueryRow("PRAGMA user_version").Scan(&version); e != nil {
		return nil, e
	}
	if appID != applicationID || version != 1 {
		return nil, fmt.Errorf("incompatible database identity or schema")
	}
	for _, d := range []string{"media", "tmp"} {
		if e = os.MkdirAll(filepath.Join(dir, d), 0700); e != nil {
			return nil, e
		}
	}
	good = true
	return &Store{DB: db, Dir: dir, UploadSlot: make(chan struct{}, 1), ExportSlot: make(chan struct{}, 1), MediaLimit: 5 << 30, FreeReserve: 5 << 30}, nil
}

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func load(ctx context.Context, q queryer, id string) (Fabric, error) {
	var f Fabric
	var b string
	e := q.QueryRowContext(ctx, "SELECT body FROM fabrics WHERE id=?", id).Scan(&b)
	if errors.Is(e, sql.ErrNoRows) {
		return f, fail(404, "not_found", "布料不存在")
	}
	if e != nil {
		return f, e
	}
	e = json.Unmarshal([]byte(b), &f)
	return f, e
}
func attach(ctx context.Context, q queryer, f *Fabric) error {
	f.Photos = []Media{}
	for _, id := range f.PhotoIDs {
		var b string
		if e := q.QueryRowContext(ctx, "SELECT body FROM media WHERE id=? AND fabric_id=? AND removed_at IS NULL", id, f.ID).Scan(&b); e != nil {
			return e
		}
		var m Media
		if e := json.Unmarshal([]byte(b), &m); e != nil {
			return e
		}
		f.Photos = append(f.Photos, m)
	}
	return nil
}
func (s *Store) Get(ctx context.Context, id string) (Fabric, error) {
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if e != nil {
		return Fabric{}, e
	}
	defer tx.Rollback()
	f, e := load(ctx, tx, id)
	if e == nil {
		e = attach(ctx, tx, &f)
	}
	return f, e
}
func fingerprint(method, path string, body []byte) string {
	h := sha256.New()
	h.Write([]byte(method + " " + path + "\n"))
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}
func replay(ctx context.Context, tx *sql.Tx, key, fp string) ([]byte, error) {
	var old, b string
	e := tx.QueryRowContext(ctx, "SELECT fingerprint,result FROM operations WHERE key=?", key).Scan(&old, &b)
	if errors.Is(e, sql.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	if old != fp {
		return nil, fail(409, "key_reused", "该提交编号已经用于另一项修改")
	}
	return []byte(b), nil
}
func (s *Store) Write(ctx context.Context, id, key, fp, action string, in Fabric) ([]byte, error) {
	if !idPattern.MatchString(key) {
		return nil, fail(400, "idempotency_required", "缺少有效提交编号")
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	cached, e := replay(ctx, tx, key, fp)
	if e != nil || cached != nil {
		return cached, e
	}
	var before *Fabric
	var f Fabric
	if id != "" {
		old, e := load(ctx, tx, id)
		if e != nil {
			return nil, e
		}
		if old.Revision != in.Revision {
			return nil, fail(409, "revision_conflict", "这条布料已在另一处修改，请保留输入并刷新核对")
		}
		before = &old
		if old.DeletedAt != nil && action != "restore" {
			return nil, fail(409, "deleted", "这条布料已移入回收站")
		}
		f = old
	} else {
		f.ID = ID()
		f.CreatedAt = now()
	}
	switch action {
	case "delete":
		t := now()
		f.DeletedAt = &t
	case "restore":
		if f.DeletedAt == nil {
			return nil, fail(409, "not_deleted", "布料不在回收站")
		}
		t, _ := time.Parse(time.RFC3339Nano, *f.DeletedAt)
		if time.Since(t) > 30*24*time.Hour {
			return nil, fail(410, "expired", "回收期限已过")
		}
		f.DeletedAt = nil
	case "save":
		// Older clients omit this field. Keep ratios for the materials they retain.
		if in.MaterialPercentages == nil && before != nil {
			in.MaterialPercentages = before.MaterialPercentages
		}
		in.ID = f.ID
		in.CreatedAt = f.CreatedAt
		in.DeletedAt = nil
		in.Photos = nil
		f = in
		if e = f.Validate(); e != nil {
			return nil, e
		}
	case "remnant":
		if before == nil {
			return nil, fail(400, "existing_required", "请先建立布料记录，再更新余料")
		}
		if in.Status != "" && in.Status != "unused" && in.Status != "using" && in.Status != "used" {
			return nil, invalid("status", "请选择还有剩余或已经用完")
		}
		// Old clients may still send the full form. Only stock fields belong
		// to this action; preserve all descriptive fields and photo ownership.
		f.Pieces = in.Pieces
		f.Status = "using"
		if in.Status == "used" {
			f.Status = "used"
		}
		if e = f.Validate(); e != nil {
			return nil, e
		}
	default:
		return nil, fail(400, "action", "未知操作")
	}
	f.Revision = 1
	if before != nil {
		f.Revision = before.Revision + 1
	}
	f.UpdatedAt = now()
	f.Photos = nil
	if action == "save" {
		for _, mid := range f.PhotoIDs {
			var owner, removed sql.NullString
			var created string
			e = tx.QueryRowContext(ctx, "SELECT fabric_id,removed_at,created_at FROM media WHERE id=?", mid).Scan(&owner, &removed, &created)
			if e != nil {
				return nil, invalid("photos", "照片不存在或已过期，请重新上传")
			}
			if owner.Valid && owner.String != f.ID {
				return nil, invalid("photos", "照片已经属于另一条布料")
			}
			t, _ := time.Parse(time.RFC3339Nano, created)
			if removed.Valid || (!owner.Valid && time.Since(t) > 24*time.Hour) {
				return nil, invalid("photos", "照片已过期，请重新上传")
			}
		}
	}
	body, _ := json.Marshal(f)
	_, e = tx.ExecContext(ctx, "INSERT INTO fabrics(id,revision,body,created_at,updated_at,deleted_at) VALUES(?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET revision=excluded.revision,body=excluded.body,updated_at=excluded.updated_at,deleted_at=excluded.deleted_at", f.ID, f.Revision, string(body), f.CreatedAt, f.UpdatedAt, f.DeletedAt)
	if e != nil {
		return nil, e
	}
	if action == "save" {
		_, e = tx.ExecContext(ctx, "UPDATE media SET removed_at=? WHERE fabric_id=? AND removed_at IS NULL", f.UpdatedAt, f.ID)
		if e != nil {
			return nil, e
		}
		for _, mid := range f.PhotoIDs {
			if _, e = tx.ExecContext(ctx, "UPDATE media SET fabric_id=?,removed_at=NULL WHERE id=?", f.ID, mid); e != nil {
				return nil, e
			}
		}
	}
	c := Change{Revision: f.Revision, Action: action, At: f.UpdatedAt, Before: before, After: &f}
	cb, _ := json.Marshal(c)
	if _, e = tx.ExecContext(ctx, "INSERT INTO changes VALUES(?,?,?)", f.ID, f.Revision, string(cb)); e != nil {
		return nil, e
	}
	if e = attach(ctx, tx, &f); e != nil {
		return nil, e
	}
	result, _ := json.Marshal(f)
	if _, e = tx.ExecContext(ctx, "INSERT INTO operations VALUES(?,?,?,?)", key, fp, string(result), now()); e != nil {
		return nil, e
	}
	if e = tx.Commit(); e != nil {
		return nil, e
	}
	return result, nil
}

type List struct {
	Items  []Fabric       `json:"items"`
	Total  int            `json:"total"`
	Offset int            `json:"offset"`
	Stats  map[string]int `json:"stats"`
}

func filter(v url.Values) (string, []any, error) {
	clauses := []string{"deleted_at IS NULL"}
	args := []any{}
	if v.Get("trash") == "1" {
		clauses[0] = "deleted_at IS NOT NULL"
	}
	if status := v.Get("status"); status != "" && status != "all" {
		if status == "stock" {
			clauses = append(clauses, "json_extract(body,'$.status') != 'used'")
		} else if status == "unused" || status == "using" || status == "used" {
			clauses = append(clauses, "json_extract(body,'$.status')=?")
			args = append(args, status)
		} else {
			return "", nil, invalid("status", "无效状态")
		}
	}
	if q := strings.TrimSpace(v.Get("q")); q != "" {
		if len(q) > 400 {
			return "", nil, invalid("q", "搜索内容过长")
		}
		clauses = append(clauses, `instr(lower(json_extract(body,'$.name')||' '||json_extract(body,'$.materials')||' '||json_extract(body,'$.tags')||' '||json_extract(body,'$.composition')||' '||json_extract(body,'$.notes')||' '||json_extract(body,'$.location')||' '||json_extract(body,'$.shop')),lower(?))>0`)
		args = append(args, q)
	}
	for _, k := range []string{"location", "color"} {
		if x := v.Get(k); x != "" {
			clauses = append(clauses, "json_extract(body,'$."+k+"')=?")
			args = append(args, x)
		}
	}
	for k, p := range map[string]string{"material": "materials", "tag": "tags"} {
		if x := v.Get(k); x != "" {
			clauses = append(clauses, "EXISTS(SELECT 1 FROM json_each(fabrics.body,'$."+p+"') WHERE value=?)")
			args = append(args, x)
		}
	}
	var dim []string
	for _, k := range []string{"width", "length"} {
		if x := v.Get("min_" + k); x != "" {
			n, e := decimal(x, 10, 1000000)
			if e != nil {
				return "", nil, invalid("dimensions", "筛选尺寸无效")
			}
			dim = append(dim, "json_extract(p.value,'$."+k+"MM')>=?")
			args = append(args, *n)
		}
	}
	if len(dim) > 0 {
		clauses = append(clauses, "EXISTS(SELECT 1 FROM json_each(fabrics.body,'$.pieces') p WHERE json_extract(p.value,'$.irregular')=0 AND "+strings.Join(dim, " AND ")+")")
	}
	return strings.Join(clauses, " AND "), args, nil
}
func (s *Store) List(ctx context.Context, v url.Values, all bool) (List, error) {
	out := List{Items: []Fabric{}, Stats: map[string]int{}}
	where, args, e := filter(v)
	if e != nil {
		return out, e
	}
	order := "coalesce(nullif(json_extract(body,'$.purchaseDate'),''),substr(created_at,1,10)) DESC,id DESC"
	switch v.Get("sort") {
	case "updated":
		order = "updated_at DESC,id DESC"
	case "name":
		order = "json_extract(body,'$.name') COLLATE NOCASE,id"
	case "", "purchase":
	default:
		return out, invalid("sort", "无效排序")
	}
	offset, _ := strconv.Atoi(v.Get("offset"))
	if offset < 0 || offset > 1000000 {
		return out, invalid("offset", "无效页码")
	}
	out.Offset = offset
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	if e = tx.QueryRowContext(ctx, "SELECT count(*) FROM fabrics WHERE "+where, args...).Scan(&out.Total); e != nil {
		return out, e
	}
	limit := " LIMIT 24 OFFSET ?"
	qa := append([]any{}, args...)
	qa = append(qa, offset)
	if all {
		limit = ""
		qa = args
	}
	rows, e := tx.QueryContext(ctx, "SELECT body FROM fabrics WHERE "+where+" ORDER BY "+order+limit, qa...)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var b string
		var f Fabric
		if e = rows.Scan(&b); e != nil {
			rows.Close()
			return out, e
		}
		if e = json.Unmarshal([]byte(b), &f); e != nil {
			rows.Close()
			return out, e
		}
		out.Items = append(out.Items, f)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	for i := range out.Items {
		if e = attach(ctx, tx, &out.Items[i]); e != nil {
			return out, e
		}
	}
	for k, q := range map[string]string{"total": "deleted_at IS NULL", "stock": "deleted_at IS NULL AND json_extract(body,'$.status')!='used'", "incomplete": "deleted_at IS NULL AND (json_array_length(body,'$.materials')=0 OR EXISTS(SELECT 1 FROM json_each(fabrics.body,'$.pieces') WHERE json_extract(value,'$.widthMM') IS NULL OR json_extract(value,'$.lengthMM') IS NULL))"} {
		var n int
		if e = tx.QueryRowContext(ctx, "SELECT count(*) FROM fabrics WHERE "+q).Scan(&n); e != nil {
			return out, e
		}
		out.Stats[k] = n
	}
	return out, nil
}
func (s *Store) Suggestions(ctx context.Context) (map[string][]string, error) {
	out := map[string][]string{}
	for _, k := range []string{"materials", "tags", "location", "color"} {
		q := "SELECT DISTINCT json_extract(body,'$." + k + "') FROM fabrics WHERE deleted_at IS NULL ORDER BY 1 LIMIT 200"
		if k == "materials" || k == "tags" {
			q = "SELECT DISTINCT value FROM fabrics,json_each(fabrics.body,'$." + k + "') WHERE deleted_at IS NULL ORDER BY 1 LIMIT 200"
		}
		if k == "materials" {
			q = "SELECT name FROM material_catalog UNION SELECT value FROM fabrics,json_each(fabrics.body,'$.materials') WHERE deleted_at IS NULL ORDER BY 1"
		}
		rows, e := s.DB.QueryContext(ctx, q)
		if e != nil {
			return nil, e
		}
		out[k] = []string{}
		for rows.Next() {
			var x string
			if e = rows.Scan(&x); e != nil {
				rows.Close()
				return nil, e
			}
			if x != "" {
				out[k] = append(out[k], x)
			}
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return nil, e
		}
	}
	return out, nil
}

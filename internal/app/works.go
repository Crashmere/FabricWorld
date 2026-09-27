package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Name is a server-owned snapshot so a work keeps its provenance even after
// the source fabric is permanently removed. Available is a live read field.
type WorkFabric struct {
	FabricID  string `json:"fabricId"`
	Name      string `json:"name"`
	Note      string `json:"note"`
	Available bool   `json:"available"`
}

type Work struct {
	ID            string       `json:"id"`
	Revision      int          `json:"revision"`
	Name          string       `json:"name"`
	Category      string       `json:"category"`
	CompletedDate string       `json:"completedDate"`
	Pattern       string       `json:"pattern"`
	Size          string       `json:"size"`
	Recipient     string       `json:"recipient"`
	Tags          []string     `json:"tags"`
	Notes         string       `json:"notes"`
	Fabrics       []WorkFabric `json:"fabrics"`
	PhotoIDs      []string     `json:"photoIds"`
	Photos        []Media      `json:"photos"`
	CreatedAt     string       `json:"createdAt"`
	UpdatedAt     string       `json:"updatedAt"`
	DeletedAt     *string      `json:"deletedAt"`
}

type WorkChange struct {
	Revision int    `json:"revision"`
	Action   string `json:"action"`
	At       string `json:"at"`
	Before   *Work  `json:"before"`
	After    *Work  `json:"after"`
}

func (w *Work) Validate() error {
	for _, v := range []struct {
		value *string
		field string
		max   int
	}{{&w.Name, "name", 200}, {&w.Category, "category", 40}, {&w.Pattern, "pattern", 300},
		{&w.Size, "size", 80}, {&w.Recipient, "recipient", 80}, {&w.Notes, "notes", 8000}} {
		if e := textField(v.value, v.field, v.max); e != nil {
			return e
		}
	}
	if w.Name == "" {
		return invalid("name", "请为这件成品填写名称")
	}
	if w.CompletedDate != "" {
		t, e := time.Parse("2006-01-02", w.CompletedDate)
		if e != nil || t.Year() < 1900 || t.Year() > 2200 {
			return invalid("completedDate", "请输入有效完成日期，也可以留空")
		}
	}
	var e error
	w.Tags, e = words(w.Tags, "tags")
	if e != nil {
		return e
	}
	if len(w.Fabrics) > 20 {
		return invalid("fabrics", "每件成品最多关联 20 块布料")
	}
	seen := map[string]bool{}
	for i := range w.Fabrics {
		f := &w.Fabrics[i]
		if !idPattern.MatchString(f.FabricID) || seen[f.FabricID] {
			return invalid("fabrics", "关联布料无效或重复")
		}
		seen[f.FabricID] = true
		if e = textField(&f.Note, "fabrics", 300); e != nil {
			return e
		}
	}
	if len(w.PhotoIDs) > 10 {
		return invalid("photos", "每件成品最多 10 张照片")
	}
	seen = map[string]bool{}
	for _, id := range w.PhotoIDs {
		if !idPattern.MatchString(id) || seen[id] {
			return invalid("photos", "照片无效或重复")
		}
		seen[id] = true
	}
	if w.Fabrics == nil {
		w.Fabrics = []WorkFabric{}
	}
	if w.PhotoIDs == nil {
		w.PhotoIDs = []string{}
	}
	return nil
}

func loadWork(ctx context.Context, q queryer, id string) (Work, error) {
	var w Work
	var body string
	e := q.QueryRowContext(ctx, "SELECT body FROM works WHERE id=?", id).Scan(&body)
	if errors.Is(e, sql.ErrNoRows) {
		return w, fail(404, "not_found", "成品不存在")
	}
	if e != nil {
		return w, e
	}
	e = json.Unmarshal([]byte(body), &w)
	return w, e
}

func attachWork(ctx context.Context, q queryer, w *Work) error {
	w.Photos = []Media{}
	for _, id := range w.PhotoIDs {
		var b string
		if e := q.QueryRowContext(ctx, "SELECT body FROM media WHERE id=? AND work_id=? AND removed_at IS NULL", id, w.ID).Scan(&b); e != nil {
			return e
		}
		var m Media
		if e := json.Unmarshal([]byte(b), &m); e != nil {
			return e
		}
		w.Photos = append(w.Photos, m)
	}
	for i := range w.Fabrics {
		f := &w.Fabrics[i]
		var name string
		var deleted sql.NullString
		e := q.QueryRowContext(ctx, "SELECT json_extract(body,'$.name'),deleted_at FROM fabrics WHERE id=?", f.FabricID).Scan(&name, &deleted)
		if e != nil && !errors.Is(e, sql.ErrNoRows) {
			return e
		}
		f.Available = e == nil && !deleted.Valid
		if f.Available {
			f.Name = name
		}
	}
	return nil
}

func (s *Store) GetWork(ctx context.Context, id string) (Work, error) {
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if e != nil {
		return Work{}, e
	}
	defer tx.Rollback()
	w, e := loadWork(ctx, tx, id)
	if e == nil {
		e = attachWork(ctx, tx, &w)
	}
	return w, e
}

func (s *Store) WriteWork(ctx context.Context, id, key, fp, action string, in Work) ([]byte, error) {
	if !idPattern.MatchString(key) {
		return nil, fail(400, "idempotency_required", "缺少有效提交编号")
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	if cached, e := replay(ctx, tx, key, fp); e != nil || cached != nil {
		return cached, e
	}
	var before *Work
	w := Work{ID: ID(), CreatedAt: now()}
	if id != "" {
		old, e := loadWork(ctx, tx, id)
		if e != nil {
			return nil, e
		}
		if old.Revision != in.Revision {
			return nil, fail(409, "revision_conflict", "这件成品已在另一处修改，请保留输入并核对最新记录")
		}
		if old.DeletedAt != nil && action != "restore" {
			return nil, fail(409, "deleted", "这件成品已移入回收站")
		}
		before, w = &old, old
	} else if action != "save" {
		return nil, fail(400, "existing_required", "请先建立成品记录")
	}
	switch action {
	case "save":
		in.ID, in.CreatedAt, in.DeletedAt, in.Photos = w.ID, w.CreatedAt, nil, nil
		w = in
		if e = w.Validate(); e != nil {
			return nil, e
		}
		oldLinks := map[string]WorkFabric{}
		if before != nil {
			for _, f := range before.Fabrics {
				oldLinks[f.FabricID] = f
			}
		}
		for i := range w.Fabrics {
			link := &w.Fabrics[i]
			var name string
			var deleted sql.NullString
			e = tx.QueryRowContext(ctx, "SELECT json_extract(body,'$.name'),deleted_at FROM fabrics WHERE id=?", link.FabricID).Scan(&name, &deleted)
			if e != nil && !errors.Is(e, sql.ErrNoRows) {
				return nil, e
			}
			if e == nil && !deleted.Valid {
				link.Name = name
			} else if old, ok := oldLinks[link.FabricID]; ok {
				link.Name = old.Name
			} else {
				return nil, invalid("fabrics", "关联布料已删除或不存在，请重新选择")
			}
			link.Available = false // computed on reads, never trust the client
		}
		for _, mid := range w.PhotoIDs {
			var owner, fabric, removed sql.NullString
			var created string
			e = tx.QueryRowContext(ctx, "SELECT work_id,fabric_id,removed_at,created_at FROM media WHERE id=?", mid).Scan(&owner, &fabric, &removed, &created)
			if errors.Is(e, sql.ErrNoRows) {
				return nil, invalid("photos", "照片不存在或已过期，请重新上传")
			}
			if e != nil {
				return nil, e
			}
			if fabric.Valid || (owner.Valid && owner.String != w.ID) {
				return nil, invalid("photos", "照片已经属于另一条记录")
			}
			t, _ := time.Parse(time.RFC3339Nano, created)
			if removed.Valid || (!owner.Valid && time.Since(t) > 24*time.Hour) {
				return nil, invalid("photos", "照片已过期，请重新上传")
			}
		}
	case "delete":
		t := now()
		w.DeletedAt = &t
	case "restore":
		if w.DeletedAt == nil {
			return nil, fail(409, "not_deleted", "成品不在回收站")
		}
		t, _ := time.Parse(time.RFC3339Nano, *w.DeletedAt)
		if time.Since(t) > 30*24*time.Hour {
			return nil, fail(410, "expired", "回收期限已过")
		}
		w.DeletedAt = nil
	default:
		return nil, fail(400, "action", "未知操作")
	}
	w.Revision = 1
	if before != nil {
		w.Revision = before.Revision + 1
	}
	w.UpdatedAt, w.Photos = now(), nil
	body, _ := json.Marshal(w)
	if _, e = tx.ExecContext(ctx, `INSERT INTO works(id,revision,body,created_at,updated_at,deleted_at) VALUES(?,?,?,?,?,?)
ON CONFLICT(id) DO UPDATE SET revision=excluded.revision,body=excluded.body,updated_at=excluded.updated_at,deleted_at=excluded.deleted_at`,
		w.ID, w.Revision, string(body), w.CreatedAt, w.UpdatedAt, w.DeletedAt); e != nil {
		return nil, e
	}
	if action == "save" {
		if _, e = tx.ExecContext(ctx, "UPDATE media SET removed_at=? WHERE work_id=? AND removed_at IS NULL", w.UpdatedAt, w.ID); e != nil {
			return nil, e
		}
		for _, mid := range w.PhotoIDs {
			if _, e = tx.ExecContext(ctx, "UPDATE media SET work_id=?,removed_at=NULL WHERE id=?", w.ID, mid); e != nil {
				return nil, e
			}
		}
	}
	change, _ := json.Marshal(WorkChange{w.Revision, action, w.UpdatedAt, before, &w})
	if _, e = tx.ExecContext(ctx, "INSERT INTO work_changes VALUES(?,?,?)", w.ID, w.Revision, string(change)); e != nil {
		return nil, e
	}
	if e = attachWork(ctx, tx, &w); e != nil {
		return nil, e
	}
	result, _ := json.Marshal(w)
	if _, e = tx.ExecContext(ctx, "INSERT INTO operations VALUES(?,?,?,?)", key, fp, string(result), now()); e != nil {
		return nil, e
	}
	if e = tx.Commit(); e != nil {
		return nil, e
	}
	return result, nil
}

type WorkList struct {
	Items      []Work   `json:"items"`
	Total      int      `json:"total"`
	Offset     int      `json:"offset"`
	Categories []string `json:"categories"`
}

func (s *Store) ListWorks(ctx context.Context, v url.Values, all bool) (WorkList, error) {
	list := WorkList{Items: []Work{}, Categories: []string{}}
	clauses, args := []string{"deleted_at IS NULL"}, []any{}
	if v.Get("trash") == "1" {
		clauses[0] = "deleted_at IS NOT NULL"
	}
	if q := strings.TrimSpace(v.Get("q")); q != "" {
		if len(q) > 1000 {
			return list, invalid("q", "搜索内容过长")
		}
		clauses = append(clauses, `instr(lower(json_extract(body,'$.name')||' '||json_extract(body,'$.category')||' '||
json_extract(body,'$.pattern')||' '||json_extract(body,'$.recipient')||' '||json_extract(body,'$.size')||' '||
json_extract(body,'$.notes')||' '||json_extract(body,'$.tags')||' '||json_extract(body,'$.fabrics')),lower(?))>0`)
		args = append(args, q)
	}
	if category := v.Get("category"); category != "" {
		clauses = append(clauses, "json_extract(body,'$.category')=?")
		args = append(args, category)
	}
	if id := v.Get("fabric"); id != "" {
		if !idPattern.MatchString(id) {
			return list, invalid("fabric", "布料编号无效")
		}
		clauses = append(clauses, "EXISTS(SELECT 1 FROM json_each(body,'$.fabrics') WHERE json_extract(value,'$.fabricId')=?)")
		args = append(args, id)
	}
	order := "coalesce(nullif(json_extract(body,'$.completedDate'),''),substr(created_at,1,10)) DESC,id DESC"
	switch v.Get("sort") {
	case "", "completed":
	case "updated":
		order = "updated_at DESC,id DESC"
	case "name":
		order = "json_extract(body,'$.name') COLLATE NOCASE,id"
	default:
		return list, invalid("sort", "排序方式无效")
	}
	if offset := v.Get("offset"); offset != "" && !all {
		n, e := strconv.Atoi(offset)
		if e != nil || n < 0 || n > 1000000 {
			return list, invalid("offset", "分页位置无效")
		}
		list.Offset = n
	}
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if e != nil {
		return list, e
	}
	defer tx.Rollback()
	where := strings.Join(clauses, " AND ")
	if e = tx.QueryRowContext(ctx, "SELECT count(*) FROM works WHERE "+where, args...).Scan(&list.Total); e != nil {
		return list, e
	}
	query := "SELECT body FROM works WHERE " + where + " ORDER BY " + order
	if !all {
		query += " LIMIT 24 OFFSET ?"
		args = append(args, list.Offset)
	}
	rows, e := tx.QueryContext(ctx, query, args...)
	if e != nil {
		return list, e
	}
	for rows.Next() {
		var b string
		var w Work
		if e = rows.Scan(&b); e != nil {
			break
		}
		if e = json.Unmarshal([]byte(b), &w); e != nil {
			break
		}
		list.Items = append(list.Items, w)
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		return list, e
	}
	for i := range list.Items {
		if e = attachWork(ctx, tx, &list.Items[i]); e != nil {
			return list, e
		}
	}
	rows, e = tx.QueryContext(ctx, "SELECT DISTINCT json_extract(body,'$.category') AS category FROM works WHERE deleted_at IS NULL AND category!='' ORDER BY category LIMIT 200")
	if e != nil {
		return list, e
	}
	defer rows.Close()
	for rows.Next() {
		var category string
		if e = rows.Scan(&category); e != nil {
			return list, e
		}
		list.Categories = append(list.Categories, category)
	}
	return list, rows.Err()
}

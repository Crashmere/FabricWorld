package app

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// Counts always come from current fabrics, including trash. Catalog entries
// retain explicitly added names even before any fabric uses them.
type MaterialUsage struct {
	Name       string `json:"name"`
	Count      int    `json:"count"`
	TrashCount int    `json:"trashCount"`
	Version    string `json:"version"`
}

type RemoveMaterialInput struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type AddMaterialInput struct {
	Name string `json:"name"`
}

type RemoveMaterialResult struct {
	Name     string `json:"name"`
	Affected int    `json:"affected"`
}

func materialVersion(refs []string) string {
	b, _ := json.Marshal(refs)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func (s *Store) Materials(ctx context.Context) ([]MaterialUsage, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT name,id,revision,trash,registered FROM (
SELECT DISTINCT m.value AS name, f.id, f.revision, f.deleted_at IS NOT NULL AS trash, 0 AS registered
FROM fabrics f, json_each(f.body, '$.materials') m
UNION ALL SELECT name,id,0,0,1 FROM material_catalog)
ORDER BY name,registered DESC,id`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	items := []MaterialUsage{}
	refs := []string{}
	for rows.Next() {
		var name, id string
		var revision int
		var trash, registered bool
		if e = rows.Scan(&name, &id, &revision, &trash, &registered); e != nil {
			return nil, e
		}
		if len(items) == 0 || items[len(items)-1].Name != name {
			if len(items) > 0 {
				items[len(items)-1].Version = materialVersion(refs)
			}
			items = append(items, MaterialUsage{Name: name})
			refs = []string{}
		}
		if registered {
			refs = append(refs, "catalog:"+id)
			continue
		}
		item := &items[len(items)-1]
		item.Count++
		if trash {
			item.TrashCount++
		}
		refs = append(refs, fmt.Sprintf("%s:%d", id, revision))
	}
	if len(items) > 0 {
		items[len(items)-1].Version = materialVersion(refs)
	}
	return items, rows.Err()
}

func (s *Store) AddMaterial(ctx context.Context, key, fp string, in AddMaterialInput) ([]byte, error) {
	if !idPattern.MatchString(key) {
		return nil, fail(400, "idempotency_required", "缺少有效提交编号")
	}
	if e := textField(&in.Name, "name", 40); e != nil {
		return nil, e
	}
	if in.Name == "" {
		return nil, invalid("name", "请输入材质名称")
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	if cached, e := replay(ctx, tx, key, fp); e != nil || cached != nil {
		return cached, e
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO material_catalog(name,id) VALUES(?,?) ON CONFLICT(name) DO NOTHING", in.Name, ID()); e != nil {
		return nil, e
	}
	result, _ := json.Marshal(in)
	if _, e = tx.ExecContext(ctx, "INSERT INTO operations VALUES(?,?,?,?)", key, fp, string(result), now()); e != nil {
		return nil, e
	}
	if e = tx.Commit(); e != nil {
		return nil, e
	}
	return result, nil
}

func (s *Store) RemoveMaterial(ctx context.Context, key, fp string, in RemoveMaterialInput) ([]byte, error) {
	if !idPattern.MatchString(key) {
		return nil, fail(400, "idempotency_required", "缺少有效提交编号")
	}
	if e := textField(&in.Name, "name", 40); e != nil {
		return nil, e
	}
	if in.Name == "" || len(in.Version) != 64 {
		return nil, invalid("material", "请刷新后选择要移除的材质")
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	if cached, e := replay(ctx, tx, key, fp); e != nil || cached != nil {
		return cached, e
	}
	refs := []string{}
	var catalogID string
	e = tx.QueryRowContext(ctx, "SELECT id FROM material_catalog WHERE name=?", in.Name).Scan(&catalogID)
	if e == nil {
		refs = append(refs, "catalog:"+catalogID)
	} else if !errors.Is(e, sql.ErrNoRows) {
		return nil, e
	}
	rows, e := tx.QueryContext(ctx, `SELECT body FROM fabrics WHERE EXISTS
(SELECT 1 FROM json_each(fabrics.body, '$.materials') WHERE value=?) ORDER BY id`, in.Name)
	if e != nil {
		return nil, e
	}
	fabrics := []Fabric{}
	for rows.Next() {
		var body string
		var f Fabric
		if e = rows.Scan(&body); e == nil {
			e = json.Unmarshal([]byte(body), &f)
		}
		if e != nil {
			rows.Close()
			return nil, e
		}
		fabrics = append(fabrics, f)
		refs = append(refs, fmt.Sprintf("%s:%d", f.ID, f.Revision))
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	if materialVersion(refs) != in.Version {
		return nil, fail(409, "material_changed", "相关布料已发生变化，请刷新列表后重新确认")
	}
	if _, e = tx.ExecContext(ctx, "DELETE FROM material_catalog WHERE name=?", in.Name); e != nil {
		return nil, e
	}
	at := now()
	for _, before := range fabrics {
		f := before
		f.Materials = []string{}
		f.MaterialPercentages = map[string]string{}
		for _, name := range before.Materials {
			if name != in.Name {
				f.Materials = append(f.Materials, name)
				f.MaterialPercentages[name] = before.MaterialPercentages[name]
			}
		}
		if len(f.Materials) == 1 {
			f.MaterialPercentages[f.Materials[0]] = "100"
		}
		f.Revision++
		f.UpdatedAt = at
		body, e := json.Marshal(f)
		if e != nil {
			return nil, e
		}
		if _, e = tx.ExecContext(ctx, "UPDATE fabrics SET body=?,revision=?,updated_at=? WHERE id=?", string(body), f.Revision, at, f.ID); e != nil {
			return nil, e
		}
		change, e := json.Marshal(Change{Revision: f.Revision, Action: "remove_material", At: at, Before: &before, After: &f})
		if e != nil {
			return nil, e
		}
		if _, e = tx.ExecContext(ctx, "INSERT INTO changes VALUES(?,?,?)", f.ID, f.Revision, string(change)); e != nil {
			return nil, e
		}
	}
	result, _ := json.Marshal(RemoveMaterialResult{Name: in.Name, Affected: len(fabrics)})
	if _, e = tx.ExecContext(ctx, "INSERT INTO operations VALUES(?,?,?,?)", key, fp, string(result), at); e != nil {
		return nil, e
	}
	if e = tx.Commit(); e != nil {
		return nil, e
	}
	return result, nil
}

func (s *Store) removeMaterialHTTP(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var in RemoveMaterialInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if e := decoder.Decode(&in); e != nil {
		return fail(400, "json", "材质请求格式无效")
	}
	body, _ := json.Marshal(in)
	result, e := s.RemoveMaterial(r.Context(), r.Header.Get("Idempotency-Key"), fingerprint(r.Method, r.URL.Path, body), in)
	if e != nil {
		return e
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, e = w.Write(result)
	return e
}

func (s *Store) addMaterialHTTP(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var in AddMaterialInput
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if e := decoder.Decode(&in); e != nil {
		return fail(400, "json", "材质请求格式无效")
	}
	body, _ := json.Marshal(in)
	result, e := s.AddMaterial(r.Context(), r.Header.Get("Idempotency-Key"), fingerprint(r.Method, r.URL.Path, body), in)
	if e != nil {
		return e
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, e = w.Write(result)
	return e
}

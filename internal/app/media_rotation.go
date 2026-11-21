package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"time"
)

type RotationInput struct {
	Turns    int  `json:"turns"`
	Revision int  `json:"revision"`
	Draft    bool `json:"draft"`
}

type RotationResult struct {
	Photo  Media   `json:"photo"`
	Fabric *Fabric `json:"fabric,omitempty"`
	Work   *Work   `json:"work,omitempty"`
}

type rotationOwner struct {
	fabric sql.NullString
	work   sql.NullString
}

func rotationSource(ctx context.Context, q queryer, id string, in RotationInput) (rotationOwner, error) {
	var owner rotationOwner
	var removed, deleted sql.NullString
	var revision sql.NullInt64
	var created string
	e := q.QueryRowContext(ctx, `SELECT m.fabric_id,m.work_id,m.removed_at,m.created_at,
coalesce(f.deleted_at,w.deleted_at),coalesce(f.revision,w.revision)
FROM media m LEFT JOIN fabrics f ON f.id=m.fabric_id LEFT JOIN works w ON w.id=m.work_id WHERE m.id=?`, id).
		Scan(&owner.fabric, &owner.work, &removed, &created, &deleted, &revision)
	if errors.Is(e, sql.ErrNoRows) || removed.Valid || deleted.Valid {
		return owner, fail(404, "not_found", "照片不存在或已被移除，请刷新记录")
	}
	if e != nil {
		return owner, e
	}
	t, _ := time.Parse(time.RFC3339Nano, created)
	if !revision.Valid && time.Since(t) > 24*time.Hour {
		return owner, fail(404, "expired", "照片已过期，请重新上传")
	}
	if !in.Draft && revision.Valid && int(revision.Int64) != in.Revision {
		return owner, fail(409, "revision_conflict", "记录已在另一处修改，请关闭预览并刷新核对")
	}
	return owner, nil
}

func (s *Store) rotatePhotoHTTP(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	body, e := io.ReadAll(r.Body)
	if e != nil {
		return fail(413, "too_large", "旋转请求过大")
	}
	var in RotationInput
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if e = decoder.Decode(&in); e != nil {
		return fail(400, "json", "旋转请求格式无效")
	}
	if decoder.Decode(new(any)) != io.EOF {
		return fail(400, "json", "旋转请求格式无效")
	}
	result, e := s.RotatePhoto(r.Context(), r.PathValue("id"), r.Header.Get("Idempotency-Key"), fingerprint(r.Method, r.URL.Path, body), in)
	if e != nil {
		return e
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, e = w.Write(result)
	return e
}

func (s *Store) RotatePhoto(ctx context.Context, id, key, fp string, in RotationInput) ([]byte, error) {
	if !idPattern.MatchString(id) {
		return nil, fail(404, "not_found", "照片不存在")
	}
	if !idPattern.MatchString(key) {
		return nil, fail(400, "idempotency_required", "缺少有效提交编号")
	}
	if in.Turns < 1 || in.Turns > 3 || in.Revision < 0 {
		return nil, invalid("turns", "请选择旋转 90°、180° 或 270°")
	}
	select {
	case s.UploadSlot <- struct{}{}:
		defer func() { <-s.UploadSlot }()
	default:
		return nil, fail(429, "busy", "正在处理照片，请稍后重试")
	}
	unlock, e := s.Lock(false)
	if e != nil {
		return nil, e
	}
	defer unlock()
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if e != nil {
		return nil, e
	}
	cached, e := replay(ctx, tx, key, fp)
	tx.Rollback()
	if e != nil || cached != nil {
		return cached, e
	}
	owner, e := rotationSource(ctx, s.DB, id, in)
	if e != nil {
		return nil, e
	}
	if e = s.capacity(); e != nil {
		return nil, e
	}
	tmp, e := os.MkdirTemp(filepath.Join(s.Dir, "tmp"), "rotate-")
	if e != nil {
		return nil, e
	}
	defer os.RemoveAll(tmp)
	ictx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	main, thumb := filepath.Join(tmp, "main.jpg"), filepath.Join(tmp, "thumb.webp")
	angle := []string{"", "d90", "d180", "d270"}[in.Turns]
	// The stored JPEG is already upright and sRGB; rotate pixels, not EXIF.
	if _, e = runImage(ictx, "vips", "rot", filepath.Join(s.Dir, "media", id+".jpg"), main+"[Q=88,strip]", angle); e != nil {
		log.Printf("image rotation: %v", e)
		return nil, fail(422, "image_decode", "照片旋转失败，请重试")
	}
	if _, e = runImage(ictx, "vips", "thumbnail", main, thumb+"[Q=80,strip]", "640", "--height=640", "--size=down"); e != nil {
		return nil, e
	}
	m, e := s.publishMedia(ictx, main, thumb)
	commitAttempted := false
	defer func() {
		if !commitAttempted && m.ID != "" {
			os.Remove(filepath.Join(s.Dir, "media", m.ID+".jpg"))
			os.Remove(filepath.Join(s.Dir, "media", m.ID+".webp"))
		}
	}()
	if e != nil {
		return nil, e
	}
	tx, e = s.DB.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	if cached, e := replay(ctx, tx, key, fp); e != nil || cached != nil {
		return cached, e
	}
	// Other record writes can run while libvips is working; recheck at commit.
	current, e := rotationSource(ctx, tx, id, in)
	if e != nil {
		return nil, e
	}
	if current != owner {
		return nil, fail(409, "revision_conflict", "照片所属记录已改变，请刷新核对")
	}
	b, _ := json.Marshal(m)
	if _, e = tx.ExecContext(ctx, "INSERT INTO media(id,body,created_at) VALUES(?,?,?)", m.ID, string(b), m.CreatedAt); e != nil {
		return nil, e
	}
	result := RotationResult{Photo: m}
	if !in.Draft && (owner.fabric.Valid || owner.work.Valid) {
		if _, e = tx.ExecContext(ctx, "UPDATE media SET fabric_id=?,work_id=? WHERE id=?", owner.fabric, owner.work, m.ID); e != nil {
			return nil, e
		}
		if _, e = tx.ExecContext(ctx, "UPDATE media SET removed_at=? WHERE id=?", m.CreatedAt, id); e != nil {
			return nil, e
		}
		if owner.fabric.Valid {
			f, e := rotateFabricRecord(ctx, tx, owner.fabric.String, id, m)
			if e != nil {
				return nil, e
			}
			result.Fabric = &f
		} else {
			w, e := rotateWorkRecord(ctx, tx, owner.work.String, id, m)
			if e != nil {
				return nil, e
			}
			result.Work = &w
		}
	}
	b, _ = json.Marshal(result)
	if _, e = tx.ExecContext(ctx, "INSERT INTO operations VALUES(?,?,?,?)", key, fp, string(b), now()); e != nil {
		return nil, e
	}
	// Preserve files after an uncertain commit; normal orphan cleanup handles them.
	commitAttempted = true
	if e = tx.Commit(); e != nil {
		return nil, e
	}
	return b, nil
}

func rotatedPhotoIDs(ids []string, oldID, newID string) ([]string, error) {
	i := slices.Index(ids, oldID)
	if i < 0 {
		return nil, fail(409, "revision_conflict", "这张照片已不在记录中，请刷新核对")
	}
	copy := slices.Clone(ids)
	copy[i] = newID
	return copy, nil
}

func rotateFabricRecord(ctx context.Context, tx *sql.Tx, id, photoID string, m Media) (Fabric, error) {
	before, e := load(ctx, tx, id)
	if e != nil {
		return Fabric{}, e
	}
	f := before
	f.PhotoIDs, e = rotatedPhotoIDs(before.PhotoIDs, photoID, m.ID)
	if e != nil {
		return f, e
	}
	f.Revision++
	f.UpdatedAt, f.Photos = m.CreatedAt, nil
	body, _ := json.Marshal(f)
	if _, e = tx.ExecContext(ctx, "UPDATE fabrics SET revision=?,body=?,updated_at=? WHERE id=?", f.Revision, string(body), f.UpdatedAt, id); e != nil {
		return f, e
	}
	change, _ := json.Marshal(Change{f.Revision, "rotate_photo", f.UpdatedAt, &before, &f})
	if _, e = tx.ExecContext(ctx, "INSERT INTO changes VALUES(?,?,?)", id, f.Revision, string(change)); e != nil {
		return f, e
	}
	e = attach(ctx, tx, &f)
	return f, e
}

func rotateWorkRecord(ctx context.Context, tx *sql.Tx, id, photoID string, m Media) (Work, error) {
	before, e := loadWork(ctx, tx, id)
	if e != nil {
		return Work{}, e
	}
	w := before
	w.PhotoIDs, e = rotatedPhotoIDs(before.PhotoIDs, photoID, m.ID)
	if e != nil {
		return w, e
	}
	w.Revision++
	w.UpdatedAt, w.Photos = m.CreatedAt, nil
	body, _ := json.Marshal(w)
	if _, e = tx.ExecContext(ctx, "UPDATE works SET revision=?,body=?,updated_at=? WHERE id=?", w.Revision, string(body), w.UpdatedAt, id); e != nil {
		return w, e
	}
	change, _ := json.Marshal(WorkChange{w.Revision, "rotate_photo", w.UpdatedAt, &before, &w})
	if _, e = tx.ExecContext(ctx, "INSERT INTO work_changes VALUES(?,?,?)", id, w.Revision, string(change)); e != nil {
		return w, e
	}
	e = attachWork(ctx, tx, &w)
	return w, e
}

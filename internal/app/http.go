package app

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func jsonResponse(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func errorResponse(w http.ResponseWriter, e error) {
	var p *Problem
	if !errors.As(e, &p) {
		log.Printf("request error: %v", e)
		p = &Problem{Status: 500, Code: "internal", Message: "服务暂时无法完成请求，请稍后重试"}
	}
	jsonResponse(w, p.Status, p)
}
func originOK(r *http.Request) bool {
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, e := url.Parse(origin)
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return e == nil && u.Host == r.Host && u.Scheme == scheme && u.Path == ""
}
func (s *Store) Handler(assets fs.FS) http.Handler {
	limits := &limiter{}
	mux := http.NewServeMux()
	handle := func(pattern string, fn func(http.ResponseWriter, *http.Request) error) {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			if e := fn(w, r); e != nil {
				errorResponse(w, e)
			}
		})
	}
	handle("GET /healthz", func(w http.ResponseWriter, r *http.Request) error {
		if e := s.DB.PingContext(r.Context()); e != nil {
			return e
		}
		jsonResponse(w, 200, map[string]string{"status": "ok"})
		return nil
	})
	handle("GET /api/fabrics", func(w http.ResponseWriter, r *http.Request) error {
		v, e := s.List(r.Context(), r.URL.Query(), false)
		if e == nil {
			jsonResponse(w, 200, v)
		}
		return e
	})
	handle("GET /api/fabrics/{id}", func(w http.ResponseWriter, r *http.Request) error {
		v, e := s.Get(r.Context(), r.PathValue("id"))
		if e == nil {
			jsonResponse(w, 200, v)
		}
		return e
	})
	write := func(action string) func(http.ResponseWriter, *http.Request) error {
		return func(w http.ResponseWriter, r *http.Request) error {
			r.Body = http.MaxBytesReader(w, r.Body, 256<<10)
			body, e := io.ReadAll(r.Body)
			if e != nil {
				return fail(413, "too_large", "表单内容过大")
			}
			var in WriteInput
			dec := json.NewDecoder(bytes.NewReader(body))
			dec.DisallowUnknownFields()
			if e = dec.Decode(&in); e != nil {
				return fail(400, "json", "表单格式无效")
			}
			a := action
			if action == "save" && in.Action == "remnant" {
				a = "remnant"
			}
			result, e := s.Write(r.Context(), r.PathValue("id"), r.Header.Get("Idempotency-Key"), fingerprint(r.Method, r.URL.Path, body), a, in.Fabric)
			if e != nil {
				return e
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Write(result)
			return nil
		}
	}
	handle("POST /api/integrations/ledger", s.importLedger)
	handle("POST /api/fabrics", write("save"))
	handle("PUT /api/fabrics/{id}", write("save"))
	handle("DELETE /api/fabrics/{id}", write("delete"))
	handle("POST /api/fabrics/{id}/restore", write("restore"))
	handle("GET /api/fabrics/{id}/changes", func(w http.ResponseWriter, r *http.Request) error {
		rows, e := s.DB.QueryContext(r.Context(), "SELECT body FROM changes WHERE fabric_id=? ORDER BY revision DESC LIMIT 100", r.PathValue("id"))
		if e != nil {
			return e
		}
		defer rows.Close()
		v := []json.RawMessage{}
		for rows.Next() {
			var b string
			if e = rows.Scan(&b); e != nil {
				return e
			}
			v = append(v, json.RawMessage(b))
		}
		if e = rows.Err(); e != nil {
			return e
		}
		jsonResponse(w, 200, v)
		return nil
	})
	handle("GET /api/suggestions", func(w http.ResponseWriter, r *http.Request) error {
		v, e := s.Suggestions(r.Context())
		if e == nil {
			jsonResponse(w, 200, v)
		}
		return e
	})
	handle("GET /api/operations/{key}", func(w http.ResponseWriter, r *http.Request) error {
		var result string
		e := s.DB.QueryRowContext(r.Context(), "SELECT result FROM operations WHERE key=?", r.PathValue("key")).Scan(&result)
		if errors.Is(e, sql.ErrNoRows) {
			return fail(404, "unknown", "尚未查询到提交结果")
		}
		if e != nil {
			return e
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write([]byte(result))
		return nil
	})
	handle("POST /api/uploads", func(w http.ResponseWriter, r *http.Request) error {
		r.Body = http.MaxBytesReader(w, r.Body, MaxUpload+64*1024)
		reader, e := r.MultipartReader()
		if e != nil {
			return fail(400, "multipart", "照片上传格式无效")
		}
		part, e := reader.NextPart()
		if e != nil || part.FormName() != "file" {
			return fail(400, "file", "请选择照片")
		}
		defer part.Close()
		m, e := s.Upload(r.Context(), r.Header.Get("Idempotency-Key"), part)
		if e != nil {
			return e
		}
		jsonResponse(w, 200, m)
		return nil
	})
	handle("GET /media/{id}/{variant}", func(w http.ResponseWriter, r *http.Request) error {
		p, e := s.MediaPath(r.Context(), r.PathValue("id"), r.PathValue("variant"))
		if e != nil {
			return e
		}
		w.Header().Set("Cache-Control", "private, max-age=300")
		http.ServeFile(w, r, p)
		return nil
	})
	handle("GET /api/export", s.export)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		errorResponse(w, fail(404, "not_found", "接口不存在"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			http.Error(w, "method not allowed", 405)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" {
			name = "index.html"
		}
		b, e := fs.ReadFile(assets, name)
		if e != nil {
			if strings.HasPrefix(name, "assets/") || strings.Contains(filepath.Base(name), ".") {
				http.NotFound(w, r)
				return
			}
			name = "index.html"
			b, e = fs.ReadFile(assets, name)
		}
		if e != nil {
			http.Error(w, "frontend unavailable", 503)
			return
		}
		if strings.HasPrefix(name, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(b))
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				log.Printf("panic: %v", v)
				errorResponse(w, fmt.Errorf("internal panic"))
			}
		}()
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("X-Robots-Tag", "noindex, nofollow")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' blob: data:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'; form-action 'self'")
		if r.Method != "GET" && r.Method != "HEAD" && !originOK(r) {
			errorResponse(w, fail(403, "origin", "请从本站页面提交"))
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" && !limits.allow(r) {
			w.Header().Set("Retry-After", "60")
			errorResponse(w, fail(429, "rate_limit", "操作较频繁，请稍后重试"))
			return
		}
		mux.ServeHTTP(w, r)
	})
}
func csvText(s string) string {
	trim := strings.TrimLeft(s, " \t\r\n")
	if trim != "" && strings.ContainsAny(trim[:1], "=+-@") {
		return "'" + s
	}
	return s
}
func (s *Store) export(w http.ResponseWriter, r *http.Request) error {
	select {
	case s.ExportSlot <- struct{}{}:
		defer func() { <-s.ExportSlot }()
	default:
		return fail(429, "busy", "已有导出任务，请稍后重试")
	}
	unlock, e := s.Lock(false)
	if e != nil {
		return e
	}
	defer unlock()
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	http.NewResponseController(w).SetWriteDeadline(time.Now().Add(5 * time.Minute))
	v := r.URL.Query()
	v.Del("trash")
	if v.Get("status") == "" {
		v.Set("status", "stock")
	}
	list, e := s.List(ctx, v, true)
	if e != nil {
		return e
	}
	format := v.Get("format")
	if format != "csv" && format != "zip" {
		return invalid("format", "请选择 CSV 或 ZIP")
	}
	if format == "csv" {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="fabricworld.csv"`)
		w.Write([]byte{239, 187, 191})
		cw := csv.NewWriter(w)
		cw.Write([]string{"名称", "材质", "成分", "状态", "收纳位置", "剩余尺寸", "颜色", "标签", "购买日期", "店铺", "购买总价", "备注"})
		for _, f := range list.Items {
			pieces := []string{}
			for _, p := range f.Pieces {
				pieces = append(pieces, fmt.Sprintf("%s×%s %s × %d", p.Width, p.Length, p.Unit, p.Count))
			}
			row := []string{f.Name, f.MaterialText(), f.Composition, map[string]string{"unused": "未使用", "using": "使用中", "used": "已用完"}[f.Status], f.Location, strings.Join(pieces, "；"), f.Color, strings.Join(f.Tags, "、"), f.PurchaseDate, f.Shop, f.Price, f.Notes}
			for i := range row {
				row[i] = csvText(row[i])
			}
			if e = cw.Write(row); e != nil {
				return nil
			}
		}
		cw.Flush()
		return nil
	}
	for _, f := range list.Items {
		for _, id := range f.PhotoIDs {
			if _, e = os.Stat(filepath.Join(s.Dir, "media", id+".jpg")); e != nil {
				return e
			}
		}
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="fabricworld.zip"`)
	zw := zip.NewWriter(w)
	entry, e := zw.Create("fabrics.json")
	if e != nil {
		return nil
	}
	json.NewEncoder(entry).Encode(map[string]any{"version": 1, "exportedAt": now(), "fabrics": list.Items})
	for _, f := range list.Items {
		for i, id := range f.PhotoIDs {
			if ctx.Err() != nil {
				return nil
			}
			entry, e = zw.CreateHeader(&zip.FileHeader{Name: "photos/" + f.ID + "/" + strconv.Itoa(i+1) + "-" + id + ".jpg", Method: zip.Store})
			if e != nil {
				return nil
			}
			file, e := os.Open(filepath.Join(s.Dir, "media", id+".jpg"))
			if e != nil {
				return nil
			}
			_, e = io.Copy(entry, file)
			file.Close()
			if e != nil {
				return nil
			}
		}
	}
	zw.Close()
	return nil
}

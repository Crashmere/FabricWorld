package app

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (s *Store) workRoutes(handle func(string, func(http.ResponseWriter, *http.Request) error)) {
	handle("GET /api/works", func(w http.ResponseWriter, r *http.Request) error {
		v, e := s.ListWorks(r.Context(), r.URL.Query(), false)
		if e == nil {
			jsonResponse(w, 200, v)
		}
		return e
	})
	handle("GET /api/works/{id}", func(w http.ResponseWriter, r *http.Request) error {
		v, e := s.GetWork(r.Context(), r.PathValue("id"))
		if e == nil {
			jsonResponse(w, 200, v)
		}
		return e
	})
	write := func(action string) func(http.ResponseWriter, *http.Request) error {
		return func(w http.ResponseWriter, r *http.Request) error {
			body, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 256<<10))
			if e != nil {
				return fail(413, "too_large", "表单内容过大")
			}
			var in Work
			dec := json.NewDecoder(bytes.NewReader(body))
			dec.DisallowUnknownFields()
			if e = dec.Decode(&in); e != nil {
				return fail(400, "json", "表单格式无效")
			}
			if dec.Decode(new(any)) != io.EOF {
				return fail(400, "json", "表单格式无效")
			}
			result, e := s.WriteWork(r.Context(), r.PathValue("id"), r.Header.Get("Idempotency-Key"), fingerprint(r.Method, r.URL.Path, body), action, in)
			if e != nil {
				return e
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Write(result)
			return nil
		}
	}
	handle("POST /api/works", write("save"))
	handle("PUT /api/works/{id}", write("save"))
	handle("DELETE /api/works/{id}", write("delete"))
	handle("POST /api/works/{id}/restore", write("restore"))
	handle("GET /api/works/{id}/changes", func(w http.ResponseWriter, r *http.Request) error {
		if _, e := s.GetWork(r.Context(), r.PathValue("id")); e != nil {
			return e
		}
		rows, e := s.DB.QueryContext(r.Context(), "SELECT body FROM work_changes WHERE work_id=? ORDER BY revision DESC LIMIT 100", r.PathValue("id"))
		if e != nil {
			return e
		}
		defer rows.Close()
		changes := []json.RawMessage{}
		for rows.Next() {
			var b string
			if e = rows.Scan(&b); e != nil {
				return e
			}
			changes = append(changes, json.RawMessage(b))
		}
		if e = rows.Err(); e != nil {
			return e
		}
		jsonResponse(w, 200, changes)
		return nil
	})
	handle("GET /api/works/export", s.exportWorks)
}

func (s *Store) exportWorks(w http.ResponseWriter, r *http.Request) error {
	format := r.URL.Query().Get("format")
	if format != "csv" && format != "zip" {
		return invalid("format", "请选择 CSV 或 ZIP")
	}
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
	list, e := s.ListWorks(ctx, v, true)
	if e != nil {
		return e
	}
	if format == "csv" {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="fabricworld-works.csv"`)
		w.Write([]byte{239, 187, 191})
		cw := csv.NewWriter(w)
		cw.Write([]string{"成品名称", "类别", "完成日期", "纸样", "尺码", "为谁制作", "标签", "所用布料", "制作心得"})
		for _, item := range list.Items {
			fabrics := []string{}
			for _, f := range item.Fabrics {
				fabrics = append(fabrics, f.Name+"（"+f.Note+"）")
			}
			row := []string{item.Name, item.Category, item.CompletedDate, item.Pattern, item.Size, item.Recipient, strings.Join(item.Tags, "、"), strings.Join(fabrics, "；"), item.Notes}
			for i := range row {
				row[i] = csvText(row[i])
			}
			if e = cw.Write(row); e != nil {
				return nil // headers are already sent
			}
		}
		cw.Flush()
		return nil
	}
	for _, item := range list.Items {
		for _, id := range item.PhotoIDs {
			if _, e = os.Stat(filepath.Join(s.Dir, "media", id+".jpg")); e != nil {
				return e
			}
		}
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="fabricworld-works.zip"`)
	zw := zip.NewWriter(w)
	entry, e := zw.Create("works.json")
	if e != nil {
		return nil
	}
	if e = json.NewEncoder(entry).Encode(map[string]any{"version": 1, "exportedAt": now(), "works": list.Items}); e != nil {
		return nil
	}
	for _, item := range list.Items {
		for i, id := range item.PhotoIDs {
			if ctx.Err() != nil {
				return nil
			}
			entry, e = zw.CreateHeader(&zip.FileHeader{Name: "photos/" + item.ID + "/" + strconv.Itoa(i+1) + "-" + id + ".jpg", Method: zip.Store})
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

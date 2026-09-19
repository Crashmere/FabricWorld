package app

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type Problem struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

func (p *Problem) Error() string { return p.Message }
func fail(status int, code, message string) error {
	return &Problem{Status: status, Code: code, Message: message}
}
func invalid(field, message string) error {
	return &Problem{Status: 422, Code: "validation", Message: message, Field: field}
}
func ID() string {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }

var idPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)
var decimalPattern = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)?$`)

type Piece struct {
	Width     string `json:"width"`
	Length    string `json:"length"`
	Unit      string `json:"unit"`
	Count     int    `json:"count"`
	Irregular bool   `json:"irregular"`
	Note      string `json:"note"`
	WidthMM   *int64 `json:"widthMM"`
	LengthMM  *int64 `json:"lengthMM"`
}
type Media struct {
	ID        string `json:"id"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Bytes     int64  `json:"bytes"`
	CreatedAt string `json:"createdAt"`
}
type Fabric struct {
	ID           string   `json:"id"`
	Revision     int      `json:"revision"`
	Name         string   `json:"name"`
	Materials    []string `json:"materials"`
	Composition  string   `json:"composition"`
	Color        string   `json:"color"`
	Tags         []string `json:"tags"`
	Status       string   `json:"status"`
	Location     string   `json:"location"`
	PurchaseDate string   `json:"purchaseDate"`
	Shop         string   `json:"shop"`
	Price        string   `json:"price"`
	PriceCents   *int64   `json:"priceCents"`
	Notes        string   `json:"notes"`
	Pieces       []Piece  `json:"pieces"`
	PhotoIDs     []string `json:"photoIds"`
	Photos       []Media  `json:"photos"`
	CreatedAt    string   `json:"createdAt"`
	UpdatedAt    string   `json:"updatedAt"`
	DeletedAt    *string  `json:"deletedAt"`
}
type WriteInput struct {
	Fabric
	Action string `json:"action"`
}
type Change struct {
	Revision int     `json:"revision"`
	Action   string  `json:"action"`
	At       string  `json:"at"`
	Before   *Fabric `json:"before"`
	After    *Fabric `json:"after"`
}

func decimal(s string, scale, limit int64) (*int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	if !decimalPattern.MatchString(s) || len(s) > 20 {
		return nil, fmt.Errorf("请输入有效数字")
	}
	parts := strings.Split(s, ".")
	digits := len(strconv.FormatInt(scale, 10)) - 1
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	if len(fraction) > digits {
		return nil, fmt.Errorf("最多支持 %d 位小数", digits)
	}
	whole, e := strconv.ParseInt(parts[0], 10, 64)
	if e != nil || whole > limit/scale {
		return nil, fmt.Errorf("数值超出范围")
	}
	fraction += strings.Repeat("0", digits-len(fraction))
	n := int64(0)
	if fraction != "" {
		n, _ = strconv.ParseInt(fraction, 10, 64)
	}
	n += whole * scale
	if n > limit {
		return nil, fmt.Errorf("数值超出范围")
	}
	return &n, nil
}
func textField(s *string, field string, max int) error {
	*s = strings.TrimSpace(*s)
	if utf8.RuneCountInString(*s) > max {
		return invalid(field, fmt.Sprintf("最多 %d 个字符", max))
	}
	for _, c := range *s {
		if c < 32 && c != '\n' && c != '\t' {
			return invalid(field, "包含无效字符")
		}
	}
	return nil
}
func words(v []string, field string) ([]string, error) {
	if len(v) > 20 {
		return nil, invalid(field, "最多 20 项")
	}
	out := []string{}
	seen := map[string]bool{}
	for _, s := range v {
		if e := textField(&s, field, 40); e != nil {
			return nil, e
		}
		if s != "" && !seen[s] {
			out = append(out, s)
			seen[s] = true
		}
	}
	return out, nil
}
func (f *Fabric) Validate() error {
	for _, v := range []struct {
		s     *string
		field string
		max   int
	}{{&f.Name, "name", 500}, {&f.Composition, "composition", 200}, {&f.Color, "color", 40}, {&f.Location, "location", 160}, {&f.Shop, "shop", 160}, {&f.Notes, "notes", 4000}} {
		if e := textField(v.s, v.field, v.max); e != nil {
			return e
		}
	}
	var e error
	f.Materials, e = words(f.Materials, "materials")
	if e != nil {
		return e
	}
	f.Tags, e = words(f.Tags, "tags")
	if e != nil {
		return e
	}
	if f.Name == "" {
		if len(f.PhotoIDs) == 0 {
			return invalid("name", "请填写名称或添加一张照片")
		}
		f.Name = "布料 " + time.Now().Format("2006-01-02")
	}
	if f.Status != "unused" && f.Status != "using" && f.Status != "used" {
		return invalid("status", "请选择布料状态")
	}
	if f.PurchaseDate != "" {
		t, e := time.Parse("2006-01-02", f.PurchaseDate)
		if e != nil || t.Year() < 1900 || t.Year() > 2200 {
			return invalid("purchaseDate", "请输入有效购买日期")
		}
	}
	f.PriceCents, e = decimal(f.Price, 100, 100000000000)
	if e != nil {
		return invalid("price", e.Error())
	}
	if len(f.Pieces) > 50 || (f.Status == "used" && len(f.Pieces) != 0) || (f.Status != "used" && len(f.Pieces) == 0) {
		return invalid("pieces", "在库布料需要 1–50 组尺寸；已用完不能有剩余布片")
	}
	for i := range f.Pieces {
		p := &f.Pieces[i]
		scale := int64(10)
		if p.Unit == "m" {
			scale = 1000
		} else if p.Unit != "cm" {
			return invalid("pieces", "尺寸单位必须为 cm 或 m")
		}
		p.WidthMM, e = decimal(p.Width, scale, 1000000)
		if e != nil {
			return invalid("pieces", e.Error())
		}
		p.LengthMM, e = decimal(p.Length, scale, 1000000)
		if e != nil {
			return invalid("pieces", e.Error())
		}
		if p.Count < 1 || p.Count > 999 || (p.WidthMM != nil && *p.WidthMM == 0) || (p.LengthMM != nil && *p.LengthMM == 0) {
			return invalid("pieces", "已知尺寸须大于 0，片数为 1–999")
		}
		if e := textField(&p.Note, "pieces", 200); e != nil {
			return e
		}
	}
	if len(f.PhotoIDs) > 10 {
		return invalid("photos", "每条最多 10 张照片")
	}
	seen := map[string]bool{}
	for _, id := range f.PhotoIDs {
		if !idPattern.MatchString(id) || seen[id] {
			return invalid("photos", "照片无效或重复")
		}
		seen[id] = true
	}
	if f.PhotoIDs == nil {
		f.PhotoIDs = []string{}
	}
	if f.Pieces == nil {
		f.Pieces = []Piece{}
	}
	return nil
}

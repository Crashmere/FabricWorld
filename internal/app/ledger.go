package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
)

var ledgerIDPattern = regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`)

func (s *Store) importLedger(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		TransactionID string `json:"transactionId"`
		Name          string `json:"name"`
		PurchaseDate  string `json:"purchaseDate"`
		Price         string `json:"price"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	dec.DisallowUnknownFields()
	if dec.Decode(&in) != nil {
		return fail(400, "json", "同步数据格式无效")
	}
	var extra any
	if dec.Decode(&extra) != io.EOF || !ledgerIDPattern.MatchString(in.TransactionID) {
		return fail(400, "transaction_id", "交易编号无效")
	}
	if in.PurchaseDate == "" || in.Price == "" {
		return fail(422, "purchase", "购买日期与金额不能为空")
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = "未命名布料"
	}
	f := Fabric{Name: name, PurchaseDate: in.PurchaseDate, Price: in.Price, Status: "unused", Pieces: []Piece{{Unit: "cm", Count: 1}}, PhotoIDs: []string{}}
	// A transaction always refers to its first import, even after edits or
	// operation cleanup. Retries must never overwrite an enriched fabric.
	fp := "ledger:" + in.TransactionID
	sum := sha256.Sum256([]byte(fp))
	result, err := s.Write(r.Context(), "", hex.EncodeToString(sum[:16]), fp, "save", f)
	if err != nil {
		return err
	}
	var imported Fabric
	if err = json.Unmarshal(result, &imported); err != nil {
		return err
	}
	current, err := s.Get(r.Context(), imported.ID)
	if err != nil {
		return err
	}
	if current.DeletedAt != nil {
		return fail(409, "deleted", "同步过的布料已在回收站，请到 FabricWorld 核对或恢复")
	}
	jsonResponse(w, 200, current)
	return nil
}

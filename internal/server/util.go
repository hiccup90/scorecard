package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/hiccup90/scorecard/internal/domain"
)

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func methodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, "方法不允许")
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst interface{}) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "JSON 格式错误")
		return false
	}
	return true
}

func parseID(w http.ResponseWriter, value string) (int64, bool) {
	value = strings.TrimSpace(value)
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "无效 ID")
		return 0, false
	}
	return id, true
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func validScoreMode(v string) bool {
	return domain.ValidScoreMode(v)
}

func nullableString(v string) interface{} {
	if v == "" {
		return nil
	}
	return v
}

func nullableInt(v int) interface{} {
	if v == 0 {
		return nil
	}
	return v
}

func min(a, b int) int { return domain.Min(a, b) }
func max(a, b int) int { return domain.Max(a, b) }

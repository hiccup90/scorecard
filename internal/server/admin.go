package server

import (
	"net/http"
	"strings"
)

func (s *Server) handleAdminActivities(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rows, err := s.db.Query(`SELECT id,label,base_points,score_mode,icon,color,category,sort_order,enabled FROM activities ORDER BY category, sort_order, id`)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer rows.Close()
		items, err := scanActivities(rows)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, items)
	case http.MethodPost:
		var body Activity
		if !decodeJSON(w, r, &body) {
			return
		}
		if strings.TrimSpace(body.Label) == "" {
			writeError(w, http.StatusBadRequest, "名称不能为空")
			return
		}
		if !validScoreMode(body.ScoreMode) {
			body.ScoreMode = "default"
		}
		if body.Icon == "" {
			body.Icon = "star"
		}
		if body.Color == "" {
			body.Color = "#3B82F6"
		}
		if body.Category == "" {
			body.Category = "生活"
		}
		res, err := s.db.Exec(`INSERT INTO activities (label,base_points,score_mode,icon,color,category,sort_order,enabled) VALUES (?,?,?,?,?,?,?,1)`, body.Label, body.BasePoints, body.ScoreMode, body.Icon, body.Color, body.Category, body.SortOrder)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		id, _ := res.LastInsertId()
		writeJSON(w, http.StatusOK, map[string]int64{"id": id})
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleAdminActivityByID(w http.ResponseWriter, r *http.Request) {
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/activities/"), "/")
	parts := strings.Split(rest, "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusNotFound, "接口不存在")
		return
	}
	id, ok := parseID(w, parts[0])
	if !ok {
		return
	}

	// POST .../restore
	if len(parts) == 2 && parts[1] == "restore" {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		res, err := s.db.Exec(`UPDATE activities SET enabled=1 WHERE id=?`, id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			writeError(w, http.StatusNotFound, "打卡项不存在")
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	if len(parts) != 1 {
		writeError(w, http.StatusNotFound, "接口不存在")
		return
	}

	switch r.Method {
	case http.MethodPut:
		var body Activity
		if !decodeJSON(w, r, &body) {
			return
		}
		if strings.TrimSpace(body.Label) == "" {
			writeError(w, http.StatusBadRequest, "名称不能为空")
			return
		}
		if !validScoreMode(body.ScoreMode) {
			body.ScoreMode = "default"
		}
		_, err := s.db.Exec(`UPDATE activities SET label=?,base_points=?,score_mode=?,icon=?,color=?,category=?,sort_order=?,enabled=? WHERE id=?`, body.Label, body.BasePoints, body.ScoreMode, body.Icon, body.Color, body.Category, body.SortOrder, boolInt(body.Enabled), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case http.MethodDelete:
		_, err := s.db.Exec(`UPDATE activities SET enabled=0 WHERE id=?`, id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleAdjustment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	sess := sessionFrom(r)
	var body struct {
		Change int    `json:"change"`
		Reason string `json:"reason"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	userID := int64(1)
	parentID := sess.UserID
	if parentID == 0 {
		parentID = 2
	}
	if body.Change == 0 {
		writeError(w, http.StatusBadRequest, "调整分值不能为 0")
		return
	}
	if body.Reason == "" {
		body.Reason = "家长调整"
	}
	_, err := s.db.Exec(`INSERT INTO point_transactions (user_id,change,reason,source_type,created_by) VALUES (?,?,?,?,?)`, userID, body.Change, body.Reason, "adjustment", parentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	auditDB(s.db.DB, parentID, "adjustment", "user", userID, body.Reason)
	summary, _ := s.summary(userID)
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "summary": summary})
}

package server

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

func (s *Server) handleCheckins(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r)
	switch r.Method {
	case http.MethodGet:
		userID := int64(1)
		if sess.Role == "child" {
			userID = sess.UserID
		}
		rows, err := s.db.Query(checkinSelect()+` WHERE c.user_id=? ORDER BY c.submitted_at DESC LIMIT 100`, userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer rows.Close()
		items, err := scanCheckins(rows)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, items)
	case http.MethodPost:
		if sess.Role != "child" && sess.Role != "parent" {
			writeError(w, http.StatusUnauthorized, "需要登录")
			return
		}
		var body struct {
			ActivityID   int64  `json:"activity_id"`
			ActivityDate string `json:"activity_date"`
			Note         string `json:"note"`
		}
		if !decodeJSON(w, r, &body) {
			return
		}
		userID := int64(1)
		if sess.Role == "child" {
			userID = sess.UserID
		}
		today := s.db.Today()
		if body.ActivityDate == "" {
			body.ActivityDate = today
		}
		if _, err := time.Parse("2006-01-02", body.ActivityDate); err != nil {
			writeError(w, http.StatusBadRequest, "日期格式无效")
			return
		}
		if body.ActivityDate > today {
			writeError(w, http.StatusBadRequest, "不能选择未来日期")
			return
		}
		if s.cfg.MakeupDays > 0 {
			minDate := s.db.Now().AddDate(0, 0, -s.cfg.MakeupDays).Format("2006-01-02")
			if body.ActivityDate < minDate {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("补签不能超过 %d 天", s.cfg.MakeupDays))
				return
			}
		}
		activity, err := s.activity(body.ActivityID)
		if err != nil {
			writeError(w, http.StatusNotFound, "打卡项不存在")
			return
		}
		source := "normal"
		if body.ActivityDate != today {
			source = "makeup"
		}
		res, err := s.db.Exec(`INSERT INTO checkins (user_id,activity_id,activity_date,source,base_points,score_mode,review_note) VALUES (?,?,?,?,?,?,?)`, userID, body.ActivityID, body.ActivityDate, source, activity.BasePoints, activity.ScoreMode, body.Note)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		id, _ := res.LastInsertId()
		auditDB(s.db.DB, sess.UserID, "checkin.create", "checkin", id, body.ActivityDate)
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "id": id})
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleAdminCheckins(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	rows, err := s.db.Query(checkinSelect() + ` ORDER BY CASE c.status WHEN 'pending' THEN 0 WHEN 'approved' THEN 1 ELSE 2 END, c.submitted_at DESC LIMIT 200`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	items, err := scanCheckins(rows)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleAdminCheckinAction(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/checkins/"), "/")
	if len(parts) != 2 {
		writeError(w, http.StatusNotFound, "接口不存在")
		return
	}
	id, ok := parseID(w, parts[0])
	if !ok {
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	switch parts[1] {
	case "approve":
		s.approveCheckin(w, r, id)
	case "reject":
		s.rejectCheckin(w, r, id)
	case "reverse":
		s.reverseCheckin(w, r, id)
	default:
		writeError(w, http.StatusNotFound, "接口不存在")
	}
}

func (s *Server) approveCheckin(w http.ResponseWriter, r *http.Request, id int64) {
	sess := sessionFrom(r)
	var body struct {
		ReviewLevel     string `json:"review_level"`
		ReviewMinutes   int    `json:"review_minutes"`
		CountsForStreak bool   `json:"counts_for_streak"`
		Note            string `json:"note"`
	}
	body.CountsForStreak = true
	if !decodeJSON(w, r, &body) {
		return
	}
	parentID := sess.UserID
	if parentID == 0 {
		parentID = 2
	}

	tx, err := s.db.Begin()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback()

	c, err := checkinForUpdate(tx, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "打卡不存在")
		return
	}
	if c.Status != "pending" {
		writeError(w, http.StatusBadRequest, "只能审核待处理打卡")
		return
	}
	awarded, suffix, reviewLevel, reviewMinutes, err := award(c.ScoreMode, c.BasePoints, body.ReviewLevel, body.ReviewMinutes)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Mark approved first so streak recalc includes this checkin.
	_, err = tx.Exec(`UPDATE checkins SET status='approved',review_level=?,review_minutes=?,awarded_points=?,streak_bonus=0,counts_for_streak=?,review_note=?,reviewed_at=CURRENT_TIMESTAMP,reviewed_by=? WHERE id=?`,
		nullableString(reviewLevel), nullableInt(reviewMinutes), awarded, boolInt(body.CountsForStreak), body.Note, parentID, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	bonus := 0
	if body.CountsForStreak {
		bonus, err = recalcStreakTx(tx, c.UserID, c.ActivityID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if bonus > 0 {
			if _, err := tx.Exec(`UPDATE checkins SET streak_bonus=? WHERE id=?`, bonus, id); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
	}

	total := awarded + bonus
	reason := c.ActivityLabel + "（" + suffix
	if bonus > 0 {
		reason += fmt.Sprintf("，连续奖励+%d", bonus)
	}
	reason += "）"
	if _, err := tx.Exec(`INSERT INTO point_transactions (user_id,change,reason,source_type,source_id,created_by) VALUES (?,?,?,?,?,?)`, c.UserID, total, reason, "checkin", id, parentID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := audit(tx, parentID, "checkin.approve", "checkin", id, reason); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	summary, _ := s.summary(c.UserID)
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "awarded_points": total, "summary": summary})
}

func (s *Server) rejectCheckin(w http.ResponseWriter, r *http.Request, id int64) {
	sess := sessionFrom(r)
	var body struct {
		Reason string `json:"reason"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	parentID := sess.UserID
	if parentID == 0 {
		parentID = 2
	}
	if body.Reason == "" {
		body.Reason = "家长驳回"
	}
	res, err := s.db.Exec(`UPDATE checkins SET status='rejected',review_note=?,reviewed_at=CURRENT_TIMESTAMP,reviewed_by=? WHERE id=? AND status='pending'`, body.Reason, parentID, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeError(w, http.StatusBadRequest, "只能驳回待审核打卡")
		return
	}
	auditDB(s.db.DB, parentID, "checkin.reject", "checkin", id, body.Reason)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) reverseCheckin(w http.ResponseWriter, r *http.Request, id int64) {
	sess := sessionFrom(r)
	var body struct {
		Reason string `json:"reason"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	parentID := sess.UserID
	if parentID == 0 {
		parentID = 2
	}
	if body.Reason == "" {
		body.Reason = "家长撤回"
	}
	tx, err := s.db.Begin()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback()
	c, err := checkinForUpdate(tx, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "打卡不存在")
		return
	}
	if c.Status != "approved" {
		writeError(w, http.StatusBadRequest, "只能撤回已通过打卡")
		return
	}
	change := -(c.AwardedPoints + c.StreakBonus)
	if _, err = tx.Exec(`INSERT INTO point_transactions (user_id,change,reason,source_type,source_id,created_by) VALUES (?,?,?,?,?,?)`, c.UserID, change, "撤回打卡："+body.Reason, "checkin_reversal", id, parentID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err = tx.Exec(`UPDATE checkins SET status='reversed',reversed_at=CURRENT_TIMESTAMP,reversed_by=?,reverse_reason=? WHERE id=?`, parentID, body.Reason, id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := recalcStreakTx(tx, c.UserID, c.ActivityID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := audit(tx, parentID, "checkin.reverse", "checkin", id, body.Reason); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

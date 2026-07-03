package server

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hiccup90/scorecard/internal/config"
	"github.com/hiccup90/scorecard/internal/database"
)

type Server struct {
	cfg    config.Config
	db     *database.DB
	mux    *http.ServeMux
	tokens map[string]time.Time
	mu     sync.Mutex
}

func New(cfg config.Config, db *database.DB) *Server {
	s := &Server{cfg: cfg, db: db, mux: http.NewServeMux(), tokens: map[string]time.Time{}}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.recover(s.log(s.mux))
}

func (s *Server) routes() {
	s.mux.HandleFunc("/api/v1/auth/parent/login", s.handleParentLogin)
	s.mux.HandleFunc("/api/v1/auth/child/login", s.handleChildLogin)
	s.mux.HandleFunc("/api/v1/auth/verify", s.requireParent(s.handleVerify))
	s.mux.HandleFunc("/api/v1/children/1/summary", s.handleChildSummary)
	s.mux.HandleFunc("/api/v1/activities", s.handleActivities)
	s.mux.HandleFunc("/api/v1/checkins", s.handleCheckins)
	s.mux.HandleFunc("/api/v1/rewards", s.handleRewards)
	s.mux.HandleFunc("/api/v1/redemptions", s.handleRedemptions)
	s.mux.HandleFunc("/api/v1/transactions", s.handleTransactions)
	s.mux.HandleFunc("/api/v1/admin/checkins", s.requireParent(s.handleAdminCheckins))
	s.mux.HandleFunc("/api/v1/admin/checkins/", s.requireParent(s.handleAdminCheckinAction))
	s.mux.HandleFunc("/api/v1/admin/activities", s.requireParent(s.handleAdminActivities))
	s.mux.HandleFunc("/api/v1/admin/activities/", s.requireParent(s.handleAdminActivityByID))
	s.mux.HandleFunc("/api/v1/admin/rewards", s.requireParent(s.handleAdminRewards))
	s.mux.HandleFunc("/api/v1/admin/rewards/", s.requireParent(s.handleAdminRewardByID))
	s.mux.HandleFunc("/api/v1/admin/redemptions", s.requireParent(s.handleAdminRedemptions))
	s.mux.HandleFunc("/api/v1/admin/redemptions/", s.requireParent(s.handleAdminRedemptionAction))
	s.mux.HandleFunc("/api/v1/admin/adjustments", s.requireParent(s.handleAdjustment))
	s.mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { writeError(w, http.StatusNotFound, "接口不存在") })
	s.mux.HandleFunc("/", s.handleStatic)
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	path := filepath.Join(s.cfg.StaticDir, filepath.Clean(r.URL.Path))
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		http.ServeFile(w, r, path)
		return
	}
	http.ServeFile(w, r, filepath.Join(s.cfg.StaticDir, "index.html"))
}

func (s *Server) handleParentLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { methodNotAllowed(w); return }
	var body struct{ PIN string `json:"pin"` }
	if !decodeJSON(w, r, &body) { return }
	if body.PIN != s.cfg.AdminPIN { writeError(w, http.StatusForbidden, "PIN 码错误"); return }
	token, err := newToken()
	if err != nil { writeError(w, http.StatusInternalServerError, "生成 token 失败"); return }
	s.mu.Lock(); s.tokens[token] = time.Now().Add(24 * time.Hour); s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "token": token})
}

func (s *Server) handleChildLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { methodNotAllowed(w); return }
	var body struct{ PIN string `json:"pin"` }
	if !decodeJSON(w, r, &body) { return }
	if body.PIN != s.cfg.ChildPIN { writeError(w, http.StatusForbidden, "PIN 码错误"); return }
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

func (s *Server) handleVerify(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) requireParent(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Auth-Token")
		s.mu.Lock()
		expires, ok := s.tokens[token]
		if ok && time.Now().After(expires) { delete(s.tokens, token); ok = false }
		s.mu.Unlock()
		if !ok { writeError(w, http.StatusUnauthorized, "需要登录"); return }
		next(w, r)
	}
}

func (s *Server) handleChildSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet { methodNotAllowed(w); return }
	summary, err := s.summary(1)
	if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) handleActivities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet { methodNotAllowed(w); return }
	rows, err := s.db.Query(`SELECT id,label,base_points,score_mode,icon,color,category,sort_order,enabled FROM activities WHERE enabled=1 ORDER BY category, sort_order, id`)
	if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	defer rows.Close()
	items, err := scanActivities(rows)
	if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleAdminActivities(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rows, err := s.db.Query(`SELECT id,label,base_points,score_mode,icon,color,category,sort_order,enabled FROM activities ORDER BY category, sort_order, id`)
		if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
		defer rows.Close()
		items, err := scanActivities(rows)
		if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
		writeJSON(w, http.StatusOK, items)
	case http.MethodPost:
		var body Activity
		if !decodeJSON(w, r, &body) { return }
		if strings.TrimSpace(body.Label) == "" { writeError(w, http.StatusBadRequest, "名称不能为空"); return }
		if !validScoreMode(body.ScoreMode) { body.ScoreMode = "default" }
		if body.Icon == "" { body.Icon = "star" }
		if body.Color == "" { body.Color = "#3B82F6" }
		if body.Category == "" { body.Category = "生活" }
		res, err := s.db.Exec(`INSERT INTO activities (label,base_points,score_mode,icon,color,category,sort_order,enabled) VALUES (?,?,?,?,?,?,?,1)`, body.Label, body.BasePoints, body.ScoreMode, body.Icon, body.Color, body.Category, body.SortOrder)
		if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
		id, _ := res.LastInsertId()
		writeJSON(w, http.StatusOK, map[string]int64{"id": id})
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleAdminActivityByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, strings.TrimPrefix(r.URL.Path, "/api/v1/admin/activities/"))
	if !ok { return }
	switch r.Method {
	case http.MethodPut:
		var body Activity
		if !decodeJSON(w, r, &body) { return }
		if strings.TrimSpace(body.Label) == "" { writeError(w, http.StatusBadRequest, "名称不能为空"); return }
		if !validScoreMode(body.ScoreMode) { body.ScoreMode = "default" }
		_, err := s.db.Exec(`UPDATE activities SET label=?,base_points=?,score_mode=?,icon=?,color=?,category=?,sort_order=?,enabled=? WHERE id=?`, body.Label, body.BasePoints, body.ScoreMode, body.Icon, body.Color, body.Category, body.SortOrder, boolInt(body.Enabled), id)
		if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case http.MethodDelete:
		_, err := s.db.Exec(`UPDATE activities SET enabled=0 WHERE id=?`, id)
		if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleCheckins(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rows, err := s.db.Query(checkinSelect()+` WHERE c.user_id=1 ORDER BY c.submitted_at DESC LIMIT 100`)
		if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
		defer rows.Close()
		items, err := scanCheckins(rows)
		if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
		writeJSON(w, http.StatusOK, items)
	case http.MethodPost:
		var body struct {
			UserID       int64  `json:"user_id"`
			ActivityID   int64  `json:"activity_id"`
			ActivityDate string `json:"activity_date"`
			Note         string `json:"note"`
		}
		if !decodeJSON(w, r, &body) { return }
		if body.UserID == 0 { body.UserID = 1 }
		if body.ActivityDate == "" { body.ActivityDate = database.Today() }
		activity, err := s.activity(body.ActivityID)
		if err != nil { writeError(w, http.StatusNotFound, "打卡项不存在"); return }
		source := "normal"
		if body.ActivityDate != database.Today() { source = "makeup" }
		res, err := s.db.Exec(`INSERT INTO checkins (user_id,activity_id,activity_date,source,base_points,score_mode,review_note) VALUES (?,?,?,?,?,?,?)`, body.UserID, body.ActivityID, body.ActivityDate, source, activity.BasePoints, activity.ScoreMode, body.Note)
		if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
		id, _ := res.LastInsertId()
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "id": id})
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleAdminCheckins(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet { methodNotAllowed(w); return }
	rows, err := s.db.Query(checkinSelect()+` ORDER BY CASE c.status WHEN 'pending' THEN 0 WHEN 'approved' THEN 1 ELSE 2 END, c.submitted_at DESC LIMIT 200`)
	if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	defer rows.Close()
	items, err := scanCheckins(rows)
	if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleAdminCheckinAction(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/checkins/"), "/")
	if len(parts) != 2 { writeError(w, http.StatusNotFound, "接口不存在"); return }
	id, ok := parseID(w, parts[0]); if !ok { return }
	if r.Method != http.MethodPost { methodNotAllowed(w); return }
	switch parts[1] {
	case "approve": s.approveCheckin(w, r, id)
	case "reject": s.rejectCheckin(w, r, id)
	case "reverse": s.reverseCheckin(w, r, id)
	default: writeError(w, http.StatusNotFound, "接口不存在")
	}
}

func (s *Server) approveCheckin(w http.ResponseWriter, r *http.Request, id int64) {
	var body struct {
		ParentID        int64  `json:"parent_id"`
		ReviewLevel     string `json:"review_level"`
		ReviewMinutes   int    `json:"review_minutes"`
		CountsForStreak bool   `json:"counts_for_streak"`
		Note            string `json:"note"`
	}
	body.CountsForStreak = true
	if !decodeJSON(w, r, &body) { return }
	if body.ParentID == 0 { body.ParentID = 2 }

	tx, err := s.db.Begin()
	if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	defer tx.Rollback()

	c, err := checkinForUpdate(tx, id)
	if err != nil { writeError(w, http.StatusNotFound, "打卡不存在"); return }
	if c.Status != "pending" { writeError(w, http.StatusBadRequest, "只能审核待处理打卡"); return }
	awarded, suffix, reviewLevel, reviewMinutes, err := award(c.ScoreMode, c.BasePoints, body.ReviewLevel, body.ReviewMinutes)
	if err != nil { writeError(w, http.StatusBadRequest, err.Error()); return }
	bonus := 0
	if body.CountsForStreak {
		bonus, err = recalcStreakTx(tx, c.UserID, c.ActivityID)
		if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	}
	total := awarded + bonus
	reason := c.ActivityLabel + "（" + suffix
	if bonus > 0 { reason += fmt.Sprintf("，连续奖励+%d", bonus) }
	reason += "）"
	res, err := tx.Exec(`INSERT INTO point_transactions (user_id,change,reason,source_type,source_id,created_by) VALUES (?,?,?,?,?,?)`, c.UserID, total, reason, "checkin", id, body.ParentID)
	if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	_, err = res.LastInsertId()
	if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	_, err = tx.Exec(`UPDATE checkins SET status='approved',review_level=?,review_minutes=?,awarded_points=?,streak_bonus=?,counts_for_streak=?,review_note=?,reviewed_at=CURRENT_TIMESTAMP,reviewed_by=? WHERE id=?`, nullableString(reviewLevel), nullableInt(reviewMinutes), awarded, bonus, boolInt(body.CountsForStreak), body.Note, body.ParentID, id)
	if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	if err := tx.Commit(); err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	summary, _ := s.summary(c.UserID)
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "awarded_points": total, "summary": summary})
}

func (s *Server) rejectCheckin(w http.ResponseWriter, r *http.Request, id int64) {
	var body struct { ParentID int64 `json:"parent_id"`; Reason string `json:"reason"` }
	if !decodeJSON(w, r, &body) { return }
	if body.ParentID == 0 { body.ParentID = 2 }
	if body.Reason == "" { body.Reason = "家长驳回" }
	res, err := s.db.Exec(`UPDATE checkins SET status='rejected',review_note=?,reviewed_at=CURRENT_TIMESTAMP,reviewed_by=? WHERE id=? AND status='pending'`, body.Reason, body.ParentID, id)
	if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	if n, _ := res.RowsAffected(); n == 0 { writeError(w, http.StatusBadRequest, "只能驳回待审核打卡"); return }
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) reverseCheckin(w http.ResponseWriter, r *http.Request, id int64) {
	var body struct { ParentID int64 `json:"parent_id"`; Reason string `json:"reason"` }
	if !decodeJSON(w, r, &body) { return }
	if body.ParentID == 0 { body.ParentID = 2 }
	if body.Reason == "" { body.Reason = "家长撤回" }
	tx, err := s.db.Begin()
	if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	defer tx.Rollback()
	c, err := checkinForUpdate(tx, id)
	if err != nil { writeError(w, http.StatusNotFound, "打卡不存在"); return }
	if c.Status != "approved" { writeError(w, http.StatusBadRequest, "只能撤回已通过打卡"); return }
	change := -(c.AwardedPoints + c.StreakBonus)
	_, err = tx.Exec(`INSERT INTO point_transactions (user_id,change,reason,source_type,source_id,created_by) VALUES (?,?,?,?,?,?)`, c.UserID, change, "撤回打卡："+body.Reason, "checkin_reversal", id, body.ParentID)
	if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	_, err = tx.Exec(`UPDATE checkins SET status='reversed',reversed_at=CURRENT_TIMESTAMP,reversed_by=?,reverse_reason=? WHERE id=?`, body.ParentID, body.Reason, id)
	if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	if _, err := recalcStreakTx(tx, c.UserID, c.ActivityID); err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	if err := tx.Commit(); err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleTransactions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet { methodNotAllowed(w); return }
	rows, err := s.db.Query(`SELECT id,user_id,change,reason,source_type,source_id,created_at FROM point_transactions WHERE user_id=1 ORDER BY created_at DESC LIMIT 100`)
	if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	defer rows.Close()
	var out []Transaction
	for rows.Next() {
		var t Transaction; var sid sql.NullInt64
		if err := rows.Scan(&t.ID,&t.UserID,&t.Change,&t.Reason,&t.SourceType,&sid,&t.CreatedAt); err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
		if sid.Valid { t.SourceID = &sid.Int64 }
		out = append(out, t)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleAdjustment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { methodNotAllowed(w); return }
	var body struct { UserID int64 `json:"user_id"`; Change int `json:"change"`; Reason string `json:"reason"`; ParentID int64 `json:"parent_id"` }
	if !decodeJSON(w, r, &body) { return }
	if body.UserID == 0 { body.UserID = 1 }
	if body.ParentID == 0 { body.ParentID = 2 }
	if body.Change == 0 { writeError(w, http.StatusBadRequest, "调整分值不能为 0"); return }
	if body.Reason == "" { body.Reason = "家长调整" }
	_, err := s.db.Exec(`INSERT INTO point_transactions (user_id,change,reason,source_type,created_by) VALUES (?,?,?,?,?)`, body.UserID, body.Change, body.Reason, "adjustment", body.ParentID)
	if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	summary, _ := s.summary(body.UserID)
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "summary": summary})
}

func (s *Server) handleRewards(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet { methodNotAllowed(w); return }
	rows, err := s.db.Query(`SELECT id,name,cost,description,stock,auto_approve,enabled FROM rewards WHERE enabled=1 ORDER BY cost,id`)
	if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	defer rows.Close()
	rewards, err := scanRewards(rows)
	if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	writeJSON(w, http.StatusOK, rewards)
}

func (s *Server) handleAdminRewards(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rows, err := s.db.Query(`SELECT id,name,cost,description,stock,auto_approve,enabled FROM rewards ORDER BY cost,id`)
		if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
		defer rows.Close()
		rewards, err := scanRewards(rows)
		if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
		writeJSON(w, http.StatusOK, rewards)
	case http.MethodPost:
		var body Reward
		if !decodeJSON(w, r, &body) { return }
		if strings.TrimSpace(body.Name) == "" || body.Cost <= 0 { writeError(w, http.StatusBadRequest, "奖励名称和积分必须有效"); return }
		res, err := s.db.Exec(`INSERT INTO rewards (name,cost,description,stock,auto_approve,enabled) VALUES (?,?,?,?,?,1)`, body.Name, body.Cost, body.Description, body.Stock, boolInt(body.AutoApprove))
		if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
		id, _ := res.LastInsertId(); writeJSON(w, http.StatusOK, map[string]int64{"id": id})
	default: methodNotAllowed(w)
	}
}

func (s *Server) handleAdminRewardByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, strings.TrimPrefix(r.URL.Path, "/api/v1/admin/rewards/")); if !ok { return }
	switch r.Method {
	case http.MethodPut:
		var body Reward
		if !decodeJSON(w, r, &body) { return }
		_, err := s.db.Exec(`UPDATE rewards SET name=?,cost=?,description=?,stock=?,auto_approve=?,enabled=? WHERE id=?`, body.Name, body.Cost, body.Description, body.Stock, boolInt(body.AutoApprove), boolInt(body.Enabled), id)
		if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case http.MethodDelete:
		_, err := s.db.Exec(`UPDATE rewards SET enabled=0 WHERE id=?`, id)
		if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default: methodNotAllowed(w)
	}
}

func (s *Server) handleRedemptions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rows, err := s.db.Query(redemptionSelect()+` WHERE rd.user_id=1 ORDER BY rd.created_at DESC LIMIT 100`)
		if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
		defer rows.Close(); out, err := scanRedemptions(rows); if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
		writeJSON(w, http.StatusOK, out)
	case http.MethodPost:
		var body struct { UserID int64 `json:"user_id"`; RewardID int64 `json:"reward_id"` }
		if !decodeJSON(w, r, &body) { return }
		if body.UserID == 0 { body.UserID = 1 }
		err := s.createRedemption(body.UserID, body.RewardID)
		if err != nil { writeError(w, http.StatusBadRequest, err.Error()); return }
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default: methodNotAllowed(w)
	}
}

func (s *Server) handleAdminRedemptions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet { methodNotAllowed(w); return }
	rows, err := s.db.Query(redemptionSelect()+` ORDER BY CASE rd.status WHEN 'pending' THEN 0 ELSE 1 END, rd.created_at DESC LIMIT 200`)
	if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	defer rows.Close(); out, err := scanRedemptions(rows); if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleAdminRedemptionAction(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/redemptions/"), "/")
	if len(parts) != 2 { writeError(w, http.StatusNotFound, "接口不存在"); return }
	id, ok := parseID(w, parts[0]); if !ok { return }
	if r.Method != http.MethodPost { methodNotAllowed(w); return }
	if parts[1] == "approve" { s.approveRedemption(w, r, id); return }
	if parts[1] == "reject" { s.rejectRedemption(w, r, id); return }
	writeError(w, http.StatusNotFound, "接口不存在")
}

func (s *Server) createRedemption(userID, rewardID int64) error {
	tx, err := s.db.Begin(); if err != nil { return err }
	defer tx.Rollback()
	var name string; var cost, stock, auto int
	if err := tx.QueryRow(`SELECT name,cost,stock,auto_approve FROM rewards WHERE id=? AND enabled=1`, rewardID).Scan(&name,&cost,&stock,&auto); err != nil { return errors.New("奖励不存在") }
	if stock == 0 { return errors.New("奖励库存不足") }
	balance, err := balanceTx(tx, userID); if err != nil { return err }
	if balance < cost { return errors.New("积分不足") }
	status := "pending"
	if auto == 1 { status = "fulfilled" }
	res, err := tx.Exec(`INSERT INTO redemptions (user_id,reward_id,cost_at_time,status) VALUES (?,?,?,?)`, userID, rewardID, cost, status)
	if err != nil { return err }
	id, _ := res.LastInsertId()
	if auto == 1 {
		if _, err := tx.Exec(`INSERT INTO point_transactions (user_id,change,reason,source_type,source_id,created_by) VALUES (?,?,?,?,?,?)`, userID, -cost, "兑换「"+name+"」", "redemption", id, userID); err != nil { return err }
		if stock > 0 { if _, err := tx.Exec(`UPDATE rewards SET stock=stock-1 WHERE id=?`, rewardID); err != nil { return err } }
	}
	return tx.Commit()
}

func (s *Server) approveRedemption(w http.ResponseWriter, r *http.Request, id int64) {
	var body struct { ParentID int64 `json:"parent_id"`; Note string `json:"note"` }
	if !decodeJSON(w, r, &body) { return }
	if body.ParentID == 0 { body.ParentID = 2 }
	tx, err := s.db.Begin(); if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	defer tx.Rollback()
	var userID, rewardID int64; var cost int; var status, name string; var stock int
	if err := tx.QueryRow(`SELECT rd.user_id,rd.reward_id,rd.cost_at_time,rd.status,r.name,r.stock FROM redemptions rd JOIN rewards r ON r.id=rd.reward_id WHERE rd.id=?`, id).Scan(&userID,&rewardID,&cost,&status,&name,&stock); err != nil { writeError(w, http.StatusNotFound, "兑换不存在"); return }
	if status != "pending" { writeError(w, http.StatusBadRequest, "只能审批待处理兑换"); return }
	if stock == 0 { writeError(w, http.StatusBadRequest, "奖励库存不足"); return }
	balance, _ := balanceTx(tx, userID); if balance < cost { writeError(w, http.StatusBadRequest, "积分不足"); return }
	if _, err := tx.Exec(`INSERT INTO point_transactions (user_id,change,reason,source_type,source_id,created_by) VALUES (?,?,?,?,?,?)`, userID, -cost, "兑换「"+name+"」", "redemption", id, body.ParentID); err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	if _, err := tx.Exec(`UPDATE redemptions SET status='fulfilled',reviewed_at=CURRENT_TIMESTAMP,reviewed_by=?,review_note=? WHERE id=?`, body.ParentID, body.Note, id); err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	if stock > 0 { if _, err := tx.Exec(`UPDATE rewards SET stock=stock-1 WHERE id=?`, rewardID); err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return } }
	if err := tx.Commit(); err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) rejectRedemption(w http.ResponseWriter, r *http.Request, id int64) {
	var body struct { ParentID int64 `json:"parent_id"`; Note string `json:"note"` }
	if !decodeJSON(w, r, &body) { return }
	if body.ParentID == 0 { body.ParentID = 2 }
	res, err := s.db.Exec(`UPDATE redemptions SET status='rejected',reviewed_at=CURRENT_TIMESTAMP,reviewed_by=?,review_note=? WHERE id=? AND status='pending'`, body.ParentID, body.Note, id)
	if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	if n, _ := res.RowsAffected(); n == 0 { writeError(w, http.StatusBadRequest, "只能驳回待处理兑换"); return }
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) summary(userID int64) (Summary, error) {
	var out Summary
	if err := s.db.QueryRow(`SELECT COALESCE(SUM(change),0) FROM point_transactions WHERE user_id=?`, userID).Scan(&out.Balance); err != nil { return out, err }
	if err := s.db.QueryRow(`SELECT COALESCE(SUM(change),0) FROM point_transactions WHERE user_id=? AND date(created_at)=date('now','localtime')`, userID).Scan(&out.TodayTotal); err != nil { return out, err }
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM checkins WHERE user_id=? AND status='pending'`, userID).Scan(&out.PendingCount); err != nil { return out, err }
	if err := s.db.QueryRow(`SELECT COALESCE(MAX(streak_days),0) FROM streaks WHERE user_id=?`, userID).Scan(&out.MaxStreakDays); err != nil { return out, err }
	return out, nil
}

func (s *Server) activity(id int64) (Activity, error) {
	var a Activity; var enabled int
	err := s.db.QueryRow(`SELECT id,label,base_points,score_mode,icon,color,category,sort_order,enabled FROM activities WHERE id=? AND enabled=1`, id).Scan(&a.ID,&a.Label,&a.BasePoints,&a.ScoreMode,&a.Icon,&a.Color,&a.Category,&a.SortOrder,&enabled)
	a.Enabled = enabled == 1
	return a, err
}

func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil { return "", err }
	return hex.EncodeToString(b), nil
}

func (s *Server) log(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now(); next.ServeHTTP(w, r); log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

func (s *Server) recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() { if err := recover(); err != nil { writeError(w, http.StatusInternalServerError, "服务内部错误") } }()
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) { w.Header().Set("Content-Type", "application/json; charset=utf-8"); w.WriteHeader(status); json.NewEncoder(w).Encode(v) }
func writeError(w http.ResponseWriter, status int, msg string) { writeJSON(w, status, map[string]string{"error": msg}) }
func methodNotAllowed(w http.ResponseWriter) { writeError(w, http.StatusMethodNotAllowed, "方法不允许") }
func decodeJSON(w http.ResponseWriter, r *http.Request, dst interface{}) bool { if err := json.NewDecoder(r.Body).Decode(dst); err != nil { writeError(w, http.StatusBadRequest, "JSON 格式错误"); return false }; return true }
func parseID(w http.ResponseWriter, value string) (int64, bool) { id, err := strconv.ParseInt(value, 10, 64); if err != nil || id <= 0 { writeError(w, http.StatusBadRequest, "无效 ID"); return 0, false }; return id, true }
func boolInt(v bool) int { if v { return 1 }; return 0 }
func validScoreMode(v string) bool { return v == "default" || v == "quality" || v == "duration" }
func nullableString(v string) interface{} { if v == "" { return nil }; return v }
func nullableInt(v int) interface{} { if v == 0 { return nil }; return v }

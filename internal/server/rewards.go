package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

func (s *Server) handleRewards(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	rows, err := s.db.Query(`SELECT id,name,cost,description,stock,auto_approve,enabled FROM rewards WHERE enabled=1 ORDER BY cost,id`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	rewards, err := scanRewards(rows)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rewards)
}

func (s *Server) handleAdminRewards(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rows, err := s.db.Query(`SELECT id,name,cost,description,stock,auto_approve,enabled FROM rewards ORDER BY cost,id`)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer rows.Close()
		rewards, err := scanRewards(rows)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, rewards)
	case http.MethodPost:
		var body Reward
		if !decodeJSON(w, r, &body) {
			return
		}
		if strings.TrimSpace(body.Name) == "" || body.Cost <= 0 {
			writeError(w, http.StatusBadRequest, "奖励名称和积分必须有效")
			return
		}
		res, err := s.db.Exec(`INSERT INTO rewards (name,cost,description,stock,auto_approve,enabled) VALUES (?,?,?,?,?,1)`, body.Name, body.Cost, body.Description, body.Stock, boolInt(body.AutoApprove))
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

func (s *Server) handleAdminRewardByID(w http.ResponseWriter, r *http.Request) {
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/rewards/"), "/")
	parts := strings.Split(rest, "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusNotFound, "接口不存在")
		return
	}
	id, ok := parseID(w, parts[0])
	if !ok {
		return
	}
	if len(parts) == 2 && parts[1] == "restore" {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		res, err := s.db.Exec(`UPDATE rewards SET enabled=1 WHERE id=?`, id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			writeError(w, http.StatusNotFound, "奖励不存在")
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
		var body Reward
		if !decodeJSON(w, r, &body) {
			return
		}
		_, err := s.db.Exec(`UPDATE rewards SET name=?,cost=?,description=?,stock=?,auto_approve=?,enabled=? WHERE id=?`, body.Name, body.Cost, body.Description, body.Stock, boolInt(body.AutoApprove), boolInt(body.Enabled), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case http.MethodDelete:
		_, err := s.db.Exec(`UPDATE rewards SET enabled=0 WHERE id=?`, id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleRedemptions(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r)
	switch r.Method {
	case http.MethodGet:
		userID := int64(1)
		if sess.Role == "child" {
			userID = sess.UserID
		}
		rows, err := s.db.Query(redemptionSelect()+` WHERE rd.user_id=? ORDER BY rd.created_at DESC LIMIT 100`, userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer rows.Close()
		out, err := scanRedemptions(rows)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, out)
	case http.MethodPost:
		if sess.Role != "child" {
			// parent may create on behalf? keep child-only for redemption request
			if sess.Role != "parent" {
				writeError(w, http.StatusUnauthorized, "需要登录")
				return
			}
		}
		var body struct {
			RewardID int64 `json:"reward_id"`
		}
		if !decodeJSON(w, r, &body) {
			return
		}
		userID := int64(1)
		if sess.Role == "child" {
			userID = sess.UserID
		}
		err := s.createRedemption(userID, body.RewardID, sess.UserID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleAdminRedemptions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	rows, err := s.db.Query(redemptionSelect() + ` ORDER BY CASE rd.status WHEN 'pending' THEN 0 ELSE 1 END, rd.created_at DESC LIMIT 200`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	out, err := scanRedemptions(rows)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleAdminRedemptionAction(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/redemptions/"), "/")
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
	if parts[1] == "approve" {
		s.approveRedemption(w, r, id)
		return
	}
	if parts[1] == "reject" {
		s.rejectRedemption(w, r, id)
		return
	}
	writeError(w, http.StatusNotFound, "接口不存在")
}

func (s *Server) createRedemption(userID, rewardID, actorID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var name string
	var cost, stock, auto int
	if err := tx.QueryRow(`SELECT name,cost,stock,auto_approve FROM rewards WHERE id=? AND enabled=1`, rewardID).Scan(&name, &cost, &stock, &auto); err != nil {
		return errors.New("奖励不存在")
	}
	if stock == 0 {
		return errors.New("奖励库存不足")
	}
	balance, err := balanceTx(tx, userID)
	if err != nil {
		return err
	}
	if balance < cost {
		return errors.New("积分不足")
	}
	status := "pending"
	if auto == 1 {
		status = "fulfilled"
	}
	res, err := tx.Exec(`INSERT INTO redemptions (user_id,reward_id,cost_at_time,status) VALUES (?,?,?,?)`, userID, rewardID, cost, status)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	if auto == 1 {
		if _, err := tx.Exec(`INSERT INTO point_transactions (user_id,change,reason,source_type,source_id,created_by) VALUES (?,?,?,?,?,?)`, userID, -cost, "兑换「"+name+"」", "redemption", id, actorID); err != nil {
			return err
		}
		if stock > 0 {
			res, err := tx.Exec(`UPDATE rewards SET stock=stock-1 WHERE id=? AND stock>0`, rewardID)
			if err != nil {
				return err
			}
			if n, _ := res.RowsAffected(); n == 0 {
				return errors.New("奖励库存不足")
			}
		}
	}
	if err := audit(tx, actorID, "redemption.create", "redemption", id, name); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Server) approveRedemption(w http.ResponseWriter, r *http.Request, id int64) {
	sess := sessionFrom(r)
	var body struct {
		Note string `json:"note"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
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
	var userID, rewardID int64
	var cost int
	var status, name string
	var stock int
	if err := tx.QueryRow(`SELECT rd.user_id,rd.reward_id,rd.cost_at_time,rd.status,r.name,r.stock FROM redemptions rd JOIN rewards r ON r.id=rd.reward_id WHERE rd.id=?`, id).Scan(&userID, &rewardID, &cost, &status, &name, &stock); err != nil {
		writeError(w, http.StatusNotFound, "兑换不存在")
		return
	}
	if status != "pending" {
		writeError(w, http.StatusBadRequest, "只能审批待处理兑换")
		return
	}
	if stock == 0 {
		writeError(w, http.StatusBadRequest, "奖励库存不足")
		return
	}
	balance, _ := balanceTx(tx, userID)
	if balance < cost {
		writeError(w, http.StatusBadRequest, "积分不足")
		return
	}
	if _, err := tx.Exec(`INSERT INTO point_transactions (user_id,change,reason,source_type,source_id,created_by) VALUES (?,?,?,?,?,?)`, userID, -cost, "兑换「"+name+"」", "redemption", id, parentID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := tx.Exec(`UPDATE redemptions SET status='fulfilled',reviewed_at=CURRENT_TIMESTAMP,reviewed_by=?,review_note=? WHERE id=?`, parentID, body.Note, id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if stock > 0 {
		res, err := tx.Exec(`UPDATE rewards SET stock=stock-1 WHERE id=? AND stock>0`, rewardID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			writeError(w, http.StatusBadRequest, "奖励库存不足")
			return
		}
	}
	if err := audit(tx, parentID, "redemption.approve", "redemption", id, name); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) rejectRedemption(w http.ResponseWriter, r *http.Request, id int64) {
	sess := sessionFrom(r)
	var body struct {
		Note string `json:"note"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	parentID := sess.UserID
	if parentID == 0 {
		parentID = 2
	}
	res, err := s.db.Exec(`UPDATE redemptions SET status='rejected',reviewed_at=CURRENT_TIMESTAMP,reviewed_by=?,review_note=? WHERE id=? AND status='pending'`, parentID, body.Note, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeError(w, http.StatusBadRequest, "只能驳回待处理兑换")
		return
	}
	auditDB(s.db.DB, parentID, "redemption.reject", "redemption", id, body.Note)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

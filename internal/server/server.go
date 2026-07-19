package server

import (
	"database/sql"
	"log/slog"
	"net/http"
	"time"

	"github.com/hiccup90/scorecard/internal/config"
	"github.com/hiccup90/scorecard/internal/database"
	"github.com/hiccup90/scorecard/internal/platform/middleware"
)

type Server struct {
	cfg     config.Config
	db      *database.DB
	mux     *http.ServeMux
	limiter *loginLimiter
	log     *slog.Logger
}

func New(cfg config.Config, db *database.DB, log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}
	s := &Server{cfg: cfg, db: db, mux: http.NewServeMux(), limiter: newLoginLimiter(), log: log}
	s.routes()
	go s.sessionCleanupLoop()
	return s
}

func (s *Server) Handler() http.Handler {
	return middleware.Chain(s.mux,
		middleware.RequestID,
		middleware.Recover(s.log),
		middleware.AccessLog(s.log),
		middleware.SecurityHeaders,
	)
}

func (s *Server) routes() {
	s.mux.HandleFunc("/healthz", s.handleHealthz)
	s.mux.HandleFunc("/readyz", s.handleReadyz)
	s.mux.HandleFunc("/api/v1/health", s.handleHealthz)
	s.mux.HandleFunc("/api/v1/version", s.handleVersion)

	s.mux.HandleFunc("/api/v1/auth/parent/login", s.handleParentLogin)
	s.mux.HandleFunc("/api/v1/auth/child/login", s.handleChildLogin)
	s.mux.HandleFunc("/api/v1/auth/verify", s.requireAny(s.handleVerify))
	s.mux.HandleFunc("/api/v1/auth/logout", s.requireAny(s.handleLogout))

	s.mux.HandleFunc("/api/v1/children/1/summary", s.requireAny(s.handleChildSummary))
	s.mux.HandleFunc("/api/v1/activities", s.requireAny(s.handleActivities))
	s.mux.HandleFunc("/api/v1/checkins", s.requireAny(s.handleCheckins))
	s.mux.HandleFunc("/api/v1/checkins/", s.requireAny(s.handleCheckinByID))
	s.mux.HandleFunc("/api/v1/rewards", s.requireAny(s.handleRewards))
	s.mux.HandleFunc("/api/v1/redemptions", s.requireAny(s.handleRedemptions))
	s.mux.HandleFunc("/api/v1/transactions", s.requireAny(s.handleTransactions))

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

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if err := s.db.Ping(); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database not ready")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"version": s.cfg.Version,
		"tz":      s.cfg.Timezone,
	})
}

func (s *Server) sessionCleanupLoop() {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for range t.C {
		s.cleanupSessions()
	}
}

func (s *Server) summary(userID int64) (Summary, error) {
	var out Summary
	if err := s.db.QueryRow(`SELECT COALESCE(SUM(change),0) FROM point_transactions WHERE user_id=?`, userID).Scan(&out.Balance); err != nil {
		return out, err
	}
	// SQLite CURRENT_TIMESTAMP is UTC; convert with configured local offset for "today".
	today := s.db.Today()
	if err := s.db.QueryRow(
		`SELECT COALESCE(SUM(change),0) FROM point_transactions
		 WHERE user_id=? AND date(created_at, ?)=?`,
		userID, localOffsetSQL(s.cfg.Location), today,
	).Scan(&out.TodayTotal); err != nil {
		return out, err
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM checkins WHERE user_id=? AND status='pending'`, userID).Scan(&out.PendingCount); err != nil {
		return out, err
	}
	if err := s.db.QueryRow(`SELECT COALESCE(MAX(streak_days),0) FROM streaks WHERE user_id=?`, userID).Scan(&out.MaxStreakDays); err != nil {
		return out, err
	}
	return out, nil
}

func (s *Server) activity(id int64) (Activity, error) {
	var a Activity
	var enabled int
	err := s.db.QueryRow(`SELECT id,label,base_points,score_mode,icon,color,category,sort_order,enabled FROM activities WHERE id=? AND enabled=1`, id).Scan(&a.ID, &a.Label, &a.BasePoints, &a.ScoreMode, &a.Icon, &a.Color, &a.Category, &a.SortOrder, &enabled)
	a.Enabled = enabled == 1
	return a, err
}

func (s *Server) handleChildSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	userID := int64(1)
	if sess := sessionFrom(r); sess.Role == "child" {
		userID = sess.UserID
	}
	summary, err := s.summary(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) handleActivities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	rows, err := s.db.Query(`SELECT id,label,base_points,score_mode,icon,color,category,sort_order,enabled FROM activities WHERE enabled=1 ORDER BY category, sort_order, id`)
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
}

func (s *Server) handleTransactions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	userID := int64(1)
	if sess := sessionFrom(r); sess.Role == "child" {
		userID = sess.UserID
	}
	rows, err := s.db.Query(`SELECT id,user_id,change,reason,source_type,source_id,created_at FROM point_transactions WHERE user_id=? ORDER BY created_at DESC LIMIT 100`, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	out := []Transaction{}
	for rows.Next() {
		var t Transaction
		var sid sql.NullInt64
		if err := rows.Scan(&t.ID, &t.UserID, &t.Change, &t.Reason, &t.SourceType, &sid, &t.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if sid.Valid {
			v := sid.Int64
			t.SourceID = &v
		}
		out = append(out, t)
	}
	writeJSON(w, http.StatusOK, out)
}

package server

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"
)

type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{attempts: map[string][]time.Time{}}
}

func (l *loginLimiter) allow(key string, limit int, window time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-window)
	list := l.attempts[key]
	kept := list[:0]
	for _, t := range list {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= limit {
		l.attempts[key] = kept
		return false
	}
	l.attempts[key] = append(kept, now)
	return true
}

func (s *Server) handleParentLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	ip := clientIP(r)
	if !s.limiter.allow("parent:"+ip, 10, 15*time.Minute) {
		writeError(w, http.StatusTooManyRequests, "尝试次数过多，请稍后再试")
		return
	}
	var body struct {
		PIN string `json:"pin"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	pin := strings.TrimSpace(body.PIN)
	if pin == "" || pin != s.cfg.AdminPIN {
		writeError(w, http.StatusForbidden, "PIN 码错误")
		return
	}
	token, err := s.createSession(2, "parent")
	if err != nil {
		s.log.Error("create parent session", "err", err)
		writeError(w, http.StatusInternalServerError, "登录失败：无法创建会话，请检查数据库 sessions 表")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "token": token, "role": "parent", "user_id": 2})
}

func (s *Server) handleChildLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	ip := clientIP(r)
	if !s.limiter.allow("child:"+ip, 15, 15*time.Minute) {
		writeError(w, http.StatusTooManyRequests, "尝试次数过多，请稍后再试")
		return
	}
	var body struct {
		PIN string `json:"pin"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	pin := strings.TrimSpace(body.PIN)
	// Child PIN defaults to ADMIN_PIN when CHILD_PIN unset; accept either for convenience.
	if pin == "" || (pin != s.cfg.ChildPIN && pin != s.cfg.AdminPIN) {
		writeError(w, http.StatusForbidden, "PIN 码错误")
		return
	}
	token, err := s.createSession(1, "child")
	if err != nil {
		s.log.Error("create child session", "err", err)
		writeError(w, http.StatusInternalServerError, "登录失败：无法创建会话，请检查数据库 sessions 表")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "token": token, "role": "child", "user_id": 1})
}

func (s *Server) handleVerify(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r)
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "role": sess.Role, "user_id": sess.UserID})
}

func (s *Server) createSession(userID int64, role string) (string, error) {
	token, err := newToken()
	if err != nil {
		return "", err
	}
	exp := time.Now().UTC().Add(s.cfg.TokenTTL).Format(time.RFC3339)
	if _, err := s.db.Exec(`INSERT INTO sessions (token, user_id, role, expires_at) VALUES (?,?,?,?)`, token, userID, role, exp); err != nil {
		return "", err
	}
	return token, nil
}

func (s *Server) lookupSession(token string) (session, bool) {
	if token == "" {
		return session{}, false
	}
	var sess session
	var expRaw string
	err := s.db.QueryRow(`SELECT user_id, role, expires_at FROM sessions WHERE token=?`, token).Scan(&sess.UserID, &sess.Role, &expRaw)
	if err != nil {
		return session{}, false
	}
	exp, err := time.Parse(time.RFC3339, expRaw)
	if err != nil || time.Now().UTC().After(exp) {
		_, _ = s.db.Exec(`DELETE FROM sessions WHERE token=?`, token)
		return session{}, false
	}
	return sess, true
}

func (s *Server) requireRole(roles ...string) func(http.HandlerFunc) http.HandlerFunc {
	allowed := map[string]bool{}
	for _, r := range roles {
		allowed[r] = true
	}
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			token := r.Header.Get("X-Auth-Token")
			sess, ok := s.lookupSession(token)
			if !ok || !allowed[sess.Role] {
				writeError(w, http.StatusUnauthorized, "需要登录")
				return
			}
			next(w, r.WithContext(withSession(r.Context(), sess)))
		}
	}
}

func (s *Server) requireParent(next http.HandlerFunc) http.HandlerFunc {
	return s.requireRole("parent")(next)
}

func (s *Server) requireChild(next http.HandlerFunc) http.HandlerFunc {
	return s.requireRole("child")(next)
}

func (s *Server) requireAny(next http.HandlerFunc) http.HandlerFunc {
	return s.requireRole("child", "parent")(next)
}

func (s *Server) cleanupSessions() {
	_, _ = s.db.Exec(`DELETE FROM sessions WHERE expires_at < ?`, time.Now().UTC().Format(time.RFC3339))
}

func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	return r.RemoteAddr
}

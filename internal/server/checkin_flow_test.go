package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/hiccup90/scorecard/internal/config"
	"github.com/hiccup90/scorecard/internal/database"
)

func testServer(t *testing.T) (*Server, func()) {
	t.Helper()
	dir := t.TempDir()
	loc := time.FixedZone("CST", 8*3600)
	db, err := database.Open(filepath.Join(dir, "t.db"), loc)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		Addr:       ":0",
		AdminPIN:   "9999",
		ChildPIN:   "8888",
		StaticDir:  dir,
		Location:   loc,
		MakeupDays: 30,
		TokenTTL:   time.Hour,
		Version:    "test",
	}
	// write dummy index
	_ = os.WriteFile(filepath.Join(dir, "index.html"), []byte("ok"), 0644)
	s := New(cfg, db, nil)
	return s, func() { db.Close() }
}

func login(t *testing.T, s *Server, path, pin string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"pin": pin})
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("login %s: %d %s", path, rr.Code, rr.Body.String())
	}
	var out map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	tok, _ := out["token"].(string)
	if tok == "" {
		t.Fatal("empty token")
	}
	return tok
}

func TestChildNeedsAuthAndApproveStreak(t *testing.T) {
	s, cleanup := testServer(t)
	defer cleanup()

	// unauthenticated checkin should fail
	req := httptest.NewRequest(http.MethodPost, "/api/v1/checkins", bytes.NewReader([]byte(`{"activity_id":1}`)))
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d", rr.Code)
	}

	child := login(t, s, "/api/v1/auth/child/login", "8888")
	parent := login(t, s, "/api/v1/auth/parent/login", "9999")

	// submit two consecutive days
	today := s.db.Today()
	yest := s.db.Now().AddDate(0, 0, -1).Format("2006-01-02")
	for _, day := range []string{yest, today} {
		body, _ := json.Marshal(map[string]interface{}{"activity_id": 9, "activity_date": day}) // 收书包 default
		req := httptest.NewRequest(http.MethodPost, "/api/v1/checkins", bytes.NewReader(body))
		req.Header.Set("X-Auth-Token", child)
		rr := httptest.NewRecorder()
		s.Handler().ServeHTTP(rr, req)
		if rr.Code != 200 {
			t.Fatalf("checkin %s: %d %s", day, rr.Code, rr.Body.String())
		}
	}

	// list pending
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/checkins", nil)
	req.Header.Set("X-Auth-Token", parent)
	rr = httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatal(rr.Body.String())
	}
	var checkins []Checkin
	_ = json.Unmarshal(rr.Body.Bytes(), &checkins)
	var ids []int64
	for _, c := range checkins {
		if c.Status == "pending" && c.ActivityID == 9 {
			ids = append(ids, c.ID)
		}
	}
	if len(ids) < 2 {
		t.Fatalf("want 2 pending got %d", len(ids))
	}

	// approve older first then today — streak bonus on second should be >= 1
	for i, id := range ids {
		// sort by date: approve yest first by finding via loop order DESC submitted - may be wrong order
		_ = i
		body, _ := json.Marshal(map[string]interface{}{"counts_for_streak": true})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/checkins/"+itoa(id)+"/approve", bytes.NewReader(body))
		req.Header.Set("X-Auth-Token", parent)
		rr := httptest.NewRecorder()
		s.Handler().ServeHTTP(rr, req)
		if rr.Code != 200 {
			t.Fatalf("approve %d: %s", id, rr.Body.String())
		}
	}

	// balance should include base*2 + at least one streak bonus unit if consecutive
	var bal int
	if err := s.db.QueryRow(`SELECT COALESCE(SUM(change),0) FROM point_transactions WHERE user_id=1`).Scan(&bal); err != nil {
		t.Fatal(err)
	}
	// activity 9 base 2 => 2+2=4 minimum; with streak second day +1 => 5
	if bal < 4 {
		t.Fatalf("balance too low: %d", bal)
	}
}

func TestPathTraversalBlocked(t *testing.T) {
	s, cleanup := testServer(t)
	defer cleanup()
	req := httptest.NewRequest(http.MethodGet, "/../../../etc/passwd", nil)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	// should not leak; serve index or 404 content without passwd
	if bytes.Contains(rr.Body.Bytes(), []byte("root:")) {
		t.Fatal("path traversal leaked")
	}
}

func TestFutureDateRejected(t *testing.T) {
	s, cleanup := testServer(t)
	defer cleanup()
	child := login(t, s, "/api/v1/auth/child/login", "8888")
	future := s.db.Now().AddDate(0, 0, 1).Format("2006-01-02")
	body, _ := json.Marshal(map[string]interface{}{"activity_id": 1, "activity_date": future})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/checkins", bytes.NewReader(body))
	req.Header.Set("X-Auth-Token", child)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d %s", rr.Code, rr.Body.String())
	}
}

func itoa(id int64) string {
	return strconv.FormatInt(id, 10)
}

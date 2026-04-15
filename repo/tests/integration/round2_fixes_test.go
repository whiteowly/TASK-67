package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/campusrec/campusrec/internal/scheduler"
	"github.com/campusrec/campusrec/tests/testutil"
)

// ---------------------------------------------------------------------------
// Fix 1: Scheduler — tasks register and execute
// ---------------------------------------------------------------------------

func TestSchedulerRunsTasks(t *testing.T) {
	callCount := 0
	s := scheduler.New()
	s.Register("test", 100*time.Millisecond, func(ctx context.Context) error {
		callCount++
		return nil
	})
	s.Start()
	time.Sleep(350 * time.Millisecond)
	s.Stop()
	if callCount < 2 {
		t.Errorf("expected >=2 runs, got %d", callCount)
	}
}

func TestSchedulerRecoversPanic(t *testing.T) {
	s := scheduler.New()
	s.Register("panic", 50*time.Millisecond, func(ctx context.Context) error {
		panic("boom")
	})
	s.Start()
	time.Sleep(150 * time.Millisecond)
	s.Stop()
}

// ---------------------------------------------------------------------------
// Fix 2: Attendance policy enforcement
// ---------------------------------------------------------------------------

func TestCheckinRejectsOutsideWindow(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	staff := loginExistingUser(t, r, "staff1", "Seed@Pass1234")
	member := loginExistingUser(t, r, "member1", "Seed@Pass1234")

	// Register for a session (starts in 48h)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/catalog/sessions?status=published", "", member))
	var sess struct{ Data []struct{ ID string `json:"id"` } `json:"data"` }
	json.Unmarshal(w.Body.Bytes(), &sess)
	if len(sess.Data) == 0 {
		t.Skip("no sessions")
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/registrations",
		fmt.Sprintf(`{"session_id":"%s"}`, sess.Data[0].ID), member))
	rid := extractID(t, w.Body.Bytes())
	if rid == "" {
		t.Skip("no reg")
	}

	// Check-in should fail — session is 48h away, outside any lead window
	w = httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/attendance/checkin",
		fmt.Sprintf(`{"registration_id":"%s","method":"qr_staff"}`, rid), staff))
	if w.Code == 200 {
		t.Error("check-in should fail — outside lead window")
	}
	if w.Code == 500 {
		t.Fatalf("check-in returned 500: %s", w.Body.String())
	}
}

func TestLeaveRejectsInvalidRegistration(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	member := loginExistingUser(t, r, "member1", "Seed@Pass1234")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/attendance/leave",
		`{"registration_id":"00000000-0000-0000-0000-000000000000"}`, member))
	if w.Code == 500 {
		t.Fatal("leave invalid should not 500")
	}
	if w.Code != 400 {
		t.Errorf("leave invalid: expected 400, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// Fix 3: Stale occupancy + no-show service methods work
// ---------------------------------------------------------------------------

func TestDetectNoShowsOnEmptyDB(t *testing.T) {
	_, svc := testutil.SetupTestRouter(t)
	count, err := svc.Attendance.DetectNoShows(context.Background())
	if err != nil {
		t.Fatalf("DetectNoShows: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 on empty DB, got %d", count)
	}
}

func TestDetectStaleOccupancyOnEmptyDB(t *testing.T) {
	_, svc := testutil.SetupTestRouter(t)
	count, err := svc.Attendance.DetectStaleOccupancy(context.Background())
	if err != nil {
		t.Fatalf("DetectStaleOccupancy: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 on empty DB, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// Fix 4: Import pipeline — real checksum duplicate detection
// ---------------------------------------------------------------------------

func TestImportDuplicateChecksumRejected(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	admin := loginExistingUser(t, r, "admin", "Seed@Pass1234")

	csvContent := "name,email\nAlice,a@b.com\n"

	// First upload
	w := httptest.NewRecorder()
	r.ServeHTTP(w, makeMultipartReq("/api/v1/imports", "data.csv", csvContent, "general", admin))
	if w.Code != 200 && w.Code != 201 {
		t.Fatalf("first upload: %d: %s", w.Code, w.Body.String())
	}

	// Second upload with same content (different filename) → duplicate
	w = httptest.NewRecorder()
	r.ServeHTTP(w, makeMultipartReq("/api/v1/imports", "data2.csv", csvContent, "general", admin))
	if w.Code == 200 || w.Code == 201 {
		t.Fatal("duplicate content upload should be rejected")
	}
}

// ---------------------------------------------------------------------------
// Fix 5: Backup produces real metadata
// ---------------------------------------------------------------------------

func TestBackupMetadataIsReal(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	admin := loginExistingUser(t, r, "admin", "Seed@Pass1234")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/admin/backups", "", admin))
	if w.Code != 200 && w.Code != 201 {
		t.Fatalf("backup: %d", w.Code)
	}

	var resp struct {
		Data struct {
			Status       string  `json:"status"`
			ArtifactPath *string `json:"artifact_path"`
			Checksum     *string `json:"checksum"`
			SizeBytes    *int64  `json:"size_bytes"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Data.Status != "completed" {
		t.Errorf("status: %q", resp.Data.Status)
	}
	if resp.Data.Checksum == nil || *resp.Data.Checksum == "placeholder" || *resp.Data.Checksum == "" {
		t.Error("checksum should be real")
	}
	if resp.Data.ArtifactPath == nil || *resp.Data.ArtifactPath == "" {
		t.Error("artifact_path should be set")
	}
	if resp.Data.SizeBytes == nil || *resp.Data.SizeBytes == 0 {
		t.Error("size_bytes should be non-zero")
	}
}

func TestArchiveRunCompletes(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	admin := loginExistingUser(t, r, "admin", "Seed@Pass1234")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("POST", "/api/v1/admin/archives",
		`{"archive_type":"orders"}`, admin))
	if w.Code != 200 && w.Code != 201 {
		t.Fatalf("archive: %d: %s", w.Code, w.Body.String())
	}

	var resp struct{ Data struct{ Status string `json:"status"` } `json:"data"` }
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Data.Status != "completed" {
		t.Errorf("archive status: %q", resp.Data.Status)
	}
}

// ---------------------------------------------------------------------------
// Fix 6: KPI returns real data from seeded DB
// ---------------------------------------------------------------------------

func TestKPIReturnsRealData(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	admin := loginExistingUser(t, r, "admin", "Seed@Pass1234")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/admin/kpis", "", admin))
	if w.Code != 200 {
		t.Fatalf("kpi: %d", w.Code)
	}

	var resp struct {
		Data struct {
			TotalUsers     int `json:"total_users"`
			ActiveSessions int `json:"active_sessions"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Data.TotalUsers == 0 {
		t.Error("total_users should be >0 with seeded data")
	}
	if resp.Data.ActiveSessions == 0 {
		t.Error("active_sessions should be >0 with seeded data")
	}
}

func TestJobStatusEndpoint(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	admin := loginExistingUser(t, r, "admin", "Seed@Pass1234")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/admin/jobs", "", admin))
	if w.Code != 200 {
		t.Fatalf("jobs: %d", w.Code)
	}
	var resp struct{ Success bool `json:"success"` }
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.Success {
		t.Error("jobs should return success")
	}
}

// ---------------------------------------------------------------------------
// Fix 7: Ticket GET returns 404
// ---------------------------------------------------------------------------

func TestTicketGetMissingReturns404(t *testing.T) {
	r, _ := testutil.SetupTestRouter(t)
	staff := loginExistingUser(t, r, "staff1", "Seed@Pass1234")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, authReq("GET", "/api/v1/tickets/00000000-0000-0000-0000-000000000000", "", staff))
	if w.Code == 200 {
		t.Fatal("missing ticket should not return 200")
	}
	if w.Code == 500 {
		t.Fatal("missing ticket returned 500")
	}
	// 404 expected (staff has access, but ticket doesn't exist)
	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func makeMultipartReq(path, filename, content, templateType, token string) *http.Request {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, _ := w.CreateFormFile("file", filename)
	fw.Write([]byte(content))
	w.WriteField("template_type", templateType)
	w.Close()

	req, _ := http.NewRequest("POST", path, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.AddCookie(&http.Cookie{Name: "session_token", Value: token})
	return req
}

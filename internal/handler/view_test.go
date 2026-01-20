package handler

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"shift-change-app/internal/database"
)

type captureRenderer struct {
	template string
	data     interface{}
	called   bool
}

func (r *captureRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	r.template = name
	r.data = data
	r.called = true
	return nil
}

type expectedQuery struct {
	query  string
	result fakeResult
}

type fakeResult struct {
	columns []string
	values  [][]driver.Value
}

type fakeDB struct {
	expected []expectedQuery
	index    int
	mu       sync.Mutex
}

func (db *fakeDB) next(query string) (fakeResult, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.index >= len(db.expected) {
		return fakeResult{}, fmt.Errorf("unexpected query: %s", query)
	}
	expected := db.expected[db.index]
	trimmed := strings.TrimSpace(query)
	if trimmed != expected.query {
		return fakeResult{}, fmt.Errorf("unexpected query: %s", query)
	}
	db.index++
	return expected.result, nil
}

func (db *fakeDB) expectationsMet() error {
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.index != len(db.expected) {
		return fmt.Errorf("not all expectations met: %d/%d", db.index, len(db.expected))
	}
	return nil
}

type fakeDriver struct{}

type fakeConn struct {
	db *fakeDB
}

func (d *fakeDriver) Open(name string) (driver.Conn, error) {
	db, ok := fakeRegistry.Load(name)
	if !ok {
		return nil, fmt.Errorf("unknown fake db: %s", name)
	}
	return &fakeConn{db: db.(*fakeDB)}, nil
}

func (c *fakeConn) Prepare(query string) (driver.Stmt, error) {
	return nil, errors.New("prepare not supported")
}

func (c *fakeConn) Close() error {
	return nil
}

func (c *fakeConn) Begin() (driver.Tx, error) {
	return nil, errors.New("transactions not supported")
}

func (c *fakeConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	result, err := c.db.next(query)
	if err != nil {
		return nil, err
	}
	return &fakeRows{columns: result.columns, values: result.values}, nil
}

func (c *fakeConn) Query(query string, args []driver.Value) (driver.Rows, error) {
	return nil, errors.New("query without context not supported")
}

type fakeRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *fakeRows) Columns() []string {
	return r.columns
}

func (r *fakeRows) Close() error {
	return nil
}

func (r *fakeRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	row := r.values[r.index]
	for i := range dest {
		dest[i] = row[i]
	}
	r.index++
	return nil
}

var (
	registerOnce sync.Once
	fakeRegistry sync.Map
	fakeCounter  uint64
)

func openFakeDB(expected []expectedQuery) (*sql.DB, *fakeDB, error) {
	registerOnce.Do(func() {
		sql.Register("fake-driver", &fakeDriver{})
	})
	id := atomic.AddUint64(&fakeCounter, 1)
	dsn := fmt.Sprintf("fake-%d", id)
	fake := &fakeDB{expected: expected}
	fakeRegistry.Store(dsn, fake)
	db, err := sql.Open("fake-driver", dsn)
	if err != nil {
		return nil, nil, err
	}
	return db, fake, nil
}

func TestShowHome_MissingUserID(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/home", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := &Handler{}
	if err := h.ShowHome(c); err != nil {
		t.Fatalf("ShowHome returned error: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != "user_id is required" {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestShowHome_Success(t *testing.T) {
	e := echo.New()
	renderer := &captureRenderer{}
	e.Renderer = renderer

	userID := uuid.New()
	groupID := uuid.New()
	now := time.Now().UTC()

	expected := []expectedQuery{
		{
			query: strings.TrimSpace(`-- name: GetUserByID :one
SELECT id, line_user_id, display_name, profile_image_url, created_at, updated_at, deleted_at FROM users WHERE id = $1`),
			result: fakeResult{
				columns: []string{"id", "line_user_id", "display_name", "profile_image_url", "created_at", "updated_at", "deleted_at"},
				values: [][]driver.Value{{
					userID.String(),
					"line-user",
					"User Name",
					nil,
					now,
					now,
					nil,
				}},
			},
		},
		{
			query: strings.TrimSpace(`-- name: ListUserGroups :many
SELECT g.id, g.name, g.invitation_code, gm.role
FROM job_groups g
         JOIN group_members gm ON g.id = gm.group_id
WHERE gm.user_id = $1
  AND g.deleted_at IS NULL
ORDER BY g.created_at DESC`),
			result: fakeResult{
				columns: []string{"id", "name", "invitation_code", "role"},
				values: [][]driver.Value{{
					groupID.String(),
					"Group A",
					"INVITE",
					"member",
				}},
			},
		},
	}

	db, fake, err := openFakeDB(expected)
	if err != nil {
		t.Fatalf("failed to open fake db: %v", err)
	}
	defer db.Close()

	queries := database.New(db)
	h := NewHandler(db, queries, nil)

	if err := os.Setenv("LIFF_ID", "liff-123"); err != nil {
		t.Fatalf("failed to set env: %v", err)
	}
	defer os.Unsetenv("LIFF_ID")

	req := httptest.NewRequest(http.MethodGet, "/home?user_id="+userID.String(), nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.ShowHome(c); err != nil {
		t.Fatalf("ShowHome returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if !renderer.called {
		t.Fatalf("renderer was not called")
	}
	if renderer.template != "home.html" {
		t.Fatalf("expected template home.html, got %s", renderer.template)
	}

	data, ok := renderer.data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map, got %T", renderer.data)
	}

	user, ok := data["User"].(database.User)
	if !ok {
		t.Fatalf("expected User in data")
	}
	if user.ID != userID {
		t.Fatalf("expected user ID %s, got %s", userID, user.ID)
	}
	if user.DisplayName != "User Name" {
		t.Fatalf("unexpected user display name: %s", user.DisplayName)
	}

	currentUserID, ok := data["CurrentUserID"].(string)
	if !ok {
		t.Fatalf("expected CurrentUserID string")
	}
	if currentUserID != userID.String() {
		t.Fatalf("unexpected CurrentUserID: %s", currentUserID)
	}

	groups, ok := data["UserGroups"].([]database.ListUserGroupsRow)
	if !ok {
		t.Fatalf("expected UserGroups slice")
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if groups[0].ID != groupID {
		t.Fatalf("unexpected group ID: %s", groups[0].ID)
	}
	if groups[0].Name != "Group A" {
		t.Fatalf("unexpected group name: %s", groups[0].Name)
	}

	liffID, ok := data["LiffID"].(string)
	if !ok {
		t.Fatalf("expected LiffID string")
	}
	if liffID != "liff-123" {
		t.Fatalf("unexpected LiffID: %s", liffID)
	}

	if err := fake.expectationsMet(); err != nil {
		t.Fatalf("query expectations not met: %v", err)
	}
}

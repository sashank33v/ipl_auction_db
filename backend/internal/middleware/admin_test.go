package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdminOnly(t *testing.T) {
	cases := []struct {
		name, userID string
		allowed      bool
		err          error
		want         int
		lookup       bool
	}{
		{"anonymous", "", false, nil, 401, false},
		{"participant", "participant", false, nil, 403, true},
		{"host without admin", "host", false, nil, 403, true},
		{"admin", "admin", true, nil, 204, true},
		{"lookup failure", "admin", true, errors.New("database unavailable"), 500, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			checked, reached := false, false
			h := AdminOnly(func(ctx context.Context, id string) (bool, error) {
				checked = true
				if id != tc.userID {
					t.Fatalf("unexpected id %q", id)
				}
				return tc.allowed, tc.err
			})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true; w.WriteHeader(http.StatusNoContent) }))
			req := httptest.NewRequest(http.MethodPost, "/admin/players/", nil)
			req.Header.Set("X-Admin", "true") // client-supplied permission is ignored
			if tc.userID != "" {
				req = req.WithContext(context.WithValue(req.Context(), userIDContextKey, tc.userID))
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.want || reached != (tc.want == 204) || checked != tc.lookup {
				t.Fatalf("status=%d handler=%t lookup=%t", rec.Code, reached, checked)
			}
		})
	}
}

func TestAdminPermissionRevocation(t *testing.T) {
	allowed := true
	h := AdminOnly(func(context.Context, string) (bool, error) { return allowed, nil })(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	req := httptest.NewRequest(http.MethodDelete, "/admin/players/example", nil)
	req = req.WithContext(context.WithValue(req.Context(), userIDContextKey, "admin"))
	for _, want := range []int{204, 403} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("got %d want %d", rec.Code, want)
		}
		allowed = false
	}
}

package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// chdirRepoRoot points the process at the repository root so the admin SPA
// static build (static/admin) used by webui.Handler is discoverable, and
// restores the previous working directory when the test finishes.
func chdirRepoRoot(t *testing.T) {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Skip("cannot locate test source; skip static-admin assertions")
	}
	repoRoot := filepath.Dir(filepath.Dir(filename))
	if _, err := os.Stat(filepath.Join(repoRoot, "static", "admin", "index.html")); err != nil {
		t.Skip("static/admin build not present; skip static-admin assertions")
	}
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Fatalf("restore wd: %v", err)
		}
	})
}

func TestAdminDocumentNavigationServesSPAOnAPIPathCollision(t *testing.T) {
	chdirRepoRoot(t)
	t.Setenv("DS2API_CONFIG_JSON", `{"keys":["k1"],"accounts":[{"email":"u@example.com","password":"p"}]}`)
	t.Setenv("DS2API_ENV_WRITEBACK", "0")

	app, err := NewApp()
	if err != nil {
		t.Fatalf("NewApp() error: %v", err)
	}

	cases := []struct {
		name string
		path string
	}{
		{name: "accounts", path: "/admin/accounts"},
		{name: "proxies", path: "/admin/proxies"},
		{name: "settings", path: "/admin/settings"},
		{name: "test", path: "/admin/test"},
		{name: "import", path: "/admin/import"},
		{name: "vercel", path: "/admin/vercel"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Browser refresh: document navigation with no credentials.
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.Header.Set("Sec-Fetch-Mode", "navigate")
			req.Header.Set("Accept", "text/html,application/xhtml+xml")
			rec := httptest.NewRecorder()
			app.Router.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("refresh %s: status = %d, want 200 (body=%s)", tc.path, rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), "DS2API") {
				t.Fatalf("refresh %s: expected SPA shell, got: %s", tc.path, rec.Body.String()[:200])
			}

			// API call without credentials must still be rejected by the API.
			req = httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec = httptest.NewRecorder()
			app.Router.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("api %s: status = %d, want 401", tc.path, rec.Code)
			}

			// Old browser without Sec-Fetch-*: navigation Accept header and no
			// Authorization header should also render the SPA shell.
			req = httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.Header.Set("Accept", "text/html,application/xhtml+xml")
			rec = httptest.NewRecorder()
			app.Router.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("legacy refresh %s: status = %d, want 200", tc.path, rec.Code)
			}
		})
	}
}

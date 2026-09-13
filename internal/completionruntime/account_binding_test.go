package completionruntime

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"ds2api/internal/account"
	"ds2api/internal/auth"
	"ds2api/internal/config"
	dsclient "ds2api/internal/deepseek/client"
	"ds2api/internal/promptcompat"
)

type switchingResourceCaller struct {
	fakeDeepSeekCaller
	switches       int
	uploadAccounts []string
}

func (d *switchingResourceCaller) UploadFile(ctx context.Context, a *auth.RequestAuth, req dsclient.UploadFileRequest, _ int) (*dsclient.UploadFileResult, error) {
	if len(d.uploadAccounts) > 0 && a.SwitchAccount(ctx) {
		d.switches++
	}
	d.uploadAccounts = append(d.uploadAccounts, a.AccountID)
	return &dsclient.UploadFileResult{ID: fmt.Sprintf("file-%d-%s", len(d.uploadAccounts), a.AccountID)}, nil
}

func (d *switchingResourceCaller) GetPow(ctx context.Context, a *auth.RequestAuth, _ int) (string, error) {
	if a.SwitchAccount(ctx) {
		d.switches++
	}
	return "pow", nil
}

func TestCompletionResourcesStayOnOneAccount(t *testing.T) {
	for _, withFiles := range []bool{false, true} {
		t.Run(fmt.Sprint("withFiles=", withFiles), func(t *testing.T) {
			t.Setenv("DS2API_CONFIG_JSON", `{"keys":["key"],"accounts":[{"email":"a@test.com","password":"p"},{"email":"b@test.com","password":"p"}]}`)
			store := config.LoadStore()
			resolver := auth.NewResolver(store, account.NewPool(store), func(_ context.Context, a config.Account) (string, error) { return "token-" + a.Identifier(), nil })
			req, err := http.NewRequest(http.MethodPost, "http://local.test/", nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", "Bearer key")
			a, err := resolver.Determine(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resolver.Release(a)
			originalAccount := a.AccountID
			ds := &switchingResourceCaller{fakeDeepSeekCaller: fakeDeepSeekCaller{sessionByAccount: true}}
			std := promptcompat.StandardRequest{ResolvedModel: "deepseek-flash", FinalPrompt: "hello", Messages: []any{map[string]any{"role": "user", "content": "hello"}}, ToolsRaw: []any{map[string]any{"type": "function", "function": map[string]any{"name": "weather", "parameters": map[string]any{"type": "object"}}}}}
			opts := Options{}
			if withFiles {
				opts.CurrentInputFile = currentInputRuntimeConfig{}
			}
			result, outErr := StartCompletion(context.Background(), ds, a, std, opts)
			if outErr != nil {
				t.Fatalf("start: %#v", outErr)
			}
			if err := result.Response.Body.Close(); err != nil {
				t.Fatal(err)
			}
			if ds.switches != 0 || a.AccountID != originalAccount {
				t.Fatalf("upstream resources crossed accounts: switches=%d uploads=%v session=%s completion_account=%s", ds.switches, ds.uploadAccounts, result.SessionID, a.AccountID)
			}
			if withFiles && len(ds.uploadAccounts) != 2 {
				t.Fatalf("expected HISTORY and TOOLS uploads, got %v", ds.uploadAccounts)
			}
		})
	}
}

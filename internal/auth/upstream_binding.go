package auth

import "context"

// BindUpstreamAccount keeps a newly created session or generated context file
// on its owning account. Transport retries cannot reuse it with another token.
func (a *RequestAuth) BindUpstreamAccount() {
	if a != nil && a.UseConfigToken && a.AccountID != "" {
		a.upstreamAccount = a.AccountID
	}
}

// SwitchAccountForFreshCompletion is only for runtime retries that create a new
// session and reupload generated context files. Explicit attachment/account pins
// in TargetAccount remain in force.
func (a *RequestAuth) SwitchAccountForFreshCompletion(ctx context.Context) bool {
	if a == nil || a.resolver == nil {
		return false
	}
	previous := a.upstreamAccount
	a.upstreamAccount = ""
	if a.SwitchAccount(ctx) {
		return true
	}
	if a.AccountID == previous {
		a.upstreamAccount = previous
	}
	return false
}

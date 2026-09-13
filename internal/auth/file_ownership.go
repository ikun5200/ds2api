package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"ds2api/internal/config"
)

const (
	fileOwnerTTL      = 24 * time.Hour
	maxFileOwnerCache = 10000
)

var ErrFileAccountConflict = errors.New("file references belong to a different DeepSeek account")

type fileOwner struct {
	accountID string
	updatedAt time.Time
}

// RememberFileOwner records successful uploads/verification without changing
// the upstream file ID. Managed callers have separate ownership namespaces.
func (a *RequestAuth) RememberFileOwner(fileID string) {
	fileID = strings.TrimSpace(fileID)
	if a == nil || a.resolver == nil || !a.UseConfigToken || a.AccountID == "" || a.CallerID == "" || fileID == "" {
		return
	}
	r := a.resolver
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fileOwners == nil {
		r.fileOwners = make(map[string]fileOwner)
	}
	now := time.Now()
	key := a.CallerID + "\x00" + fileID
	if _, exists := r.fileOwners[key]; !exists && len(r.fileOwners) >= maxFileOwnerCache {
		oldestKey := ""
		oldestTime := now
		for candidate, owner := range r.fileOwners {
			if now.Sub(owner.updatedAt) >= fileOwnerTTL {
				delete(r.fileOwners, candidate)
			} else if owner.updatedAt.Before(oldestTime) {
				oldestKey, oldestTime = candidate, owner.updatedAt
			}
		}
		if len(r.fileOwners) >= maxFileOwnerCache {
			delete(r.fileOwners, oldestKey)
		}
	}
	r.fileOwners[key] = fileOwner{accountID: a.AccountID, updatedAt: now}
}

func (a *RequestAuth) FileOwner(fileID string) (string, bool) {
	if a == nil || a.resolver == nil || !a.UseConfigToken || a.CallerID == "" {
		return "", false
	}
	r := a.resolver
	r.mu.Lock()
	defer r.mu.Unlock()
	key := a.CallerID + "\x00" + strings.TrimSpace(fileID)
	owner, ok := r.fileOwners[key]
	if !ok {
		return "", false
	}
	if time.Since(owner.updatedAt) >= fileOwnerTTL {
		delete(r.fileOwners, key)
		return "", false
	}
	return owner.accountID, true
}

// SelectFileAccount runs before creating any upstream resource. Releasing the
// old pool lease before acquiring the owner avoids cross-account lease deadlock.
func (a *RequestAuth) SelectFileAccount(ctx context.Context, owner string) error {
	if a == nil || !a.UseConfigToken {
		return nil
	}
	owner = strings.TrimSpace(owner)
	if owner == "" {
		return ErrNoAccount
	}
	if (a.TargetAccount != "" && a.AccountID != owner) || (a.upstreamAccount != "" && a.upstreamAccount != owner) {
		return ErrFileAccountConflict
	}
	if a.AccountID == owner {
		a.TargetAccount = owner
		return nil
	}
	if a.resolver == nil {
		return ErrNoAccount
	}
	r := a.resolver
	if a.AccountID != "" {
		r.Pool.Release(a.AccountID)
	}
	// A failed acquisition leaves no lease for the caller's deferred Release.
	a.AccountID, a.DeepSeekToken = "", ""
	a.Account = config.Account{}
	a.TargetAccount = owner
	next, err := r.acquireManagedRequestAuth(ctx, a.CallerID, owner)
	if err != nil {
		return err
	}
	*a = *next
	return nil
}

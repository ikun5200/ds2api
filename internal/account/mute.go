package account

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ds2api/internal/config"
)

const (
	defaultMuteDuration   = 30 * time.Minute
	muteProbeGrace        = 2 * time.Minute
	muteProbeErrorBackoff = 15 * time.Minute
	accountHealthFilePerm = 0o600
	accountHealthDirPerm  = 0o755
)

// MuteState is the persisted upstream risk-control state for one account.
// MutedUntil is the upstream-provided unmute timestamp when available.
type MuteState struct {
	Muted       bool      `json:"muted"`
	Reason      string    `json:"reason,omitempty"`
	MutedAt     time.Time `json:"muted_at,omitempty"`
	MutedUntil  time.Time `json:"muted_until,omitempty"`
	NextProbeAt time.Time `json:"next_probe_at,omitempty"`
	LastProbeAt time.Time `json:"last_probe_at,omitempty"`
	ProbeCount  int       `json:"probe_count,omitempty"`
	LastError   string    `json:"last_error,omitempty"`
}

func (p *Pool) MarkMuted(accountID string, until time.Time, reason string) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return
	}
	now := time.Now()
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.mute == nil {
		p.mute = map[string]MuteState{}
	}
	state := p.mute[accountID]
	state.Muted = true
	state.Reason = strings.TrimSpace(reason)
	if state.MutedAt.IsZero() {
		state.MutedAt = now
	}
	if !until.IsZero() {
		state.MutedUntil = until
	} else if state.MutedUntil.IsZero() {
		state.MutedUntil = now.Add(defaultMuteDuration)
	}
	state.NextProbeAt = nextProbeAt(now, state.MutedUntil)
	p.mute[accountID] = state
	p.saveMuteStateLocked()
}

func (p *Pool) MarkProbeResult(accountID string, muted bool, until time.Time, reason string, probeErr error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return
	}
	now := time.Now()
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.mute == nil {
		p.mute = map[string]MuteState{}
	}
	state, exists := p.mute[accountID]
	if !exists {
		state = MuteState{Muted: true, MutedAt: now}
	}
	state.LastProbeAt = now
	state.ProbeCount++
	switch {
	case probeErr != nil:
		state.LastError = probeErr.Error()
		state.NextProbeAt = now.Add(muteProbeErrorBackoff)
	case muted:
		state.Muted = true
		state.Reason = strings.TrimSpace(reason)
		if !until.IsZero() {
			state.MutedUntil = until
		}
		state.NextProbeAt = nextProbeAt(now, state.MutedUntil)
		state.LastError = ""
	default:
		delete(p.mute, accountID)
		p.saveMuteStateLocked()
		p.notifyWaiterLocked()
		return
	}
	p.mute[accountID] = state
	p.saveMuteStateLocked()
}

func (p *Pool) ClearMute(accountID string) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.mute[accountID]; !ok {
		return
	}
	delete(p.mute, accountID)
	p.saveMuteStateLocked()
	p.notifyWaiterLocked()
}

func (p *Pool) MuteState(accountID string) (MuteState, bool) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return MuteState{}, false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	state, ok := p.mute[accountID]
	return state, ok && state.Muted
}

func (p *Pool) MutedAccountsDue(now time.Time) []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]string, 0)
	for _, id := range p.queue {
		state, ok := p.mute[id]
		if !ok || !state.Muted {
			continue
		}
		if !state.NextProbeAt.IsZero() && now.Before(state.NextProbeAt) {
			continue
		}
		out = append(out, id)
	}
	sort.SliceStable(out, func(i, j int) bool {
		left, right := p.mute[out[i]], p.mute[out[j]]
		return left.MutedUntil.Before(right.MutedUntil)
	})
	return out
}

func (p *Pool) AllMuted() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.allMutedLocked()
}

func (p *Pool) EarliestMutedUntil() (time.Time, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.earliestMutedUntilLocked()
}

func (p *Pool) allMutedLocked() bool {
	if len(p.queue) == 0 {
		return false
	}
	for _, id := range p.queue {
		state, ok := p.mute[id]
		if !ok || !state.Muted {
			return false
		}
	}
	return true
}

func (p *Pool) earliestMutedUntilLocked() (time.Time, bool) {
	var earliest time.Time
	for _, id := range p.queue {
		state, ok := p.mute[id]
		if !ok || !state.Muted {
			continue
		}
		until := state.MutedUntil
		if until.IsZero() {
			continue
		}
		if earliest.IsZero() || until.Before(earliest) {
			earliest = until
		}
	}
	return earliest, !earliest.IsZero()
}

func (p *Pool) isMutedLocked(accountID string) bool {
	state, ok := p.mute[accountID]
	return ok && state.Muted
}

func (p *Pool) mutedStatusLocked() []map[string]any {
	out := make([]map[string]any, 0, len(p.mute))
	for id, state := range p.mute {
		if !state.Muted {
			continue
		}
		out = append(out, map[string]any{
			"account":       id,
			"reason":        state.Reason,
			"muted_at":      unixOrZero(state.MutedAt),
			"muted_until":   unixOrZero(state.MutedUntil),
			"next_probe_at": unixOrZero(state.NextProbeAt),
			"last_probe_at": unixOrZero(state.LastProbeAt),
			"probe_count":   state.ProbeCount,
			"last_error":    state.LastError,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return int64FromAny(out[i]["muted_until"]) < int64FromAny(out[j]["muted_until"])
	})
	return out
}

func unixOrZero(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

func (p *Pool) loadMuteStateLocked() {
	if p.muteLoaded {
		return
	}
	p.muteLoaded = true
	if p.mutePath == "" {
		return
	}
	raw, err := os.ReadFile(p.mutePath)
	if err != nil {
		if !os.IsNotExist(err) {
			config.Logger.Warn("[account_health] read failed", "path", p.mutePath, "error", err)
		}
		return
	}
	loaded := map[string]MuteState{}
	if err := json.Unmarshal(raw, &loaded); err != nil {
		config.Logger.Warn("[account_health] parse failed", "path", p.mutePath, "error", err)
		return
	}
	for id, state := range loaded {
		if !state.Muted {
			continue
		}
		if state.MutedUntil.IsZero() {
			state.MutedUntil = time.Now().Add(defaultMuteDuration)
		}
		if state.NextProbeAt.IsZero() {
			state.NextProbeAt = state.MutedUntil.Add(muteProbeGrace)
		}
		p.mute[id] = state
	}
}

func (p *Pool) pruneMuteStateLocked() {
	if p.store == nil {
		return
	}
	known := map[string]struct{}{}
	for _, acc := range p.store.Accounts() {
		if id := acc.Identifier(); id != "" {
			known[id] = struct{}{}
		}
	}
	for id := range p.mute {
		if _, ok := known[id]; !ok {
			delete(p.mute, id)
		}
	}
}

func (p *Pool) saveMuteStateLocked() {
	if p.mutePath == "" {
		return
	}
	raw, err := json.MarshalIndent(p.mute, "", "  ")
	if err != nil {
		config.Logger.Warn("[account_health] marshal failed", "path", p.mutePath, "error", err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(p.mutePath), accountHealthDirPerm); err != nil {
		config.Logger.Warn("[account_health] mkdir failed", "path", p.mutePath, "error", err)
		return
	}
	tmp := p.mutePath + ".tmp"
	if err := os.WriteFile(tmp, raw, accountHealthFilePerm); err != nil {
		config.Logger.Warn("[account_health] write failed", "path", tmp, "error", err)
		return
	}
	if err := os.Rename(tmp, p.mutePath); err != nil {
		config.Logger.Warn("[account_health] rename failed", "path", p.mutePath, "error", err)
	}
}

func nextProbeAt(now, until time.Time) time.Time {
	if until.IsZero() {
		return now.Add(defaultMuteDuration + muteProbeGrace)
	}
	next := until.Add(muteProbeGrace)
	if next.Before(now) {
		if until.After(now) {
			return now.Add(muteProbeGrace)
		}
		return now
	}
	return next
}

func int64FromAny(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case float64:
		return int64(x)
	default:
		return 0
	}
}

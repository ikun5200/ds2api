package account

import "ds2api/internal/config"

// Swap transfers one request's lease without opening a gap in the global
// concurrency count. If the destination cannot be acquired, the old lease stays
// held. An empty accountID acquires an initial lease under the normal limits.
func (p *Pool) Swap(accountID, target string, exclude map[string]bool) (config.Account, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if accountID == "" {
		return p.acquireLocked(target, normalizeExclude(exclude))
	}
	count := p.inUse[accountID]
	if count <= 0 {
		return config.Account{}, false
	}
	if count == 1 {
		delete(p.inUse, accountID)
	} else {
		p.inUse[accountID] = count - 1
	}
	acc, ok := p.acquireLocked(target, normalizeExclude(exclude))
	if !ok {
		p.inUse[accountID] = count
		return config.Account{}, false
	}
	p.notifyWaiterLocked()
	return acc, true
}

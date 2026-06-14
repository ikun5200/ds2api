export async function readMutationResponse(res) {
    const contentType = String(res.headers.get('content-type') || '').toLowerCase()
    if (!contentType.includes('application/json')) {
        return {}
    }
    try {
        return await res.json()
    } catch (_err) {
        return {}
    }
}

export function mutationMessageType(data) {
    return data?.needs_vercel_sync ? 'warning' : 'success'
}

export function mutationMessage(data, fallback, syncFallback = '') {
    const localizedSyncMessage = String(syncFallback || '').trim()
    const syncMessage = String(data?.manual_sync_message || '').trim()
    if (data?.needs_vercel_sync && localizedSyncMessage) {
        return localizedSyncMessage
    }
    if (data?.needs_vercel_sync && syncMessage) {
        return syncMessage
    }
    return fallback
}

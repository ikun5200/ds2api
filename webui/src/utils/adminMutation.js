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
    return data?.env_backed ? 'warning' : 'success'
}

export function mutationMessage(data, fallback) {
    const syncMessage = String(data?.manual_sync_message || '').trim()
    if (syncMessage) {
        return syncMessage
    }
    return fallback
}

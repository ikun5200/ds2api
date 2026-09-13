// Keep upload credentials out of serializable file metadata and request bodies.
const attachmentCredentials = new WeakMap()

export function bindAttachedFileCredential(file, apiKey, usesManagedKey = false) {
    attachmentCredentials.set(file, {
        managed: usesManagedKey,
        key: usesManagedKey ? '' : apiKey,
    })
    return file
}

export function hasAttachmentCredentialMismatch(attachedFiles, apiKey, usesManagedKey = false) {
    return (attachedFiles || []).some(file => {
        const credential = attachmentCredentials.get(file)
        if (!credential) return false
        if (credential.managed && usesManagedKey) return false
        return credential.managed !== usesManagedKey || credential.key !== apiKey
    })
}

export function getAttachedFileAccountIds(attachedFiles = []) {
    const ids = []
    const seen = new Set()

    for (const file of attachedFiles || []) {
        const raw = file?.account_id ?? file?.accountId ?? file?.owner_account_id ?? file?.ownerAccountId ?? ''
        const id = String(raw).trim()
        if (!id || seen.has(id)) continue
        seen.add(id)
        ids.push(id)
    }

    return ids
}

export function getAttachedFileAccountId(attachedFiles = []) {
    const ids = getAttachedFileAccountIds(attachedFiles)
    return ids.length > 0 ? ids[0] : ''
}

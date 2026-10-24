import { useEffect, useState } from 'react'
import { X } from 'lucide-react'

export default function AddAccountModal({
    show,
    t,
    newAccount,
    setNewAccount,
    loading,
    onClose,
    onAdd,
    onFetchTokenInfo,
}) {
    const [resolvingToken, setResolvingToken] = useState(false)
    const [tokenInfo, setTokenInfo] = useState(null)

    useEffect(() => {
        if (!show) {
            setTokenInfo(null)
            setResolvingToken(false)
        }
    }, [show])

    if (!show) {
        return null
    }

    const resolveToken = async () => {
        const token = String(newAccount.token || '').trim()
        if (!token || !onFetchTokenInfo) return
        setResolvingToken(true)
        setTokenInfo(null)
        try {
            const info = await onFetchTokenInfo(token)
            setTokenInfo(info)
            setNewAccount(prev => ({
                ...prev,
                email: info.email || prev.email,
                mobile: info.mobile || prev.mobile,
                name: prev.name || info.suggested_name || '',
            }))
        } catch (e) {
            setTokenInfo({ error: e.message })
        } finally {
            setResolvingToken(false)
        }
    }

    const muteUntilText = tokenInfo?.mute_until
        ? new Date(tokenInfo.mute_until * 1000).toLocaleString()
        : t('accountManager.tokenMuteUnknown')

    return (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-background/70 backdrop-blur-sm p-4 animate-in fade-in">
            <div className="bg-card w-full max-w-md max-h-[calc(100dvh-2rem)] rounded-xl border border-border shadow-2xl overflow-y-auto animate-in zoom-in-95">
                <div className="p-4 border-b border-border flex justify-between items-center">
                    <h3 className="font-semibold">{t('accountManager.modalAddAccountTitle')}</h3>
                    <button onClick={onClose} className="text-muted-foreground hover:text-foreground">
                        <X className="w-5 h-5" />
                    </button>
                </div>
                <div className="p-6 space-y-4">
                    <div>
                        <label className="block text-sm font-medium mb-1.5">{t('accountManager.nameOptional')}</label>
                        <input
                            type="text"
                            className="input-field"
                            placeholder={t('accountManager.namePlaceholder')}
                            value={newAccount.name}
                            onChange={e => setNewAccount({ ...newAccount, name: e.target.value })}
                        />
                    </div>
                    <div>
                        <label className="block text-sm font-medium mb-1.5">{t('accountManager.remarkOptional')}</label>
                        <input
                            type="text"
                            className="input-field"
                            placeholder={t('accountManager.remarkPlaceholder')}
                            value={newAccount.remark}
                            onChange={e => setNewAccount({ ...newAccount, remark: e.target.value })}
                        />
                    </div>
                    <div>
                        <label className="block text-sm font-medium mb-1.5">{t('accountManager.emailOptional')}</label>
                        <input
                            type="email"
                            className="input-field"
                            placeholder="user@example.com"
                            value={newAccount.email}
                            onChange={e => setNewAccount({ ...newAccount, email: e.target.value })}
                        />
                    </div>
                    <div>
                        <label className="block text-sm font-medium mb-1.5">{t('accountManager.mobileOptional')}</label>
                        <input
                            type="text"
                            className="input-field"
                            placeholder="+86..."
                            value={newAccount.mobile}
                            onChange={e => setNewAccount({ ...newAccount, mobile: e.target.value })}
                        />
                    </div>
                    <div className="rounded-lg border border-border bg-background/70 p-3 space-y-2">
                        <div className="flex items-center justify-between gap-2">
                            <label className="block text-sm font-medium">{t('accountManager.tokenLabel')}</label>
                            <button
                                type="button"
                                onClick={resolveToken}
                                disabled={resolvingToken || !String(newAccount.token || '').trim()}
                                className="px-2.5 py-1 rounded-md border border-border text-xs font-medium hover:bg-secondary transition-colors disabled:opacity-50"
                            >
                                {resolvingToken ? t('accountManager.tokenResolving') : t('accountManager.tokenResolve')}
                            </button>
                        </div>
                        <input
                            type="password"
                            autoComplete="off"
                            className="input-field"
                            placeholder={t('accountManager.tokenPlaceholder')}
                            value={newAccount.token || ''}
                            onChange={e => {
                                setNewAccount({ ...newAccount, token: e.target.value })
                                setTokenInfo(null)
                            }}
                        />
                        <p className="text-xs text-muted-foreground">{t('accountManager.tokenHint')}</p>
                        {tokenInfo?.error && (
                            <p className="text-xs text-destructive break-all">{tokenInfo.error}</p>
                        )}
                        {tokenInfo && !tokenInfo.error && (
                            <div className="rounded-md bg-emerald-500/10 px-2 py-1.5 text-xs text-emerald-600 space-y-0.5">
                                <div>{t('accountManager.tokenResolved', { name: tokenInfo.suggested_name || tokenInfo.email || '-' })}</div>
                                {tokenInfo.email && <div className="break-all">{tokenInfo.email}</div>}
                                {tokenInfo.muted && (
                                    <div className="text-amber-600">{t('accountManager.tokenMuted', { time: muteUntilText })}</div>
                                )}
                            </div>
                        )}
                    </div>
                    <div>
                        <label className="block text-sm font-medium mb-1.5">
                            {t('accountManager.passwordLabel')}
                            {!String(newAccount.token || '').trim() && <span className="text-destructive"> *</span>}
                        </label>
                        <input
                            type="password"
                            className="input-field"
                            placeholder={t('accountManager.passwordPlaceholder')}
                            value={newAccount.password}
                            onChange={e => setNewAccount({ ...newAccount, password: e.target.value })}
                        />
                        <p className="mt-1.5 text-xs text-muted-foreground">{t('accountManager.passwordOptionalHint')}</p>
                    </div>
                    <div>
                        <label htmlFor="new-account-device" className="block text-sm font-medium mb-1.5">{t('accountManager.deviceIdLabel')}</label>
                        <input
                            id="new-account-device"
                            type="password"
                            autoComplete="off"
                            className="input-field"
                            placeholder={t('accountManager.deviceIdPlaceholder')}
                            value={newAccount.device_id || ''}
                            onChange={e => setNewAccount({ ...newAccount, device_id: e.target.value })}
                        />
                        <p className="mt-1.5 text-xs text-muted-foreground">
                            {String(newAccount.token || '').trim()
                                ? t('accountManager.tokenDeviceHint')
                                : t('accountManager.deviceIdHint')}
                        </p>
                    </div>
                    <label className="flex items-start gap-3 rounded-lg border border-border bg-background/70 p-3 text-sm">
                        <input
                            type="checkbox"
                            className="mt-0.5 h-4 w-4 rounded border-border text-primary focus:ring-ring"
                            checked={Boolean(newAccount.disabled)}
                            onChange={e => setNewAccount({ ...newAccount, disabled: e.target.checked })}
                        />
                        <span>
                            <span className="block font-medium">{t('accountManager.createDisabled')}</span>
                            <span className="block text-xs text-muted-foreground mt-0.5">{t('accountManager.createDisabledHint')}</span>
                        </span>
                    </label>
                    <div className="flex justify-end gap-2 pt-2">
                        <button onClick={onClose} className="px-4 py-2 rounded-lg border border-border hover:bg-secondary transition-colors text-sm font-medium">{t('actions.cancel')}</button>
                        <button onClick={onAdd} disabled={loading} className="px-4 py-2 bg-primary text-primary-foreground rounded-lg hover:bg-primary/90 transition-colors text-sm font-medium disabled:opacity-50">
                            {loading ? t('accountManager.addAccountLoading') : t('accountManager.addAccountAction')}
                        </button>
                    </div>
                </div>
            </div>
        </div>
    )
}

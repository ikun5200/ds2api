import { X } from 'lucide-react'

export default function AddAccountModal({
    show,
    t,
    newAccount,
    setNewAccount,
    loading,
    onClose,
    onAdd,
}) {
    if (!show) {
        return null
    }

    return (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-background/70 backdrop-blur-sm p-4 animate-in fade-in">
            <div className="bg-card w-full max-w-md rounded-xl border border-border shadow-2xl overflow-hidden animate-in zoom-in-95">
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
                    <div>
                        <label className="block text-sm font-medium mb-1.5">{t('accountManager.passwordLabel')} <span className="text-destructive">*</span></label>
                        <input
                            type="password"
                            className="input-field"
                            placeholder={t('accountManager.passwordPlaceholder')}
                            value={newAccount.password}
                            onChange={e => setNewAccount({ ...newAccount, password: e.target.value })}
                        />
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

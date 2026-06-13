import { CheckCircle2, Server, ShieldCheck } from 'lucide-react'

export default function QueueCards({ queueStatus, t }) {
    if (!queueStatus) {
        return null
    }
    const configuredTotal = queueStatus.configured_total ?? queueStatus.total
    const disabledTotal = queueStatus.disabled ?? Math.max(0, configuredTotal - queueStatus.total)

    return (
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div className="bg-card border border-border rounded-xl p-4 flex flex-col justify-between shadow-sm relative overflow-hidden group">
                <div className="absolute right-0 top-0 p-4 opacity-5 group-hover:opacity-10 transition-opacity">
                    <CheckCircle2 className="w-16 h-16" />
                </div>
                <p className="text-xs font-medium text-muted-foreground uppercase tracking-widest">{t('accountManager.available')}</p>
                <div className="mt-2 flex items-baseline gap-2">
                    <span className="text-3xl font-bold text-foreground">{queueStatus.available}</span>
                    <span className="text-xs text-muted-foreground">{t('accountManager.accountsUnit')}</span>
                </div>
            </div>
            <div className="bg-card border border-border rounded-xl p-4 flex flex-col justify-between shadow-sm relative overflow-hidden group">
                <div className="absolute right-0 top-0 p-4 opacity-5 group-hover:opacity-10 transition-opacity">
                    <Server className="w-16 h-16" />
                </div>
                <p className="text-xs font-medium text-muted-foreground uppercase tracking-widest">{t('accountManager.inUse')}</p>
                <div className="mt-2 flex items-baseline gap-2">
                    <span className="text-3xl font-bold text-foreground">{queueStatus.in_use}</span>
                    <span className="text-xs text-muted-foreground">{t('accountManager.threadsUnit')}</span>
                </div>
            </div>
            <div className="bg-card border border-border rounded-xl p-4 flex flex-col justify-between shadow-sm relative overflow-hidden group">
                <div className="absolute right-0 top-0 p-4 opacity-5 group-hover:opacity-10 transition-opacity">
                    <ShieldCheck className="w-16 h-16" />
                </div>
                <p className="text-xs font-medium text-muted-foreground uppercase tracking-widest">{t('accountManager.totalPool')}</p>
                <div className="mt-2 flex items-baseline gap-2">
                    <span className="text-3xl font-bold text-foreground">{queueStatus.total}</span>
                    <span className="text-xs text-muted-foreground">{t('accountManager.accountsUnit')}</span>
                </div>
                {configuredTotal !== queueStatus.total && (
                    <p className="mt-2 text-xs text-muted-foreground">
                        {t('accountManager.configuredAccounts', { total: configuredTotal, disabled: disabledTotal })}
                    </p>
                )}
            </div>
        </div>
    )
}

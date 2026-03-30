export default function BehaviorSection({ t, form, setForm }) {
    return (
        <div className="bg-card border border-border rounded-xl p-5 space-y-4">
            <h3 className="font-semibold">{t('settings.behaviorTitle')}</h3>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <label className="text-sm space-y-2">
                    <span className="text-muted-foreground">{t('settings.responsesTTL')}</span>
                    <input
                        type="number"
                        min={30}
                        value={form.responses.store_ttl_seconds}
                        onChange={(e) => setForm((prev) => ({
                            ...prev,
                            responses: { ...prev.responses, store_ttl_seconds: Number(e.target.value || 30) },
                        }))}
                        className="w-full bg-background border border-border rounded-lg px-3 py-2"
                    />
                </label>
                <label className="text-sm space-y-2">
                    <span className="text-muted-foreground">{t('settings.embeddingsProvider')}</span>
                    <input
                        type="text"
                        value={form.embeddings.provider}
                        onChange={(e) => setForm((prev) => ({
                            ...prev,
                            embeddings: { ...prev.embeddings, provider: e.target.value },
                        }))}
                        className="w-full bg-background border border-border rounded-lg px-3 py-2"
                    />
                </label>
            </div>
        </div>
    )
}

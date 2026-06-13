import { Brain, Github, LayoutDashboard, Radio, Search, Scale, Workflow } from 'lucide-react'
import { useI18n } from '../i18n'
import LanguageToggle from './LanguageToggle'

const featureIcons = {
    compatibility: Workflow,
    loadBalancing: Scale,
    reasoning: Brain,
    search: Search,
}

export default function LandingPage({ onEnter }) {
    const { t } = useI18n()
    const features = [
        { key: 'compatibility', title: t('landing.features.compatibility.title'), desc: t('landing.features.compatibility.desc') },
        { key: 'loadBalancing', title: t('landing.features.loadBalancing.title'), desc: t('landing.features.loadBalancing.desc') },
        { key: 'reasoning', title: t('landing.features.reasoning.title'), desc: t('landing.features.reasoning.desc') },
        { key: 'search', title: t('landing.features.search.title'), desc: t('landing.features.search.desc') },
    ]

    return (
        <div className="min-h-screen bg-background text-foreground">
            <header className="mx-auto flex w-full max-w-6xl items-center justify-between px-6 py-5">
                <div className="flex items-center gap-3">
                    <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-primary text-primary-foreground">
                        <LayoutDashboard className="h-5 w-5" />
                    </div>
                    <div>
                        <div className="text-sm font-semibold tracking-wide">DS2API</div>
                        <div className="text-xs text-muted-foreground">Admin Console</div>
                    </div>
                </div>
                <LanguageToggle />
            </header>

            <main className="mx-auto grid min-h-[calc(100vh-80px)] w-full max-w-6xl content-center gap-8 px-6 pb-10 lg:grid-cols-[1fr_360px] lg:items-center">
                <section className="max-w-3xl">
                    <p className="mb-4 text-sm font-semibold uppercase tracking-[0.18em] text-primary">DeepSeek Gateway</p>
                    <h1 className="text-4xl font-semibold tracking-tight text-foreground sm:text-5xl">DS2API</h1>
                    <p className="mt-4 max-w-2xl text-base leading-7 text-muted-foreground">
                        DeepSeek to OpenAI & Claude Compatible API Interface
                    </p>
                    <div className="mt-8 flex flex-wrap gap-3">
                        <button
                            onClick={onEnter}
                            className="inline-flex h-10 items-center gap-2 rounded-lg bg-primary px-4 text-sm font-semibold text-primary-foreground shadow-sm transition-colors hover:bg-primary/90"
                        >
                            <LayoutDashboard className="h-4 w-4" />
                            {t('landing.adminConsole')}
                        </button>
                        <a
                            href="/v1/models"
                            target="_blank"
                            rel="noreferrer"
                            className="inline-flex h-10 items-center gap-2 rounded-lg border border-border bg-card px-4 text-sm font-medium text-foreground transition-colors hover:bg-secondary/70"
                        >
                            <Radio className="h-4 w-4" />
                            {t('landing.apiStatus')}
                        </a>
                        <a
                            href="https://github.com/CJackHwang/ds2api"
                            target="_blank"
                            rel="noreferrer"
                            className="inline-flex h-10 items-center gap-2 rounded-lg border border-border bg-card px-4 text-sm font-medium text-foreground transition-colors hover:bg-secondary/70"
                        >
                            <Github className="h-4 w-4" />
                            GitHub
                        </a>
                    </div>
                </section>

                <section className="grid gap-3">
                    {features.map(feature => {
                        const Icon = featureIcons[feature.key]
                        return (
                            <div key={feature.key} className="rounded-lg border border-border bg-card p-4 shadow-sm">
                                <div className="flex items-start gap-3">
                                    <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-secondary text-primary">
                                        <Icon className="h-4 w-4" />
                                    </div>
                                    <div>
                                        <h2 className="text-sm font-semibold">{feature.title}</h2>
                                        <p className="mt-1 text-sm leading-6 text-muted-foreground">{feature.desc}</p>
                                    </div>
                                </div>
                            </div>
                        )
                    })}
                </section>
            </main>
        </div>
    )
}

import { ArrowLeft, BookOpen, Boxes, CircleGauge, ExternalLink, GitBranch, KeyRound, Workflow } from 'lucide-react'
import { useI18n } from '../i18n'
import LanguageToggle from './LanguageToggle'

const repo = 'https://github.com/ikun5200/ds2api'

export default function DocsPage() {
    const { t } = useI18n()
    const flow = [
        { icon: Workflow, title: t('docs.flow.client.title'), desc: t('docs.flow.client.desc') },
        { icon: GitBranch, title: t('docs.flow.adapter.title'), desc: t('docs.flow.adapter.desc') },
        { icon: Boxes, title: t('docs.flow.runtime.title'), desc: t('docs.flow.runtime.desc') },
        { icon: CircleGauge, title: t('docs.flow.response.title'), desc: t('docs.flow.response.desc') },
    ]
    const links = [
        { title: t('docs.links.deploy.title'), desc: t('docs.links.deploy.desc'), href: `${repo}/blob/main/docs/DEPLOY.md` },
        { title: t('docs.links.api.title'), desc: t('docs.links.api.desc'), href: `${repo}/blob/main/API.md` },
        { title: t('docs.links.arch.title'), desc: t('docs.links.arch.desc'), href: `${repo}/blob/main/docs/ARCHITECTURE.md` },
        { title: t('docs.links.config.title'), desc: t('docs.links.config.desc'), href: `${repo}/blob/main/config.example.json` },
    ]

    return (
        <div className="min-h-screen bg-background text-foreground">
            <header className="mx-auto flex w-full max-w-6xl flex-wrap items-center justify-between gap-4 px-6 py-5">
                <a href="/" className="inline-flex items-center gap-2 text-sm font-semibold text-muted-foreground hover:text-foreground">
                    <ArrowLeft className="h-4 w-4" />
                    {t('docs.back')}
                </a>
                <LanguageToggle />
            </header>

            <main className="mx-auto w-full max-w-6xl px-6 pb-12">
                <section className="grid gap-8 py-8 lg:grid-cols-[1fr_360px] lg:items-center">
                    <div>
                        <div className="mb-4 inline-flex items-center gap-2 rounded-full bg-secondary px-3 py-1 text-xs font-semibold text-primary">
                            <BookOpen className="h-3.5 w-3.5" />
                            {t('docs.badge')}
                        </div>
                        <h1 className="text-4xl font-semibold tracking-tight sm:text-5xl">{t('docs.title')}</h1>
                        <p className="mt-4 max-w-2xl text-base leading-7 text-muted-foreground">{t('docs.subtitle')}</p>
                        <div className="mt-7 flex flex-wrap gap-3">
                            <a
                                href={`${repo}#readme`}
                                target="_blank"
                                rel="noreferrer"
                                className="inline-flex h-10 items-center gap-2 rounded-lg bg-primary px-4 text-sm font-semibold text-primary-foreground"
                            >
                                <ExternalLink className="h-4 w-4" />
                                {t('docs.openRepo')}
                            </a>
                            <a
                                href="/admin"
                                className="inline-flex h-10 items-center gap-2 rounded-lg border border-border bg-card px-4 text-sm font-medium"
                            >
                                <KeyRound className="h-4 w-4" />
                                {t('docs.openAdmin')}
                            </a>
                        </div>
                    </div>

                    <div className="rounded-lg border border-border bg-card p-5 shadow-sm">
                        <h2 className="text-base font-semibold">{t('docs.reading.title')}</h2>
                        <ol className="mt-4 grid gap-3 text-sm leading-6 text-muted-foreground">
                            <li>{t('docs.reading.deploy')}</li>
                            <li>{t('docs.reading.config')}</li>
                            <li>{t('docs.reading.client')}</li>
                        </ol>
                    </div>
                </section>

                <section className="rounded-lg border border-border bg-card p-5 shadow-sm">
                    <h2 className="text-lg font-semibold">{t('docs.flowTitle')}</h2>
                    <div className="mt-5 grid gap-3 md:grid-cols-4">
                        {flow.map((item, idx) => {
                            const Icon = item.icon
                            return (
                                <div key={item.title} className="relative rounded-lg border border-border bg-background p-4">
                                    <div className="mb-3 flex h-9 w-9 items-center justify-center rounded-lg bg-secondary text-primary">
                                        <Icon className="h-4 w-4" />
                                    </div>
                                    <div className="text-xs font-semibold text-primary">0{idx + 1}</div>
                                    <h3 className="mt-1 text-sm font-semibold">{item.title}</h3>
                                    <p className="mt-2 text-sm leading-6 text-muted-foreground">{item.desc}</p>
                                </div>
                            )
                        })}
                    </div>
                </section>

                <section className="mt-4 grid gap-3 md:grid-cols-2">
                    {links.map(link => (
                        <a
                            key={link.href}
                            href={link.href}
                            target="_blank"
                            rel="noreferrer"
                            className="rounded-lg border border-border bg-card p-5 shadow-sm transition-colors hover:bg-secondary/50"
                        >
                            <div className="flex items-start justify-between gap-4">
                                <div>
                                    <h3 className="text-sm font-semibold">{link.title}</h3>
                                    <p className="mt-2 text-sm leading-6 text-muted-foreground">{link.desc}</p>
                                </div>
                                <ExternalLink className="h-4 w-4 shrink-0 text-primary" />
                            </div>
                        </a>
                    ))}
                </section>
            </main>
        </div>
    )
}

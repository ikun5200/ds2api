package webui

const welcomeHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>DS2API</title>
<style>
:root{color-scheme:light;--bg:#f6f8fb;--fg:#101828;--muted:#5b677a;--primary:#1d4ed8;--accent:#0f766e;--border:#d8e0ea;--card:#fff}
*{box-sizing:border-box}
body{margin:0;min-height:100vh;font-family:Inter,ui-sans-serif,system-ui,-apple-system,
BlinkMacSystemFont,"Segoe UI",sans-serif;background:var(--bg);color:var(--fg)}
main{min-height:100vh;display:grid;place-items:center;padding:32px}
.shell{width:min(980px,100%);display:grid;grid-template-columns:1.2fr .8fr;gap:28px;align-items:center}
.eyebrow{margin:0 0 12px;color:var(--primary);font-size:13px;font-weight:700;letter-spacing:0}
h1{margin:0;font-size:64px;line-height:.98;letter-spacing:0}
p{color:var(--muted);font-size:17px;line-height:1.7}
.actions{display:flex;flex-wrap:wrap;gap:12px;margin-top:28px}
a{display:inline-flex;min-height:42px;align-items:center;justify-content:center;
border:1px solid var(--border);border-radius:8px;padding:10px 14px;color:var(--fg);
font-weight:700;text-decoration:none;background:var(--card);overflow-wrap:anywhere}
a.primary{background:var(--primary);border-color:var(--primary);color:#fff}
a.accent{background:var(--accent);border-color:var(--accent);color:#fff}
.panel{border:1px solid var(--border);border-radius:8px;background:var(--card);padding:22px;box-shadow:0 16px 40px rgba(16,24,40,.08)}
.flow{display:grid;gap:12px}
.node{border:1px solid var(--border);border-radius:8px;padding:14px;background:#f8fafc}
.node strong{display:block;color:var(--fg);font-size:15px}
.node span{display:block;margin-top:4px;color:var(--muted);font-size:13px;line-height:1.5}
.arrow{color:var(--accent);font-weight:800;text-align:center}
@media (max-width:820px){
main{place-items:start;padding:22px}
.shell{grid-template-columns:1fr;gap:20px}
h1{font-size:48px}
.panel{order:-1}
}
@media (max-width:560px){
main{padding:18px}
h1{font-size:40px}
p{font-size:15px}
.actions{display:grid;grid-template-columns:1fr;gap:10px;margin-top:22px}
a{width:100%;min-height:44px}
.panel{padding:16px}
.node{padding:12px}
}
</style>
</head>
<body>
<main>
<div class="shell">
<section>
<p class="eyebrow">DeepSeek 兼容网关</p>
<h1>DS2API</h1>
<p>把 DeepSeek Web 对话能力转换为 OpenAI、Claude 与 Gemini 兼容 API。
这里可以进入管理面板、查看接口状态，也可以先阅读中文图形化文档。</p>
<div class="actions">
<a class="primary" href="/admin">管理面板</a>
<a class="accent" href="/docs">图形化文档</a>
<a href="/v1/models" target="_blank" rel="noreferrer">API 状态</a>
<a href="https://github.com/ikun5200/ds2api" target="_blank" rel="noreferrer">GitHub</a>
</div>
</section>
<aside class="panel" aria-label="DS2API 请求链路">
<div class="flow">
<div class="node"><strong>客户端 / SDK</strong><span>OpenAI、Claude、Gemini 或兼容客户端统一请求 DS2API。</span></div>
<div class="arrow">↓</div>
<div class="node"><strong>协议适配层</strong><span>请求先归一化为共享对话模型，再进入统一运行时。</span></div>
<div class="arrow">↓</div>
<div class="node"><strong>DeepSeek 会话</strong><span>账号池、PoW、文件上传和流式响应由后端统一处理。</span></div>
<div class="arrow">↓</div>
<div class="node"><strong>响应记录存储</strong><span>Chat history 可使用内置 JSON，也可切换到 PostgreSQL、MySQL 或 MariaDB。</span></div>
<div class="arrow">↓</div>
<div class="node"><strong>兼容响应</strong><span>最终按目标协议返回内容、用量、状态和工具调用结果。</span></div>
</div>
</aside>
</div>
</main>
</body>
</html>`

const docsHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>DS2API 图形化文档</title>
<style>
:root{color-scheme:light;--bg:#f6f8fb;--fg:#101828;--muted:#5b677a;
--primary:#1d4ed8;--accent:#0f766e;--warn:#b45309;--border:#d8e0ea;--card:#fff}
*{box-sizing:border-box}
body{margin:0;font-family:Inter,ui-sans-serif,system-ui,-apple-system,
BlinkMacSystemFont,"Segoe UI",sans-serif;background:var(--bg);color:var(--fg)}
a{color:inherit;text-decoration:none}
.page{width:min(1180px,100%);margin:0 auto;padding:28px}
header{display:flex;align-items:center;justify-content:space-between;gap:16px;margin-bottom:28px}
.brand{display:flex;align-items:center;gap:12px}
.mark{display:grid;width:42px;height:42px;place-items:center;border-radius:8px;background:var(--primary);color:#fff;font-weight:800}
.nav{display:flex;flex-wrap:wrap;gap:10px}
.button{display:inline-flex;min-height:40px;align-items:center;border:1px solid var(--border);
border-radius:8px;padding:9px 13px;background:var(--card);font-weight:700;justify-content:center;overflow-wrap:anywhere}
.button.primary{background:var(--primary);border-color:var(--primary);color:#fff}
.hero{display:grid;grid-template-columns:1fr .9fr;gap:24px;align-items:center;margin-bottom:26px}
h1{margin:0;font-size:48px;line-height:1.08;letter-spacing:0}
p{color:var(--muted);line-height:1.7}
.grid{display:grid;gap:14px}
.grid.cols-3{grid-template-columns:repeat(3,1fr)}
.grid.cols-2{grid-template-columns:repeat(2,1fr)}
.card{border:1px solid var(--border);border-radius:8px;background:var(--card);padding:18px;box-shadow:0 10px 28px rgba(16,24,40,.06)}
.card h2,.card h3{margin:0;color:var(--fg)}
.card h2{font-size:18px}
.card h3{font-size:15px}
.badge{display:inline-flex;align-items:center;border-radius:999px;padding:4px 9px;
font-size:12px;font-weight:800;background:#e0f2fe;color:#075985}
.flow{display:grid;grid-template-columns:repeat(4,1fr);gap:10px;align-items:stretch}
.step{position:relative;border:1px solid var(--border);border-radius:8px;background:#fff;padding:16px;min-height:132px}
.step strong{display:block;margin-bottom:8px;color:var(--fg)}
.step span{display:block;color:var(--muted);font-size:14px;line-height:1.5}
.step:not(:last-child)::after{content:"";position:absolute;right:-9px;top:50%;width:8px;height:8px;
border-top:2px solid var(--accent);border-right:2px solid var(--accent);
transform:translateY(-50%) rotate(45deg)}
.matrix{display:grid;grid-template-columns:repeat(3,1fr);gap:12px}
.mini{border-left:4px solid var(--primary)}
.mini:nth-child(2){border-left-color:var(--accent)}
.mini:nth-child(3){border-left-color:var(--warn)}
ul{margin:12px 0 0;padding-left:18px;color:var(--muted);line-height:1.7}
.links{display:grid;grid-template-columns:repeat(2,1fr);gap:12px}
.doclink{display:block;border:1px solid var(--border);border-radius:8px;background:#fff;padding:15px}
.doclink strong{display:block;color:var(--fg)}
.doclink span{display:block;margin-top:4px;color:var(--muted);font-size:14px;line-height:1.5}
.code{font-family:ui-monospace,SFMono-Regular,Menlo,Monaco,Consolas,monospace;font-size:13px;
background:#eef2f7;border-radius:6px;padding:2px 5px;color:#334155;word-break:break-word}
.note{border-left:4px solid var(--accent)}
@media (max-width:900px){
.page{padding:22px}
.hero,.grid.cols-2,.grid.cols-3,.flow,.matrix,.links{grid-template-columns:1fr}
.step:not(:last-child)::after{display:none}
header{align-items:flex-start;flex-direction:column}
.nav{width:100%;display:grid;grid-template-columns:repeat(3,1fr)}
}
@media (max-width:560px){
.page{padding:16px}
header{gap:14px;margin-bottom:20px}
.brand{align-items:flex-start}
.nav{grid-template-columns:1fr}
.button{width:100%;min-height:44px}
.hero{gap:16px;margin-bottom:18px}
h1{font-size:34px}
p{font-size:15px}
.card{padding:15px}
.step{min-height:auto;padding:14px}
.doclink{padding:14px}
}
</style>
</head>
<body>
<div class="page">
<header>
<a class="brand" href="/">
<span class="mark">D</span>
<span><strong>DS2API</strong><br><span style="color:var(--muted);font-size:13px">图形化文档</span></span>
</a>
<nav class="nav">
<a class="button" href="/">首页</a>
<a class="button" href="/admin">管理面板</a>
<a class="button primary" href="https://github.com/ikun5200/ds2api" target="_blank" rel="noreferrer">GitHub 仓库</a>
</nav>
</header>

<section class="hero">
<div>
<span class="badge">图形化导览</span>
<h1>从部署到调用，一屏看清 DS2API</h1>
<p>这页保留常用路径、请求链路和外部存储入口。需要长期保存响应记录时，请优先查看外部存储说明。</p>
</div>
<div class="card">
<h2>推荐阅读顺序</h2>
<ul>
<li>先看部署指南，确认 Docker、Vercel 或源码运行方式。</li>
<li>再看外部存储说明，决定使用内置 JSON 还是外部数据库。</li>
<li>最后按接口文档接入 OpenAI、Claude 或 Gemini 客户端。</li>
</ul>
</div>
</section>

<section class="card">
<h2>请求链路</h2>
<p>DS2API 将不同协议的请求归一化，再复用同一套账号池、会话和流式处理逻辑。</p>
<div class="flow">
<div class="step"><strong>1. 客户端</strong><span>OpenAI / Claude / Gemini SDK 发起兼容请求。</span></div>
<div class="step"><strong>2. 协议适配</strong><span>协议层转换为统一请求模型，
保留工具调用和历史上下文。</span></div>
<div class="step"><strong>3. 运行时</strong><span>账号池、PoW、上传文件和流式解析统一处理。</span></div>
<div class="step"><strong>4. 兼容响应</strong><span>按目标协议渲染返回，包含用量、状态和工具调用结果。</span></div>
</div>
</section>

<section class="grid cols-3" style="margin-top:14px">
<div class="card mini"><h3>部署入口</h3><p>Docker、Vercel、Zeabur 和源码运行路径集中在部署指南。</p></div>
<div class="card mini"><h3>管理面板</h3><p>上线后进入 /admin 管理账号、API Key、代理、设置和响应记录。</p></div>
<div class="card mini"><h3>兼容接口</h3><p>/v1/chat/completions、/v1/responses、/anthropic 和 Gemini 路径共存。</p></div>
</section>

<section class="grid cols-2" style="margin-top:14px">
<div class="card note">
<h2>外部存储怎么选</h2>
<p>DS2API 默认用内置 JSON 文件保存 Chat history。需要跨冷启动、跨容器或多实例共享响应记录时，可以切换到外部 SQL 数据库。</p>
<ul>
<li>支持 PostgreSQL、MySQL 和 MariaDB。</li>
<li>至少设置 <span class="code">DS2API_DATABASE_MODE=external</span>、<span class="code">DS2API_DATABASE_TYPE</span> 和 <span class="code">DS2API_DATABASE_DSN</span>。</li>
<li>外部存储只保存 Chat history，不保存账号、API Key、代理、Admin 密码或运行时设置。</li>
</ul>
</div>
<div class="card">
<h2>常用连接串示例</h2>
<ul>
<li>PostgreSQL：<span class="code">postgres://user:pass@host:5432/ds2api?sslmode=disable</span></li>
<li>MySQL / MariaDB：<span class="code">user:pass@tcp(host:3306)/ds2api?parseTime=true</span></li>
<li>多套实例共用同一数据库时，用 <span class="code">DS2API_DATABASE_TABLE_PREFIX</span> 隔离表名。</li>
</ul>
</div>
</section>

<section class="grid cols-2" style="margin-top:14px">
<div class="card">
<h2>常用 GitHub 文档</h2>
<div class="links">
<a class="doclink" href="https://github.com/ikun5200/ds2api/blob/main/docs/DEPLOY.md" target="_blank" rel="noreferrer">
<strong>部署指南</strong><span>Docker、Vercel、Zeabur、源码运行与常见问题。</span></a>
<a class="doclink" href="https://github.com/ikun5200/ds2api/blob/main/docs/DEPLOY.md#321-外部存储chat-history详细说明" target="_blank" rel="noreferrer">
<strong>外部存储说明</strong><span>PostgreSQL、MySQL、MariaDB、自动建表和环境变量示例。</span></a>
<a class="doclink" href="https://github.com/ikun5200/ds2api/blob/main/API.md" target="_blank" rel="noreferrer">
<strong>接口文档</strong><span>鉴权、模型、Chat、Responses、Claude、Gemini 与 Admin API。</span></a>
<a class="doclink" href="https://github.com/ikun5200/ds2api/blob/main/docs/ARCHITECTURE.md" target="_blank" rel="noreferrer">
<strong>架构说明</strong><span>模块边界、协议适配、运行时和 WebUI 的关系。</span></a>
<a class="doclink" href="https://github.com/ikun5200/ds2api/blob/main/config.example.json" target="_blank" rel="noreferrer">
<strong>配置示例</strong><span>API Keys、账号池、模型别名、并发和自动删除配置。</span></a>
</div>
</div>
<div class="card">
<h2>快速检查</h2>
<ul>
<li>打开 <strong>/v1/models</strong> 确认 API 服务可访问。</li>
<li>打开 <strong>/admin</strong> 登录管理台并导入配置。</li>
<li>按 GitHub 文档设置客户端 base URL 和 API Key。</li>
<li>遇到部署问题时优先查看 GitHub Issues 和部署指南。</li>
</ul>
</div>
</section>
</div>
</body>
</html>`

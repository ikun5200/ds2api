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
.eyebrow{margin:0 0 12px;color:var(--primary);font-size:13px;font-weight:700;letter-spacing:.14em;text-transform:uppercase}
h1{margin:0;font-size:clamp(42px,7vw,76px);line-height:.95;letter-spacing:0}
p{color:var(--muted);font-size:17px;line-height:1.7}
.actions{display:flex;flex-wrap:wrap;gap:12px;margin-top:28px}
a{display:inline-flex;min-height:42px;align-items:center;justify-content:center;
border:1px solid var(--border);border-radius:8px;padding:10px 14px;color:var(--fg);
font-weight:700;text-decoration:none;background:var(--card)}
a.primary{background:var(--primary);border-color:var(--primary);color:#fff}
a.accent{background:var(--accent);border-color:var(--accent);color:#fff}
.panel{border:1px solid var(--border);border-radius:8px;background:var(--card);padding:22px;box-shadow:0 16px 40px rgba(16,24,40,.08)}
.flow{display:grid;gap:12px}
.node{border:1px solid var(--border);border-radius:8px;padding:14px;background:#f8fafc}
.node strong{display:block;color:var(--fg);font-size:15px}
.node span{display:block;margin-top:4px;color:var(--muted);font-size:13px;line-height:1.5}
.arrow{color:var(--accent);font-weight:800;text-align:center}
@media (max-width:820px){main{place-items:start}.shell{grid-template-columns:1fr}.panel{order:-1}}
</style>
</head>
<body>
<main>
<div class="shell">
<section>
<p class="eyebrow">DeepSeek Gateway</p>
<h1>DS2API</h1>
<p>DeepSeek to OpenAI, Claude and Gemini compatible API. Use this entry page to open the admin console,
check the API, or browse a visual guide before deployment work.</p>
<div class="actions">
<a class="primary" href="/admin">管理面板</a>
<a class="accent" href="/docs">图形化文档</a>
<a href="/v1/models" target="_blank" rel="noreferrer">API 状态</a>
<a href="https://github.com/ikun5200/ds2api" target="_blank" rel="noreferrer">GitHub</a>
</div>
</section>
<aside class="panel" aria-label="DS2API request flow">
<div class="flow">
<div class="node"><strong>Client / SDK</strong><span>OpenAI, Claude, Gemini or compatible clients call DS2API.</span></div>
<div class="arrow">↓</div>
<div class="node"><strong>Protocol adapter</strong><span>Requests are normalized into the shared conversation model.</span></div>
<div class="arrow">↓</div>
<div class="node"><strong>DeepSeek session</strong><span>Account pool, PoW and streaming runtime handle upstream calls.</span></div>
<div class="arrow">↓</div>
<div class="node"><strong>Compatible response</strong><span>DS2API renders back into the target API format.</span></div>
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
border-radius:8px;padding:9px 13px;background:var(--card);font-weight:700}
.button.primary{background:var(--primary);border-color:var(--primary);color:#fff}
.hero{display:grid;grid-template-columns:1fr .9fr;gap:24px;align-items:center;margin-bottom:26px}
h1{margin:0;font-size:clamp(34px,5vw,56px);line-height:1.05;letter-spacing:0}
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
@media (max-width:900px){.hero,.grid.cols-2,.grid.cols-3,.flow,.matrix,.links{grid-template-columns:1fr}
.step:not(:last-child)::after{display:none}header{align-items:flex-start;flex-direction:column}}
</style>
</head>
<body>
<div class="page">
<header>
<a class="brand" href="/">
<span class="mark">D</span>
<span><strong>DS2API</strong><br><span style="color:var(--muted);font-size:13px">Visual documentation</span></span>
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
<p>这页保留常用路径和架构视图，完整细节直接引用 GitHub 仓库文档，避免部署包里写死长篇说明。</p>
</div>
<div class="card">
<h2>推荐阅读顺序</h2>
<ul>
<li>先看部署指南，确认 Docker、Vercel 或源码运行方式。</li>
<li>再看配置示例，准备 API Key、账号池和并发限制。</li>
<li>最后按接口文档接入 OpenAI、Claude 或 Gemini 客户端。</li>
</ul>
</div>
</section>

<section class="card">
<h2>请求链路</h2>
<p>DS2API 将不同协议的请求归一化，再复用同一套账号池、会话和流式处理逻辑。</p>
<div class="flow">
<div class="step"><strong>1. Client</strong><span>OpenAI / Claude / Gemini SDK 发起兼容请求。</span></div>
<div class="step"><strong>2. Adapter</strong><span>协议层转换为统一请求模型，
保留工具调用和历史上下文。</span></div>
<div class="step"><strong>3. Runtime</strong><span>账号池、PoW、上传文件和流式解析统一处理。</span></div>
<div class="step"><strong>4. Response</strong><span>按目标协议渲染返回，包含用量、状态和工具调用结果。</span></div>
</div>
</section>

<section class="grid cols-3" style="margin-top:14px">
<div class="card mini"><h3>部署入口</h3><p>Docker、Vercel、Zeabur 和源码运行路径集中在部署指南。</p></div>
<div class="card mini"><h3>管理面板</h3><p>上线后进入 /admin 管理账号、API Key、代理、设置和响应记录。</p></div>
<div class="card mini"><h3>兼容接口</h3><p>/v1/chat/completions、/v1/responses、/anthropic 和 Gemini 路径共存。</p></div>
</section>

<section class="grid cols-2" style="margin-top:14px">
<div class="card">
<h2>常用 GitHub 文档</h2>
<div class="links">
<a class="doclink" href="https://github.com/ikun5200/ds2api/blob/main/docs/DEPLOY.md" target="_blank" rel="noreferrer">
<strong>部署指南</strong><span>Docker、Vercel、Zeabur、源码运行与常见问题。</span></a>
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

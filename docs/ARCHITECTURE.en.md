# DS2API Architecture & Project Layout

Language: [中文](ARCHITECTURE.md) | [English](ARCHITECTURE.en.md)

> This file is the single architecture source for directory layout, module boundaries, and execution flow.

## 1. Top-level Layout (core directories)

> Notes: this lists the main business directories (excluding metadata/dependency dirs such as `.git/` and `webui/node_modules/`), with each folder annotated by purpose. Newly added directories should be verified from the code tree rather than treated as a per-file inventory here.

```text
ds2api/
├── .github/                              # GitHub collaboration and CI config
│   ├── ISSUE_TEMPLATE/                   # Issue templates
│   └── workflows/                        # GitHub Actions workflows
├── app/                                  # Application-level handler assembly
├── artifacts/                            # Debug artifacts (raw-stream-sim, stream-debug, etc.)
├── cmd/                                  # Executable entrypoints
│   ├── ds2api/                           # Main service bootstrap
│   └── ds2api-tests/                     # E2E testsuite CLI bootstrap
├── docs/                                 # Project documentation
├── internal/                             # Core implementation (non-public packages)
│   ├── account/                          # Account pool, inflight slots, waiting queue
│   ├── auth/                             # Auth/JWT/credential resolution
│   ├── chathistory/                      # Server-side conversation history storage/query
│   ├── compat/                           # Compatibility and regression helpers
│   ├── assistantturn/                    # Upstream output to canonical assistant turn / stream event semantics
│   ├── completionruntime/                # Shared Go DeepSeek completion startup, collection, empty-output/account-switch retry
│   ├── config/                           # Config loading/validation/hot reload
│   ├── deepseek/                         # DeepSeek upstream client/protocol/transport
│   │   ├── client/                       # Login/session/completion/upload/delete calls
│   │   ├── protocol/                     # DeepSeek URLs, constants, skip path/pattern
│   │   └── transport/                    # DeepSeek transport details
│   ├── devcapture/                       # Dev capture and troubleshooting
│   ├── format/                           # Response formatting layer
│   │   └── openai/                       # OpenAI output formatting
│   ├── httpapi/                          # HTTP surfaces: OpenAI/Ollama/Admin
│   │   ├── admin/                        # Admin API root assembly and resource packages
│   │   ├── ollama/                       # Ollama-compatible model/capability query endpoints
│   │   ├── openai/                       # OpenAI HTTP surface
│   │   │   ├── chat/                     # Chat Completions execution entrypoint
│   │   │   ├── responses/                # Responses API and response store
│   │   │   ├── files/                    # Files API and shared inputfiles service entrypoint
│   │   │   ├── embeddings/               # Embeddings API
│   │   │   ├── history/                  # OpenAI context file handling
│   │   │   └── shared/                   # OpenAI HTTP errors/models/tool formatting
│   │   └── requestbody/                  # HTTP body reading and UTF-8/JSON validation helpers
│   ├── inputfiles/                       # Shared attachment decoding, URL downloads, uploads, deduplication and references/tokens
│   ├── prompt/                           # Prompt composition
│   ├── promptcompat/                     # API request -> DeepSeek web-chat plain-text compatibility
│   ├── rawsample/                        # Raw sample read/write and management
│   ├── responsehistory/                  # DeepSeek upstream response archive and session snapshots
│   ├── server/                           # Router and middleware assembly
│   │   └── data/                         # Router/runtime helper data
│   ├── sse/                              # SSE parsing utilities
│   ├── stream/                           # Unified stream consumption engine
│   ├── testsuite/                        # Testsuite execution framework
│   ├── textclean/                        # Text cleanup
│   ├── toolcall/                         # Tool-call parsing and repair
│   ├── toolstream/                       # Go streaming tool-call anti-leak and delta detection
│   ├── util/                             # Shared utility helpers
│   ├── version/                          # Version query/compare
│   └── webui/                            # WebUI static hosting logic
├── plans/                                # Stage plans and manual QA records
├── pow/                                  # PoW standalone implementation + benchmarks
├── scripts/                              # Build/release helper scripts
├── static/                               # Build artifacts (admin static resources)
├── tests/                                # Test assets and scripts
│   ├── compat/                           # Compatibility fixtures + expected outputs
│   │   ├── expected/                     # Expected output samples
│   │   └── fixtures/                     # Fixture inputs
│   │       ├── sse_chunks/               # SSE chunk fixtures
│   │       └── toolcalls/                # Tool-call fixtures
│   ├── node/                             # Node unit tests
│   ├── raw_stream_samples/               # Upstream raw SSE samples
│   │   ├── continue-thinking-snapshot-replay-20260405/    # Continue-thinking sample
│   │   ├── longtext-deepseek-v4-flash-20260429/           # Flash long-text/file-upload sample
│   │   ├── longtext-deepseek-v4-pro-20260429/             # Pro long-text/file-upload sample
│   │   ├── markdown-format-example-20260405/              # Markdown sample
│   │   └── markdown-format-example-20260405-spacefix/     # Space-fix sample
│   ├── scripts/                          # Test entry scripts
│   └── tools/                            # Testing helper tools
└── webui/                                # React admin console source
    ├── public/                           # Static assets
    └── src/                              # Frontend source code
        ├── app/                          # Routing/state scaffolding
        ├── components/                   # Shared UI components
        ├── features/                     # Feature modules
        │   ├── account/                  # Account management page
        │   ├── apiTester/                # API tester page
        │   ├── chatHistory/              # Server-side conversation history page
        │   ├── proxy/                    # Proxy management page
        │   └── settings/                 # Settings page
        ├── layout/                       # Layout components
        ├── locales/                      # i18n strings
        └── utils/                        # Frontend utilities
```

## 2. Primary Request Flow

```mermaid
flowchart LR
    C[Client / SDK] --> R[internal/server/router.go]

    subgraph HTTP[HTTP API surface]
        OA[internal/httpapi/openai]
        CHAT[openai/chat]
        RESP[openai/responses]
        FILES[openai/files + embeddings]
        AD[internal/httpapi/admin/*]
        WEB[internal/webui static admin]
    end

    subgraph COMPAT[Prompt compatibility core]
        PC[internal/promptcompat]
        PROMPT[internal/prompt]
        HIST[internal/httpapi/openai/history]
    end

    subgraph RUNTIME[Shared runtime]
        AUTH[internal/auth]
        POOL[internal/account queue + concurrency]
        CR[internal/completionruntime]
        TURN[internal/assistantturn]
        STREAM[internal/stream + internal/sse]
        TOOL[internal/toolcall + internal/toolstream]
        FMT[internal/format/openai]
        DS[internal/deepseek/client]
        POW[pow + internal/deepseek/protocol]
    end

    R --> OA --> CHAT
    OA --> RESP
    OA --> FILES
    R --> AD
    R --> WEB

    CHAT --> PC
    RESP --> PC
    PC --> PROMPT
    PC -.long history.-> HIST
    PC --> AUTH
    PC --> CR

    AUTH --> POOL
    CHAT --> CR
    RESP --> CR
    CR --> DS
    CR --> STREAM
    CR --> TURN
    STREAM --> TURN
    STREAM --> TOOL
    TURN --> FMT
    POOL --> CR
    DS --> POW
    DS --> U[DeepSeek upstream]
```

## 3. Responsibilities in `internal/`

- `internal/server`: router tree + middlewares (health, protocol routes, Admin/WebUI).
- `internal/httpapi/openai/*`: OpenAI HTTP surface split into chat, responses, files, embeddings, history, and shared packages; chat/responses share the promptcompat, stream, and toolcall semantics.
- Protocol adapter boundary: HTTP protocol entrypoints first normalize their protocol-specific request shapes into the project-standard request/turn model, run shared business logic (empty-output retry, thinking, tool call, usage, file injection, completion payload assembly, etc.) once on the normalized path, and then render back to the target protocol at the boundary. This "normalize -> share -> render" pattern itself remains a project design principle; the currently adapted protocols are the OpenAI family and the Ollama-compatible query endpoints. Protocol-specific differences stay as explicit adapter concerns and must not sink into shared business logic. Normal direct paths must share DeepSeek session/PoW/completion execution through `completionruntime`.
- `internal/httpapi/ollama`: Ollama-compatible model list and capability query endpoints.
- `internal/httpapi/requestbody`: shared HTTP body reading, JSON pre-validation, and UTF-8 error helpers across protocol adapters.
- `internal/promptcompat`: compatibility core for turning OpenAI requests into DeepSeek web-chat plain-text context.
- `internal/assistantturn`: Go output-side canonical semantics, converting DeepSeek SSE collection results and stream finalization state into assistant turns and centralizing thinking, tool call, citation, usage, stop/error behavior.
- `internal/completionruntime`: shared Go completion execution helpers for DeepSeek session/PoW/call startup, non-stream collection, empty-output retry, and one managed-account fresh retry before a final 429; streaming paths use it to start upstream requests, continue to use `internal/stream` for real-time consumption, and use `assistantturn` during finalization.
- `internal/deepseek/{client,protocol,transport}`: upstream requests, sessions, PoW adaptation, protocol constants, and transport details.
- `internal/stream` + `internal/sse`: Go stream parsing and incremental assembly.
- `internal/toolcall` + `internal/toolstream`: DSML shell compatibility plus canonical XML tool-call parsing and anti-leak sieve; DSML is normalized back to XML at the entrypoint, and internal parsing remains XML-based.
- `internal/httpapi/admin/*`: Admin API root assembly plus auth/accounts/config/settings/proxies/rawsamples/history/devcapture/version resource packages.
- `internal/chathistory`: server-side conversation history persistence, pagination, detail lookup, and retention policy.
- `internal/responsehistory`: DeepSeek upstream response archive, saving assistant text, thinking, raw tool-call fragments, and streaming detail before protocol rendering/trimming.
- `internal/config`: config loading/validation + runtime settings hot-reload.
- `internal/account`: managed account pool, inflight slots, waiting queue.
- `internal/textclean`: text cleanup helpers, e.g. stripping `[reference: N]` markers.
- `internal/compat`: compatibility regression tests using SSE fixtures to verify output consistency.
- `internal/rawsample`: upstream raw response capture, read/write, and management.
- `internal/devcapture`: developer debug capture, storing HTTP request/response for troubleshooting.
- `internal/util`: cross-package utilities including JSON writing, type conversion, token counting, thinking parsing, etc.
- `internal/version`: version query and comparison, supporting build-time injection and runtime resolution.

## 4. WebUI Runtime Relation

- `webui/` stores frontend source (Vite + React).
- Runtime serves static output from `static/admin`.
- On first local startup, if `static/admin` is missing, DS2API may auto-build it (Node.js required).

## 5. Documentation Split Strategy

- Onboarding & quick start: `README.MD` / `README.en.md`
- Architecture & layout: `docs/ARCHITECTURE*.md` (this file)
- API contracts: `API.md` / `API.en.md`
- Deployment/testing/contributing: `docs/DEPLOY*`, `docs/TESTING.md`, `docs/CONTRIBUTING*`
- Deep topics: `docs/toolcall-semantics.md`, `docs/DeepSeekSSE行为结构说明-2026-04-05.md`

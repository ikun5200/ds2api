---
name: ds2api-account-onboarding
description: DS2API 批量账号导入流水线：支持邮箱+密码或邮箱+Token 两种账号，通过管理 API 添加、触发验证并处理 RISK_DEVICE_DETECTED / user is muted / rate limit / Token 失效等结果。只要用户给出一批账号（如 "email----password" 或 "email----password----token" 格式）并要求导入/添加/激活，或要求重新生成设备 ID、修复登录失败，就用本流程。
---

# DS2API 批量账号导入流水线

把一批 DeepSeek 账号接入 ds2api。密码账号需要为每个账号生成**独立**的设备标识（device_id）；Token 账号直接使用第三字段的 Token，不需要设备标识。整个流程用捆绑脚本执行：

```bash
python3 .agents/skills/ds2api-account-onboarding/scripts/import_accounts.py \
  --accounts <账号文件> --base <ds2api地址> --admin-key <管理密钥> [--admin-password]
```

## 何时用

- 用户给出一批账号（常见格式 `email----password` 或 `email----password----token`，也兼容 `email:password`、`email,password`、空白分隔）要求导入 ds2api
- 已有账号登录失败（RISK_DEVICE_DETECTED），需要重新生成设备标识
- 用户要求“刷新所有账号 token”且批量失败时，用本流程修复

## 执行前确认

1. **目标实例**：默认 NAS `http://192.168.31.88:6011`（凭据见工作区记忆 `NAS 部署信息`，或直接问用户）。本地开发实例是 `http://127.0.0.1:5001`。
2. **管理凭据**：管理密码（config.json 的 password_hash 模式）或 DS2API_ADMIN_KEY，二者都能调 `/admin/login`，body 为 `{"admin_key": "<值>"}`。
3. **生成环境**：只有密码账号需要设备标识，且必须在**真实的 Chrome/Edge**（有 GUI）里生成，脚本用 `DS2API_BROWSER_PATH` 指定。headless 浏览器、NAS 上的第三方 chromium（如飞牛浏览器）会被数美风控 SDK 静默降级，返回 6905 字符的无效 blob 而不是 89 字符的 `B` 开头设备号——不要用。Token 账号跳过设备生成。
4. **Token 格式**：第三字段通常是 64 字符 base64，不是 device_id。脚本会把它作为 `token` 提交；ds2api 会调用上游当前用户接口校验并自动补全邮箱/名称。上游登录接口现在要求加密请求体并对新设备触发邮箱验证码，Token 是更可靠的接入方式。
5. **前置检查**：目标实例的 `.env` / 环境变量里**不能有** `DS2API_DEEPSEEK_DEVICE_ID`——它对全部密码账号生效并覆盖每账号配置（见 `internal/deepseek/client/client_auth.go` 的 `loginDeviceID`）。发现就删掉并 `docker compose up -d` 重建。Token 账号不受该变量影响。

## 流程（脚本已封装，理解逻辑便于排障）

对每个账号依次：

1. **生成设备标识（仅密码账号）**：`DS2API_BROWSER_PATH=<Edge路径> node scripts/deepseek-device.mjs --output <file>`（在 ds2api 仓库根目录执行，仓库需在生成机上）。每次生成都用全新临时 profile，产出 89 字符 `B` 开头标识。偶发超时（约 20%），重试一次即可。
2. **添加账号**：`POST /admin/accounts`。密码账号 body 为 `{"name":..., "email":..., "password":..., "device_id":...}`；Token 账号 body 为 `{"name":..., "email":..., "password":..., "token":...}`，`email` 可留空由服务端自动补全。重复邮箱返回“邮箱已存在”→ 视为跳过，继续验证。
3. **验证账号**：`POST /admin/accounts/test`，body `{"identifier":"<email>"}`。密码账号成功返回“Token 刷新成功”；Token 账号成功返回“Token 验证成功”，并附带 `token_source=token`、`user_id`、`email`、`muted`。
4. **失败分类处理**：
   - `biz_code=11 RISK_DEVICE_DETECTED`：仅密码账号；换一个**全新**设备标识重试一次（脚本自动做）。仍失败 = 账号级风控标记，无法服务端解除——告知用户在浏览器手动登录一次 chat.deepseek.com 完成验证后重试。
   - `biz_code=14 "user is muted"`：账号被禁言，导入成功但不可用；ds2api 会自动记录解禁时间并跳过该账号。
   - `biz_code=7 "rate limit reached"`：上游限流，等 30~60 秒重试即可，无持久影响。
   - `Token 校验失败` / `token validation failed`：Token 无效或已过期；让用户提供新 Token，或改用密码+设备标识登录。
   - 串行操作，账号间无需人为间隔；不要并发轰登录接口。

## URL 编写注意

管理 API 的账号路径含 email 时，`+` 必须 `%2B`（否则服务端 `PathUnescape` 会把 `+` 变成空格导致 404/误匹配）：

```python
urllib.parse.quote(email, safe="")
```

## 产出

脚本最后打印汇总表（每个账号：认证方式、设备标识、添加结果、验证结果、失败原因分类）。把表原样给用户，明确列出需要人工处理的账号（Token 失效、手动网页登录类）。

## 测试提示（供回归）

- 正常批量：给 2~3 个 `email----password` 行的文件 → 全部导入成功
- Token 批量：给 `email----password----token` 行的文件 → 不生成设备号，直接验证 Token 成功
- 含重复账号的文件 → 已存在的跳过不报错
- 设备生成超时 → 自动重试后成功

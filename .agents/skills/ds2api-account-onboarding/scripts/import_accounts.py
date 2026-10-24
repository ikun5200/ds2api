#!/usr/bin/env python3
"""DS2API batch account onboarding.

Reads a file of DeepSeek accounts (one per line: email----password, or
email:password / email,password / whitespace separated), generates a fresh
website device ID for each account with the repo's deepseek-device.mjs
(real Chrome/Edge required), adds each account through the admin API, and
verifies the login. Handles the upstream risk-control failure modes.

Usage:
  python3 import_accounts.py --accounts accounts.txt \
      --base http://192.168.31.88:6011 --admin-key <secret> \
      [--device-dir .tmp/devices] [--browser-path "/Applications/.../Edge"] \
      [--skip-device-gen] [--device-json devices.json]

Requires: Node.js 22+ on this machine, a runnable Chrome/Chromium/Edge,
and the ds2api repository checked out (the script runs from anywhere but
invokes <repo>/scripts/deepseek-device.mjs via --repo or cwd detection).
"""

import argparse
import concurrent.futures
import json
import os
import re
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path

REPO = Path(__file__).resolve().parents[4]  # .agents/skills/<name>/scripts/import_accounts.py -> repo root


def log(msg):
    print(msg, flush=True)


def read_accounts(path):
    accounts = []
    for raw in Path(path).read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        email = password = token = ""
        if "----" in line:
            parts = [part.strip() for part in line.split("----")]
            email = parts[0] if parts else ""
            password = parts[1] if len(parts) > 1 else ""
            token = parts[2] if len(parts) > 2 else ""
        else:
            for sep in (":", ",", "\t"):
                if sep in line:
                    parts = [part.strip() for part in line.split(sep, 2)]
                    email = parts[0] if parts else ""
                    password = parts[1] if len(parts) > 1 else ""
                    token = parts[2] if len(parts) > 2 else ""
                    break
            else:
                parts = line.split()
                if len(parts) >= 2:
                    email = parts[0]
                    password = parts[1]
                    token = parts[2] if len(parts) > 2 else ""
        email, password, token = email.strip(), password.strip(), token.strip()
        if email and (password or token):
            accounts.append({"email": email, "password": password, "token": token})
        else:
            log(f"  ! 无法解析行，跳过: {line[:40]}")
    return accounts


def admin_token(base, admin_key):
    body = json.dumps({"admin_key": admin_key}).encode()
    req = urllib.request.Request(base + "/admin/login", data=body,
                                 headers={"Content-Type": "application/json"}, method="POST")
    with urllib.request.urlopen(req, timeout=15) as resp:
        return json.load(resp)["token"]


def api(base, token, method, path, payload=None, timeout=30):
    body = json.dumps(payload).encode() if payload is not None else None
    req = urllib.request.Request(base + path, data=body,
                                 headers={"Authorization": f"Bearer {token}",
                                          "Content-Type": "application/json"},
                                 method=method)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return resp.status, json.load(resp)
    except urllib.error.HTTPError as e:
        raw = e.read().decode()
        try:
            return e.code, json.loads(raw)
        except Exception:
            return e.code, {"detail": raw[:200]}


def classify_failure(message):
    if "RISK_DEVICE_DETECTED" in message or "biz_code 11" in message:
        return "risk_device"
    if "user is muted" in message or "biz_code 14" in message:
        return "muted"
    if "rate limit reached" in message or "biz_code 7" in message:
        return "rate_limited"
    if "Token 校验失败" in message or "token validation failed" in message.lower():
        return "token_invalid"
    if "邮箱已存在" in message:
        return "exists"
    return "other"


def generate_device(index, device_dir, browser_path, attempts=3):
    device_dir.mkdir(parents=True, exist_ok=True)
    out = device_dir / f"device-{os.getpid()}-{index}.txt"
    env = dict(os.environ)
    if browser_path:
        env["DS2API_BROWSER_PATH"] = browser_path
    for attempt in range(1, attempts + 1):
        if out.exists():
            out.unlink()
        result = subprocess.run(
            ["node", str(REPO / "scripts" / "deepseek-device.mjs"), "--output", str(out)],
            env=env, capture_output=True, text=True, timeout=300,
        )
        if result.returncode == 0 and out.exists():
            device = out.read_text(encoding="utf-8").strip()
            if device.startswith("B") and len(device) >= 60:
                return device
        log(f"    设备生成第 {attempt} 次失败，重试...")
        time.sleep(3)
    raise RuntimeError("device generation failed after retries (need a real Chrome/Edge with GUI)")


def load_devices(json_path):
    data = json.loads(Path(json_path).read_text(encoding="utf-8"))
    return {item["email"]: item["device_id"] for item in data}


def process_account(base, token, email, password, device, index, account_token=""):
    entry = {"email": email, "device_id": device[:12] + "..." if device else "",
             "token": "yes" if account_token else "",
             "added": False, "login": None, "note": ""}
    enc = urllib.parse.quote(email, safe="")
    payload = {"name": email.split("@")[0][:30], "email": email,
               "password": password, "device_id": device}
    if account_token:
        payload["token"] = account_token
    status, resp = api(base, token, "POST", "/admin/accounts", payload)
    if status == 200:
        entry["added"] = True
    elif classify_failure(json.dumps(resp, ensure_ascii=False)) == "exists":
        entry["note"] = "已存在，跳过添加"
    else:
        entry["note"] = f"添加失败: {json.dumps(resp, ensure_ascii=False)[:120]}"
        return entry

    for attempt in range(2):
        status, resp = api(base, token, "POST", "/admin/accounts/test",
                           {"identifier": email}, timeout=120)
        message = str(resp.get("message", ""))
        if resp.get("success"):
            entry["login"] = "ok"
            entry["note"] = message[:40]
            return entry
        kind = classify_failure(message)
        if kind == "risk_device" and attempt == 0 and not account_token:
            log(f"    {email[:28]} RISK_DEVICE_DETECTED，换全新设备标识重试...")
            entry["device_id"] = ""
            device = generate_device(f"{index}-r", Path(arg_device_dir), arg_browser_path)
            entry["device_id"] = device[:12] + "..."
            api(base, token, "PUT", f"/admin/accounts/{enc}", {"device_id": device})
            continue
        if kind == "rate_limited":
            log(f"    {email[:28]} 上游限流，等待 45s 重试...")
            time.sleep(45)
            continue
        entry["login"] = kind
        entry["note"] = message[:80]
        return entry
    entry["login"] = classify_failure(entry["note"])
    return entry


def main():
    global arg_device_dir, arg_browser_path
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--accounts", required=True,
                        help="账号文件 (email----password 或 email----password----token 每行一个)")
    parser.add_argument("--base", required=True, help="ds2api 地址，如 http://192.168.31.88:6011")
    parser.add_argument("--admin-key", required=True, help="管理密码或 DS2API_ADMIN_KEY")
    parser.add_argument("--device-dir", default=".tmp/devices")
    parser.add_argument("--browser-path", default=os.environ.get("DS2API_BROWSER_PATH", ""),
                        help="Chrome/Edge 可执行文件路径（生成设备标识用）")
    parser.add_argument("--skip-device-gen", action="store_true", help="不生成，改用 --device-json 提供")
    parser.add_argument("--device-json", default="", help='已有设备号 JSON: [{"email":..,"device_id":..}]')
    args = parser.parse_args()
    arg_device_dir, arg_browser_path = args.device_dir, args.browser_path

    base = args.base.rstrip("/")
    accounts = read_accounts(args.accounts)
    if not accounts:
        sys.exit("没有可导入的账号")
    log(f"共 {len(accounts)} 个账号，目标 {base}")

    token = admin_token(base, args.admin_key)
    preloaded = load_devices(args.device_json) if args.device_json else {}

    results = []
    for index, acc in enumerate(accounts, 1):
        email = acc["email"]
        account_token = acc.get("token", "")
        log(f"[{index}/{len(accounts)}] {email[:32]}" + (" [token]" if account_token else ""))
        try:
            device = ""
            if not account_token:
                device = preloaded.get(email) or ("" if args.skip_device_gen
                                                  else generate_device(index, Path(args.device_dir),
                                                                       args.browser_path))
        except Exception as e:
            results.append({"email": email, "device_id": "", "token": "",
                            "added": False, "login": None, "note": f"设备生成失败: {e}"})
            continue
        entry = process_account(base, token, email, acc["password"], device, index, account_token)
        mark = "✓" if entry["login"] == "ok" else "✗"
        log(f"  {mark} added={entry['added']} login={entry['login']} {entry['note']}")
        results.append(entry)

    log("\n===== 汇总 =====")
    ok = [r for r in results if r["login"] == "ok"]
    manual = [r for r in results if r["login"] == "risk_device"]
    muted = [r for r in results if r["login"] == "muted"]
    invalid_tokens = [r for r in results if r["login"] == "token_invalid"]
    for r in results:
        mark = "✓" if r["login"] == "ok" else "✗"
        log(f" {mark} {r['email'][:36]:38} auth={'token' if r['token'] else 'password':8} "
            f"device={r['device_id'] or '-':16} {r['login'] or '-':14} {r['note']}")
    log(f"\n成功 {len(ok)} / {len(results)}"
        + (f" | Token 无效 {len(invalid_tokens)}（更换 token 后重试）" if invalid_tokens else "")
        + (f" | 需手动网页登录 {len(manual)}（账号级风控，浏览器登录 chat.deepseek.com 一次）" if manual else "")
        + (f" | 被禁言 {len(muted)}（建议后台停用）" if muted else ""))


if __name__ == "__main__":
    main()

#!/usr/bin/env node

// Reads the official DeepSeek device ID from an already-running Chromium
// instance via the Chrome DevTools Protocol. Intended to be launched by
// scripts/deepseek-device-linux.sh, but can also be run directly:
//
//   node scripts/deepseek-device-cdp.mjs --port 9222 --timeout 120

import { parseArgs } from 'node:util'

const { values } = parseArgs({
    options: {
        port: { type: 'string', default: '9222' },
        timeout: { type: 'string', default: '120' },
    },
})

const port = Number(values.port)
const timeoutSeconds = Number(values.timeout)
if (!Number.isInteger(port) || port <= 0 || port > 65535) {
    console.error('deepseek-device-cdp: invalid --port')
    process.exit(1)
}
if (!Number.isFinite(timeoutSeconds) || timeoutSeconds < 10) {
    console.error('deepseek-device-cdp: invalid --timeout')
    process.exit(1)
}
if (typeof WebSocket !== 'function') {
    console.error('deepseek-device-cdp: Node.js 22 or newer is required')
    process.exit(1)
}

const deadline = Date.now() + timeoutSeconds * 1000

async function findPage() {
    while (Date.now() < deadline) {
        try {
            const response = await fetch(`http://127.0.0.1:${port}/json/list`)
            if (response.ok) {
                const targets = await response.json()
                const page = targets.find(target => target.type === 'page' && target.url.startsWith('https://chat.deepseek.com/'))
                if (page) return page
            }
        } catch {
            // Chromium may still be starting.
        }
        await new Promise(resolve => setTimeout(resolve, 300))
    }
    throw new Error('timed out waiting for the DeepSeek page')
}

function connect(url) {
    return new Promise((resolve, reject) => {
        const socket = new WebSocket(url)
        socket.addEventListener('open', () => resolve(socket), { once: true })
        socket.addEventListener('error', () => reject(new Error('could not connect to Chromium DevTools')), { once: true })
    })
}

const page = await findPage()
const socket = await connect(page.webSocketDebuggerUrl)
let nextID = 0
const pending = new Map()
socket.addEventListener('message', event => {
    const message = JSON.parse(event.data)
    const handler = pending.get(message.id)
    if (!handler) return
    pending.delete(message.id)
    handler(message)
})
socket.addEventListener('close', () => {
    for (const handler of pending.values()) handler({ error: { message: 'DevTools connection closed' } })
    pending.clear()
})

function send(method, params = {}) {
    return new Promise(resolve => {
        const id = ++nextID
        pending.set(id, resolve)
        socket.send(JSON.stringify({ id, method, params }))
    })
}

await send('Runtime.enable')

const expression = `(() => {
    const challenge = Boolean(document.querySelector('#challenge-form, #challenge-stage'))
        || /just a moment|attention required/i.test(document.title)
    const sdk = window.SMSdk
    if (!sdk || typeof sdk.getDeviceId !== 'function') {
        return { ready: false, hasSdk: Boolean(sdk), challenge, title: document.title }
    }
    const id = sdk.getDeviceId()
    return {
        ready: true,
        id: typeof id === 'string' ? id : '',
        length: typeof id === 'string' ? id.length : 0,
        prefix: typeof id === 'string' ? id.slice(0, 1) : '',
        challenge,
        title: document.title,
    }
})()`

let last = null
while (Date.now() < deadline) {
    const result = await send('Runtime.evaluate', {
        expression,
        returnByValue: true,
        awaitPromise: true,
    })
    if (result.error) {
        await new Promise(resolve => setTimeout(resolve, 300))
        continue
    }
    const value = result.result?.result?.value || null
    last = value
    if (value?.challenge) {
        console.error('deepseek-device-cdp: the official page requires verification; complete it in the browser window and retry')
        socket.close()
        process.exit(3)
    }
    if (value?.ready && typeof value.id === 'string' && value.id.startsWith('B') && value.id.length > 1) {
        process.stdout.write(`DEVICE_ID=${value.id}\n`)
        socket.close()
        process.exit(0)
    }
    await new Promise(resolve => setTimeout(resolve, 1000))
}

console.error('deepseek-device-cdp: timed out; last state=' + JSON.stringify({
    ready: last?.ready,
    prefix: last?.prefix,
    length: last?.length,
    challenge: last?.challenge,
}))
socket.close()
process.exit(2)

#!/usr/bin/env node

import { spawn } from 'node:child_process'
import { constants, writeSync } from 'node:fs'
import { access, chmod, lstat, mkdtemp, open, readFile, rm, stat } from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import { setTimeout as delay } from 'node:timers/promises'
import { pathToFileURL } from 'node:url'
import { parseArgs } from 'node:util'

const website = 'https://chat.deepseek.com/sign_in'
const help = `Usage: node scripts/deepseek-device.mjs [--output FILE] [--timeout SECONDS]

Open the official DeepSeek sign-in page in an installed Chrome, Chromium, or Edge
using a fresh temporary profile. Wait for the website's original SMSdk to return
a registered device ID, then close this browser and delete its temporary profile.
No account credentials are needed. Verification challenges require user action;
this helper exits without attempting to solve them.

  -h, --help            Show this help
  -o, --output FILE     Write one line to a new file (0600 on POSIX), instead of stdout
      --timeout SECONDS Total browser/registration deadline, 10–300 (default: 120)

Existing output files and symlinks are refused. Progress and errors go to stderr.
Requires Node.js 22+ and a normal, visible browser; no headless mode is used.
Set DS2API_BROWSER_PATH to override detection with a browser executable path.
On macOS, use the executable inside the .app bundle, not the bundle directory.
`

export function parseOptions(args) {
    const { values } = parseArgs({ args, options: {
        help: { type: 'boolean', short: 'h' },
        output: { type: 'string', short: 'o' },
        timeout: { type: 'string', default: '120' },
    } })
    const seconds = Number(values.timeout)
    if (!Number.isInteger(seconds) || seconds < 10 || seconds > 300) {
        throw new Error('--timeout must be an integer between 10 and 300 seconds')
    }
    if (values.output !== undefined && !values.output.trim()) throw new Error('--output must name a file')
    return { help: Boolean(values.help), output: values.output && path.resolve(values.output), timeout: seconds * 1000 }
}

export async function assertOutputAvailable(output) {
    if (!output) return
    try {
        await lstat(output)
    } catch (error) {
        if (error.code === 'ENOENT') {
            const directory = path.dirname(output)
            if (!(await stat(directory)).isDirectory()) throw new Error(`Output parent is not a directory: ${directory}`)
            await access(directory, constants.W_OK)
            return
        }
        throw error
    }
    throw new Error(`Refusing to overwrite an existing output file or symlink: ${output}`)
}

export async function writeDeviceID(output, deviceID) {
    const file = await open(output, 'wx', 0o600)
    let failure
    try {
        await file.writeFile(`${deviceID}\n`, 'utf8')
        await file.sync()
    } catch (error) {
        failure = error
    }
    try {
        await file.close()
    } catch (error) {
        failure = failure ? new AggregateError([failure, error], 'Writing and closing the device file failed') : error
    }
    if (failure) {
        try {
            await rm(output)
        } catch (error) {
            throw new AggregateError([failure, error], 'Writing and removing the incomplete device file failed')
        }
        throw failure
    }
}

export function browserCandidates(platform = process.platform, env = process.env) {
    if (env.DS2API_BROWSER_PATH) return [path.resolve(env.DS2API_BROWSER_PATH)]
    const candidates = []
    if (platform === 'darwin') {
        for (const root of ['/Applications', path.join(os.homedir(), 'Applications')]) {
            for (const name of ['Google Chrome', 'Chromium', 'Microsoft Edge']) {
                candidates.push(path.join(root, `${name}.app`, 'Contents', 'MacOS', name))
            }
        }
    } else if (platform === 'win32') {
        for (const root of [env.PROGRAMFILES, env['PROGRAMFILES(X86)'], env.LOCALAPPDATA].filter(Boolean)) {
            for (const executable of ['Google/Chrome/Application/chrome.exe', 'Chromium/Application/chrome.exe', 'Microsoft/Edge/Application/msedge.exe']) {
                candidates.push(path.join(root, executable))
            }
        }
    }
    const names = platform === 'win32'
        ? ['chrome.exe', 'chromium.exe', 'msedge.exe']
        : ['google-chrome', 'google-chrome-stable', 'chromium', 'chromium-browser', 'microsoft-edge', 'microsoft-edge-stable']
    for (const directory of (env.PATH || '').split(path.delimiter).filter(Boolean)) {
        for (const name of names) candidates.push(path.join(directory, name))
    }
    return candidates
}

async function findBrowser() {
    for (const candidate of browserCandidates()) {
        try {
            await access(candidate, constants.X_OK)
            if ((await lstat(candidate)).isDirectory()) continue
            return candidate
        } catch (error) {
            if (!['ENOENT', 'ENOTDIR', 'EACCES'].includes(error.code)) throw error
        }
    }
    throw new Error('No supported browser executable found. Set DS2API_BROWSER_PATH to an installed Chrome, Chromium, or Edge executable.')
}

class DevTools {
    constructor(socket) {
        this.socket = socket
        this.nextID = 0
        this.pending = new Map()
        socket.addEventListener('message', event => {
            const message = JSON.parse(event.data)
            const pending = this.pending.get(message.id)
            if (!pending) return
            this.pending.delete(message.id)
            clearTimeout(pending.timer)
            if (message.error) {
                const error = new Error(`Browser command ${pending.method} failed (${message.error.code})`)
                error.contextChanged = /execution context|cannot find context/i.test(message.error.message || '')
                pending.reject(error)
            } else pending.resolve(message.result)
        })
        socket.addEventListener('close', () => {
            for (const pending of this.pending.values()) {
                clearTimeout(pending.timer)
                pending.reject(new Error('The temporary browser connection closed'))
            }
            this.pending.clear()
        })
    }

    static async connect(url, timeout) {
        const socket = new WebSocket(url)
        await new Promise((resolve, reject) => {
            const timer = setTimeout(() => {
                socket.close()
                reject(new Error('Timed out connecting to the temporary browser'))
            }, timeout)
            socket.addEventListener('open', () => { clearTimeout(timer); resolve() }, { once: true })
            socket.addEventListener('error', () => {
                clearTimeout(timer)
                reject(new Error('Could not connect to the temporary browser'))
            }, { once: true })
        })
        return new DevTools(socket)
    }

    send(method, params = {}, sessionId, timeout = 5000) {
        return new Promise((resolve, reject) => {
            if (this.socket.readyState !== WebSocket.OPEN) return reject(new Error('The temporary browser connection is unavailable'))
            const id = ++this.nextID
            const timer = setTimeout(() => {
                this.pending.delete(id)
                reject(new Error(`Browser command ${method} timed out`))
            }, timeout)
            this.pending.set(id, { resolve, reject, timer, method })
            this.socket.send(JSON.stringify({ id, method, params, ...(sessionId && { sessionId }) }))
        })
    }

    close() {
        this.socket.close()
    }
}

function remaining(deadline, signal) {
    signal.throwIfAborted()
    const left = deadline - Date.now()
    if (left <= 0) throw new Error('Timed out waiting for official device registration. Check website connectivity or verification requirements; no device ID was generated.')
    return left
}

async function waitForDevTools(profile, browser, deadline, signal) {
    for (;;) {
        remaining(deadline, signal)
        if (browser.failure) throw new Error(`Could not start the browser (${browser.failure.code || 'spawn error'})`)
        if (browser.exited) throw new Error('The temporary browser exited before device registration')
        try {
            const [port, endpoint] = (await readFile(path.join(profile, 'DevToolsActivePort'), 'utf8')).trim().split(/\r?\n/)
            if (/^\d+$/.test(port) && Number(port) > 0 && Number(port) <= 65535 && /^\/devtools\/browser\/[\w-]+$/.test(endpoint)) {
                return `ws://127.0.0.1:${port}${endpoint}`
            }
        } catch (error) {
            if (error.code !== 'ENOENT') throw error
        }
        await delay(Math.min(200, remaining(deadline, signal)), undefined, { signal })
    }
}

const inspectDevice = `(() => {
    if (location.href === 'about:blank') return {};
    if (location.origin !== 'https://chat.deepseek.com') return { outsideWebsite: true };
    const challenge = Boolean(document.querySelector('#challenge-form, #challenge-stage'))
        || /just a moment|attention required/i.test(document.title);
    if (challenge) return { challenge: true };
    const sdk = window.SMSdk;
    if (!sdk || typeof sdk.getDeviceId !== 'function') return {};
    const id = sdk.getDeviceId();
    return typeof id === 'string' && id.startsWith('B') && id.length > 1 ? { id } : {};
})()`

async function readRegisteredDevice(devtools, deadline, signal) {
    const command = (method, params = {}, sessionId) => devtools.send(method, params, sessionId, Math.min(5000, remaining(deadline, signal)))
    let target
    while (!target) {
        const { targetInfos } = await command('Target.getTargets')
        target = targetInfos.find(info => info.type === 'page' && info.url.startsWith('https://chat.deepseek.com/'))
        if (!target) await delay(Math.min(300, remaining(deadline, signal)), undefined, { signal })
    }
    const { sessionId } = await command('Target.attachToTarget', { targetId: target.targetId, flatten: true })
    for (;;) {
        let result
        try {
            result = await command('Runtime.evaluate', { expression: inspectDevice, returnByValue: true }, sessionId)
        } catch (error) {
            if (!error.contextChanged) throw error
            await delay(Math.min(300, remaining(deadline, signal)), undefined, { signal })
            continue
        }
        // Navigation may destroy an execution context. A page-level exception is
        // not a valid device; keep waiting within the same overall deadline.
        const value = result.result?.value || {}
        if (value.challenge) throw new Error('The official page requires verification. Complete that step through the website; this helper does not solve challenges.')
        if (value.outsideWebsite) throw new Error('The temporary tab left the official DeepSeek website before registration')
        if (value.id) return value.id
        await delay(Math.min(300, remaining(deadline, signal)), undefined, { signal })
    }
}

export async function stopBrowser(browser, devtools) {
    let closeError
    if (devtools) {
        try {
            await devtools.send('Browser.close', {}, undefined, 2000)
        } catch (error) {
            closeError = error
        } finally {
            devtools.close()
        }
    }
    for (const [wait, signal] of [[3000, 'SIGTERM'], [2000, 'SIGKILL'], [2000, null]]) {
        if (browser.exited || (browser.failure && !browser.child.pid)) return
        await new Promise(resolve => {
            const timer = setTimeout(resolve, wait)
            browser.exit.then(() => { clearTimeout(timer); resolve() })
        })
        if (browser.exited) return
        if (signal) browser.child.kill(signal)
    }
    throw new Error(`Could not close the browser started by this helper${browser.killError ? ` (${browser.killError.code})` : closeError ? `: ${closeError.message}` : ''}`)
}

async function acquireDevice(timeout, signal) {
    const executable = await findBrowser()
    const profile = await mkdtemp(path.join(os.tmpdir(), 'ds2api-device-'))
    const browser = { child: null, exited: false, failure: null, exit: null }
    let devtools
    try {
        await chmod(profile, 0o700)
        console.error('Opening the official DeepSeek page in a temporary browser profile...')
        browser.child = spawn(executable, [
            `--user-data-dir=${profile}`,
            '--remote-debugging-address=127.0.0.1',
            '--remote-debugging-port=0',
            '--no-first-run',
            '--no-default-browser-check',
            website,
        ], { stdio: 'ignore', windowsHide: false })
        browser.exit = new Promise(resolve => {
            browser.child.once('exit', () => { browser.exited = true; resolve() })
            browser.child.on('error', error => {
                if (browser.child.pid) browser.killError = error
                else { browser.failure = error; resolve() }
            })
        })
        const deadline = Date.now() + timeout
        const endpoint = await waitForDevTools(profile, browser, deadline, signal)
        devtools = await DevTools.connect(endpoint, Math.min(5000, remaining(deadline, signal)))
        console.error(`Waiting for the official SDK to register this browser (deadline ${timeout / 1000}s)...`)
        return await readRegisteredDevice(devtools, deadline, signal)
    } finally {
        let stopped = !browser.child
        try {
            if (browser.child) await stopBrowser(browser, devtools)
            stopped = true
        } finally {
            if (stopped) await rm(profile, { recursive: true, force: true, maxRetries: 3, retryDelay: 200 })
            else console.error(`Temporary browser still running; retained its profile at ${profile}`)
        }
    }
}

async function main() {
    const options = parseOptions(process.argv.slice(2))
    if (options.help) { process.stdout.write(help); return }
    if (Number(process.versions.node.split('.')[0]) < 22 || typeof WebSocket !== 'function') {
        throw new Error('This helper requires Node.js 22 or newer')
    }
    await assertOutputAvailable(options.output)
    const controller = new AbortController()
    const interrupt = () => controller.abort(new Error('Device initialization interrupted'))
    process.on('SIGINT', interrupt)
    process.on('SIGTERM', interrupt)
    try {
        const deviceID = await acquireDevice(options.timeout, controller.signal)
        controller.signal.throwIfAborted()
        if (options.output) await writeDeviceID(options.output, deviceID)
        else {
            const bytes = Buffer.from(`${deviceID}\n`)
            for (let offset = 0; offset < bytes.length;) offset += writeSync(1, bytes, offset, bytes.length - offset)
        }
        console.error(`Device registration succeeded (${deviceID.length} characters)${options.output ? `; saved to ${options.output}` : ''}.`)
    } finally {
        process.removeListener('SIGINT', interrupt)
        process.removeListener('SIGTERM', interrupt)
    }
}

if (process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href) {
    main().catch(error => {
        console.error(`deepseek-device: ${error.message}`)
        process.exitCode = 1
    })
}

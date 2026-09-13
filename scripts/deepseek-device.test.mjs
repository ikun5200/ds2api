import assert from 'node:assert/strict'
import { spawnSync } from 'node:child_process'
import { mkdtemp, readFile, rm, stat, symlink, writeFile } from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import { setImmediate } from 'node:timers/promises'
import { fileURLToPath } from 'node:url'
import test from 'node:test'

import { assertOutputAvailable, stopBrowser, writeDeviceID } from './deepseek-device.mjs'

const script = fileURLToPath(new URL('./deepseek-device.mjs', import.meta.url))
const fixtureID = 'Btest-fixture-only-not-a-real-device'

async function tempDirectory(t) {
    const directory = await mkdtemp(path.join(os.tmpdir(), 'ds2api-device-test-'))
    t.after(() => rm(directory, { recursive: true, force: true }))
    return directory
}

test('device output is a private single line and exclusive writes preserve an existing ID', async t => {
    const output = path.join(await tempDirectory(t), 'device-id')
    await assertOutputAvailable(output)
    await writeDeviceID(output, fixtureID)
    assert.equal(await readFile(output, 'utf8'), `${fixtureID}\n`)
    if (process.platform !== 'win32') assert.equal((await stat(output)).mode & 0o777, 0o600)
    await assert.rejects(writeDeviceID(output, 'another-test-fixture'), { code: 'EEXIST' })
    assert.equal(await readFile(output, 'utf8'), `${fixtureID}\n`)
})

test('an output symlink cannot replace or alter its target', { skip: process.platform === 'win32' }, async t => {
    const directory = await tempDirectory(t)
    const target = path.join(directory, 'keep')
    const output = path.join(directory, 'device-id')
    await writeFile(target, 'keep this content')
    await symlink(target, output)
    await assert.rejects(assertOutputAvailable(output), /Refusing to overwrite/)
    await assert.rejects(writeDeviceID(output, fixtureID), { code: 'EEXIST' })
    assert.equal(await readFile(target, 'utf8'), 'keep this content')
})

test('an existing output is rejected before starting any browser and without exposing its contents', async t => {
    const directory = await tempDirectory(t)
    const output = path.join(directory, 'device-id')
    await writeDeviceID(output, fixtureID)
    const result = spawnSync(process.execPath, [script, '--output', output], {
        encoding: 'utf8',
        timeout: 5000,
        env: { ...process.env, DS2API_BROWSER_PATH: path.join(directory, 'missing-browser') },
    })
    assert.equal(result.status, 1)
    assert.equal(result.stdout, '')
    assert.match(result.stderr, /Refusing to overwrite/)
    assert.doesNotMatch(result.stderr, /Opening the official/)
    assert.equal(result.stderr.includes(fixtureID), false)
    assert.equal(await readFile(output, 'utf8'), `${fixtureID}\n`)
})

test('help is available without installing or launching a browser', () => {
    const result = spawnSync(process.execPath, [script, '--help'], {
        encoding: 'utf8', timeout: 5000,
        env: { ...process.env, DS2API_BROWSER_PATH: path.join(os.tmpdir(), 'missing-browser') },
    })
    assert.equal(result.status, 0)
    assert.match(result.stdout, /Node\.js 22\+/)
    assert.match(result.stdout, /DS2API_BROWSER_PATH/)
    assert.equal(result.stderr, '')
})

test('graceful shutdown closes the owned CDP browser without sending process signals', async () => {
    let closed = false
    const browser = {
        child: { pid: 1, kill: () => assert.fail('should not send a signal after graceful shutdown') },
        exited: false,
    }
    const devtools = {
        send: async method => { assert.equal(method, 'Browser.close'); browser.exited = true },
        close: () => { closed = true },
    }
    await stopBrowser(browser, devtools)
    assert.equal(closed, true)
})

test('a process error is not treated as proof that the browser has exited', async t => {
    t.mock.timers.enable({ apis: ['setTimeout'] })
    const signals = []
    const browser = {
        child: { pid: 1, kill: signal => { signals.push(signal); return false } },
        exited: false,
        failure: Object.assign(new Error('cannot terminate'), { code: 'EPERM' }),
        exit: new Promise(() => {}),
    }
    const stopped = assert.rejects(stopBrowser(browser), /Could not close the browser/)
    for (const wait of [3000, 2000, 2000]) {
        t.mock.timers.tick(wait)
        await setImmediate()
    }
    await stopped
    assert.deepEqual(signals, ['SIGTERM', 'SIGKILL'])
    assert.equal(browser.exited, false)
})

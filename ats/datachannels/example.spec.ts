import { Buffer } from 'node:buffer';
import { test, expect, Page, BrowserContext } from '@playwright/test'
import { Client } from 'ssh2'

test.describe("pion's data channels example", ()  => {

    const sleep = (ms) => { return new Promise(r => setTimeout(r, ms)) }

    let page: Page,
        context: BrowserContext

    test.beforeAll(async ({ browser }) => {
        context = await browser.newContext()
        page = await context.newPage()
        page.on('console', (msg) => console.log('console log:', msg.text()))
        page.on('pageerror', (err: Error) => console.log('PAGEERROR', err.message))
        const response = await page.goto("http://client/demo.html")
        await expect(response.ok()).toBeTruthy()
        await page.evaluate(() => {
            var newScript = document.createElement('script');
            newScript.type = 'text/javascript';
            newScript.src = '/demo.js';
            document.getElementsByTagName('head')[0].appendChild(newScript);
        })
    })

    test('can connect', async () => {
        let cmdClosed = false
        let conn, stream
        try {
            conn = await new Promise((resolve, reject) => {
                const conn = new Client()
                conn.on('error', e => reject(e))
                conn.on('ready', () => resolve(conn))
                conn.connect({
                  host: 'pion',
                  port: 22,
                  username: 'pion',
                  password: 'pion'
                })
            })
        } catch(e) { expect(e).toBeNull() }
        // log key SSH events
        conn.on('error', e => console.log("ssh error", e))
        conn.on('close', e => {
            cmdClosed = true
            console.log("ssh closed", e)
        })
        conn.on('end', e => console.log("ssh ended", e))
        conn.on('keyboard-interactive', e => console.log("ssh interaction", e))
        let offer
        while (!offer) {
             await sleep(200)
             offer = await page.evaluate(() => document.getElementById('localSessionDescription').value)
        }
        try {
            stream = await new Promise((resolve, reject) => {
                
                conn.exec(`bash -c 'cd /source; echo ${offer} |  /go/bin/data-channels'`,
                        { pty: true }, async (err, s) => {
                    if (err)
                        reject(err)
                    else 
                        resolve(s)
                })
            })
        } catch(e) { expect(e).toBeNull() }
        let dataLines = 0
        let webexecCan = ""
        let finished = false
        let lineCounter = 0
        stream.on('close', (code, signal) => {
            console.log(`closed with ${signal}`)
            cmdClosed = true
            conn.end()
        }).on('data', lines => 
            new Buffer.from(lines).toString().split("\r\n").forEach(async line => {
                if (!line)
                    return
                lineCounter++
                if (lineCounter == 1) {
                    await page.evaluate(async (answer) =>
                        document.getElementById("remoteSessionDescription")
                                .value = answer,
                        line)
                    page.locator("data-test-id=start-session").click()
                    await sleep(3000)
                    await page.evaluate(async () => 
                        document.getElementById("message").value = "BADFACE")
                    await page.locator("data-test-id=send-message").click()
                    return
                }
                if (line.includes("BADFACE"))
                    finished=true
            })
        ).stderr.on('data', (data) => {
              console.log("ERROR: " + data)
        })
        expect(page.locator("#logs")).toHaveText(/Random messages will now be sent/)
        while (!finished) {
            await sleep(500)
        }
    })
})

import { Buffer } from 'node:buffer';
import { test, expect, Page, BrowserContext } from '@playwright/test'
import { Client } from 'ssh2'


test.describe("pion's data channels example", ()  => {

    const sleep = (ms) => { return new Promise(r => setTimeout(r, ms)) }

    let page: Page
    let context: BrowserContext
    let SSHconn: Client
    let stream

    test.beforeAll(async ({ browser }) => {
        context = await browser.newContext()
        page = await context.newPage()
        page.on('console', (msg) => console.log('console log:', msg.text()))
        page.on('pageerror', (err: Error) => console.log('PAGEERROR', err.message))
        // Load the javascript file
        page.on('load', () => page.evaluate(() => {
                var newScript = document.createElement('script')
                newScript.src = 'demo.js'
                document.head.appendChild(newScript)
            })
        )
        const response = await page.goto("http://client/demo.html")
        await expect(response.ok()).toBeTruthy()
        SSHconn = null
    })

    test('setup SSH', async () => {
        while (SSHconn == null) {
            try {
                SSHconn = await new Promise((resolve, reject) => {
                    const SSHconn = new Client()
                    SSHconn.on('error', e => reject(e))
                    SSHconn.on('ready', () => resolve(SSHconn))
                    SSHconn.connect({
                      host: 'pion',
                      port: 22,
                      username: 'pion',
                      password: 'pion'
                    })
                })
            } catch(e) {
                console.log("SSH connection failed, retrying")
                await sleep(3000)
            }
        }
        // log key SSH events
        SSHconn.on('error', e => console.log("ssh error", e))
        SSHconn.on('close', e => {
            console.log("ssh closed", e)
        })
        SSHconn.on('end', e => console.log("ssh ended", e))
        SSHconn.on('keyboard-interactive', e => console.log("ssh interaction", e))
    })
    test('open the command stream', async () => {
        let offer
        while (!offer) {
             await sleep(200)
             offer = await page.evaluate(() =>
                document.getElementById('localSessionDescription').value
            )
        }
        try {
            stream = await new Promise((resolve, reject) => {
                const path = "/go/src/github.com/pion/webrtc/ats/data-channels/start_server.bash"
                SSHconn.exec(`bash ${path} ${offer}`,
                        { pty: true }, async (err, s) => {
                    if (err)
                        reject(err)
                    else 
                        resolve(s)
                })
            })
        } catch(e) { expect(e).toBeNull() }
        stream.on('close', (code, signal) => {
            console.log(`SSH closed with ${signal}`)
            SSHconn.end()
        }) 
    })
    test('transmit and receive data', async()=> {
        let eof = false
        let lineCounter = 0
        stream.on('data', lines => 
            new Buffer.from(lines).toString().split("\r\n")
                .forEach(async (line: string) => {
                if (!line)
                    return
                lineCounter++
                if (lineCounter == 1) {
                    // copy the answer to the page
                    await page.evaluate(async (answer) =>
                        document.getElementById("remoteSessionDescription")
                                .value = answer,
                        line)
                    page.locator("data-test-id=start-session").click()
                    // set the message to EOF
                    await page.evaluate(async () => 
                        document.getElementById("message").value = "EOF")
                    // wait for the send channel to open
                    let connected = false
                    while (!connected) {
                        await sleep(200)
                        connected = await page.evaluate(() => sendChannel.readyState == "open")
                    }
                    // send the message
                    await page.locator("data-test-id=send-message").click()
                    return
                }
                // exit the test when EOF was received from the server
                if (line.includes("EOF"))
                    eof = true
            })
        ).stderr.on('data', (data) => {
              console.log("ERROR: " + data)
        })
        // wait for the EOF message
        while (!eof) {
            await sleep(200)
        }
    })
})

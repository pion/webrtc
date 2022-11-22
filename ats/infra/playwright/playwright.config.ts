// playwright.config.ts
import { PlaywrightTestConfig, devices } from '@playwright/test';

const config: PlaywrightTestConfig = {
  headless: true,
  forbidOnly: !!process.env.CI,
  retries: 2,
  outputDir: '/result',
  use: {
    trace: 'retain-on-failure',
  },
  projects: [
    {
      name: "chromium",
      permissions:["camera", "microphone"],

      use:{
        browserName:"chromium",
        launchOptions:{
            args: ['--use-fake-ui-for-media-stream', '--use-fake-device-for-media-stream']
        }
      },
    },
    {
      name: "firefox",
      use: {
        browserName:"firefox",
        launchOptions: {
          args:[ "--quiet", "--use-test-media-devices" ],
          firefoxUserPrefs: { "media.navigator.streams.fake": true, "media.navigator.permission.disabled": true }
        }
      }
    },
  ],
};
export default config;

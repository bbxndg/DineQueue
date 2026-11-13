import { defineConfig } from '@playwright/test';

if (!process.env.F01_BASE_URL || !process.env.F01_TEST_PASSWORD) {
  throw new Error('F01_BASE_URL and F01_TEST_PASSWORD are required; use an isolated seeded test environment.');
}

export default defineConfig({
  testDir: './e2e', workers: 1, retries: 0, reporter: 'list',
  outputDir: '/tmp/dineq-f01-playwright-results',
  use: { baseURL: process.env.F01_BASE_URL, trace: 'off', video: 'off', screenshot: 'off' },
});

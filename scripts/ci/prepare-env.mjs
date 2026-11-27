// Private, disposable local test configuration; no developer/GCP credentials.
import { randomBytes } from 'node:crypto';
import { appendFileSync, mkdirSync, mkdtempSync, writeFileSync } from 'node:fs';
import { join, resolve } from 'node:path';

const root = process.env.RUNNER_TEMP;
const port = process.env.CI_PG_PORT;
const webPort = process.env.CI_WEB_HOST_PORT || '3002';
const apiPort = process.env.CI_API_HOST_PORT || '8082';
const dbPort = process.env.CI_DB_HOST_PORT || '55432';
const validPort = value => /^\d+$/.test(value ?? '') && Number(value) >= 1 && Number(value) <= 65535;
if (!root || /[\r\n]/.test(root) || ![port, webPort, apiPort, dbPort].every(validPort)) {
  throw Error('RUNNER_TEMP and a valid CI_PG_PORT for the disposable PostgreSQL service are required.');
}
const run = `${process.env.GITHUB_RUN_ID || randomBytes(6).toString('hex')}-${process.env.GITHUB_RUN_ATTEMPT || '1'}`;
if (!/^[a-z0-9-]+$/.test(run)) throw Error('Invalid CI project identifier.');
if (new Set([port, webPort, apiPort, dbPort].map(Number)).size !== 4) throw Error('CI host ports must be distinct.');
mkdirSync(root, { recursive: true });
const directory = mkdtempSync(join(resolve(root), 'dinequeue-ep4-'));
const password = randomBytes(24).toString('hex');
const seed = randomBytes(24).toString('hex');
const goURL = `postgres://dinequeue_ci:ci-service-only@127.0.0.1:${port}/dinequeue_ci_test?sslmode=disable`;
const appURL = `postgres://dinequeue_browser:${password}@db:5432/dinequeue_browser_test?sslmode=disable`;
const fixtureURL = `postgres://dinequeue_browser:${password}@127.0.0.1:${dbPort}/dinequeue_browser_test?sslmode=disable`;
if (process.env.GITHUB_ACTIONS === 'true') {
  for (const value of [password, seed, 'ci-service-only', goURL, appURL, fixtureURL]) {
    console.log(`::add-mask::${value}`);
  }
}
const save = (name, text) => {
  const path = join(directory, name);
  writeFileSync(path, text, { mode: 0o600, flag: 'wx' });
  return path;
};
const compose = save('compose.env', [
  'POSTGRES_USER=dinequeue_browser', 'POSTGRES_DB=dinequeue_browser_test',
  `POSTGRES_PASSWORD=${password}`, `DATABASE_URL=${appURL}`,
  'PORT=8080', 'WEB_PORT=3000', `WEB_HOST_PORT=${webPort}`, `API_HOST_PORT=${apiPort}`,
  ...['SEED_CUSTOMER_PASSWORD', 'SEED_CUSTOMER2_PASSWORD', 'SEED_HOST_PASSWORD', 'SEED_OTHER_HOST_PASSWORD'].map(key => `${key}=${seed}`),
  '',
].join('\n'));
const browser = save('browser.env', [
  `F01_BASE_URL=http://localhost:${webPort}`, `F01_TEST_PASSWORD=${seed}`,
  `TEST_DATABASE_URL=${fixtureURL}`, '',
].join('\n'));
const go = save('go.env', `TEST_DATABASE_URL=${goURL}\n`);
save('browser-ports.yaml', `services:\n  db:\n    ports:\n      - "127.0.0.1:${dbPort}:5432"\n`);
if (process.env.GITHUB_ENV) {
  appendFileSync(process.env.GITHUB_ENV, [
    `DINEQUEUE_CI_DIR=${directory}`, `DINEQUEUE_COMPOSE_ENV=${compose}`,
    `DINEQUEUE_BROWSER_ENV=${browser}`, `DINEQUEUE_GO_ENV=${go}`,
    `DINEQUEUE_COMPOSE_PROJECT=dinequeue-ci-${run}`, '',
  ].join('\n'));
}
console.log('Prepared private disposable test configuration; no developer or GCP configuration was read.');

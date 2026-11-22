import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
import { runInNewContext } from 'node:vm';
import * as solid from 'solid-js';
import ts from 'typescript';

// Execute the real store, API client, and constants with isolated module state.
// TypeScript is already a dev dependency; only fetch is replaced, never the
// production connection-state transitions or Solid's browser reactivity.
const [storeCode, apiCode, domainCode] = await Promise.all(
  ['./accurateAuth.ts', '../api.ts', '../domain.ts'].map(async (path) =>
    ts.transpileModule(await readFile(new URL(path, import.meta.url), 'utf8'), {
      compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
    }).outputText,
  ),
);

function evaluate(code, globals = {}) {
  const exports = {};
  runInNewContext(code, { exports, URLSearchParams, ...globals });
  return exports;
}

function createStore() {
  const requests = [];
  const api = evaluate(apiCode, {
    fetch: () => new Promise((resolve, reject) => {
      requests.push({
        respond(body, status = 200) {
          resolve(new Response(JSON.stringify(body), {
            status,
            headers: { 'Content-Type': 'application/json' },
          }));
        },
        reject,
      });
    }),
  });
  const domain = evaluate(domainCode);
  const store = evaluate(storeCode, {
    require(name) {
      if (name === 'solid-js') return solid;
      if (name === '~/api') return api;
      if (name === '~/domain') return domain;
      throw new Error(`Unexpected store dependency: ${name}`);
    },
  });
  return { store, requests };
}

const connected = {
  enabled: true,
  connected: true,
  provider: 'web',
  accountId: 42,
  accountName: 'Fixture account',
  sessionState: 'unverified',
  database: { id: 7, alias: 'Fixture database' },
};

async function connectedStore() {
  const fixture = createStore();
  const initial = fixture.store.refreshStatus();
  fixture.requests.shift().respond(connected);
  await initial;
  return fixture;
}

test('pending session validation preserves the active connection and database', async () => {
  const { store, requests } = await connectedStore();
  const validation = store.verifyStatus(42);

  assert.equal(store.activeStatus().connected, true);
  assert.equal(store.activeStatus().database.id, 7);
  assert.equal(store.statusFor(42).connected, true);

  requests.shift().respond({ ...connected, sessionState: 'valid' });
  await validation;
  assert.equal(store.activeStatus().connected, true);
  assert.equal(store.activeStatus().sessionState, 'valid');
});

test('background metadata refresh keeps the connected indicator until its result arrives', async () => {
  const { store, requests } = await connectedStore();
  const refresh = store.refreshStatus();

  assert.equal(store.workstationStatusState(), 'ready');
  assert.equal(store.activeStatus().connected, true);
  assert.equal(store.activeStatus().database.id, 7);

  requests.shift().respond({ ...connected, connected: false, database: undefined, sessionState: 'disconnected' });
  await refresh;
  assert.equal(store.activeStatus().connected, false);
  assert.equal(store.activeStatus().database, undefined);
});

for (const sessionState of ['expired', 'unverified']) {
  test(`settled ${sessionState} validation clears the usable connection`, async () => {
    const { store, requests } = await connectedStore();
    const validation = store.verifyStatus(42);
    requests.shift().respond({ ...connected, connected: false, database: undefined, sessionState });
    await validation;

    assert.equal(store.activeStatus().connected, false);
    assert.equal(store.activeStatus().database, undefined);
    assert.equal(store.activeStatus().sessionState, sessionState);
  });
}

test('a rejected validation clears the connection rather than retaining cached success', async () => {
  const { store, requests } = await connectedStore();
  const validation = store.verifyStatus(42);
  requests.shift().reject(new Error('Fixture network failure'));
  await assert.rejects(validation, /Fixture network failure/);

  assert.equal(store.activeStatus().connected, false);
  assert.equal(store.activeStatus().database, undefined);
  assert.equal(store.activeStatus().sessionState, 'unverified');
});

test('a late successful validation cannot undo newer session invalidation', async () => {
  const { store, requests } = await connectedStore();
  const validation = store.verifyStatus(42);
  store.handleSessionInvalidated('Fixture session expired', 42);
  requests.shift().respond({ ...connected, sessionState: 'valid' });
  await validation;

  assert.equal(store.activeStatus().connected, false);
  assert.equal(store.activeStatus().database, undefined);
  assert.equal(store.activeStatus().sessionState, 'expired');
});

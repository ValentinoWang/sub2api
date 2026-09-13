import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { buildZip, crc32, RUNTIME_FILES } from '../package.mjs';

test('MV3 has exact hosts, no page bridge and no broad credentials permissions', async () => {
  const manifest = JSON.parse(await readFile(new URL('../manifest.json', import.meta.url)));
  assert.equal(manifest.manifest_version, 3);
  assert.equal(manifest.background.type, 'module');
  assert.deepEqual(manifest.permissions.sort(), ['alarms', 'scripting', 'storage']);
  assert.deepEqual(manifest.host_permissions, ['https://www.ldxp.cn/*']);
  // Chrome match patterns cannot restrict ports. backend.js separately requires 8080.
  assert.deepEqual(manifest.optional_host_permissions, ['https://ai.rest2build.lol/*', 'http://127.0.0.1/*']);
  assert.equal(manifest.action.default_popup, undefined);
  assert.equal(manifest.externally_connectable, undefined);
  assert.equal(manifest.content_scripts, undefined);
  assert.equal(manifest.web_accessible_resources, undefined);
  const worker = await readFile(new URL('../worker.js', import.meta.url), 'utf8');
  assert.match(worker, /sender\.id !== chrome\.runtime\.id/);
  assert.match(worker, /sender\.tab\?\.url && sender\.tab\.url !== sender\.url/);
  assert.match(worker, /sender\.url === chrome\.runtime\.getURL\('options\.html'\)/);
  assert.match(worker, /TRUSTED_CONTEXTS/);
  assert.doesNotMatch(worker, /setInterval|onMessageExternal|window\.postMessage/);
});
test('ZIP is reproducible, CRC-correct, contains only declared runtime files and all entrypoints', async () => {
  const zip = await buildZip();
  assert.deepEqual(zip, await buildZip());
  const names = []; let cursor = 0;
  while (zip.readUInt32LE(cursor) === 0x04034b50) {
    const size = zip.readUInt32LE(cursor + 18); const nameSize = zip.readUInt16LE(cursor + 26);
    const name = zip.subarray(cursor + 30, cursor + 30 + nameSize).toString();
    const data = zip.subarray(cursor + 30 + nameSize, cursor + 30 + nameSize + size);
    names.push(name);
    assert.equal(crc32(data), zip.readUInt32LE(cursor + 14));
    assert.deepEqual(data, await readFile(new URL(`../${name}`, import.meta.url)));
    cursor += 30 + nameSize + size;
  }
  assert.deepEqual(names, RUNTIME_FILES);
  const manifest = JSON.parse(await readFile(new URL('../manifest.json', import.meta.url)));
  for (const entry of [manifest.background.service_worker, manifest.options_ui.page]) assert.ok(names.includes(entry));
  assert.equal(zip.readUInt32LE(cursor), 0x02014b50);
  assert.equal(zip.readUInt32LE(zip.length - 22), 0x06054b50);
});

import { readFile, mkdir, writeFile } from 'node:fs/promises';
import { dirname, resolve } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

export const RUNTIME_FILES = Object.freeze([
  'manifest.json', 'worker.js', 'core.js', 'backend.js', 'merchant-adapter.js',
  'popup.html', 'popup.js', 'options.html', 'options.js', 'ui.css',
]);
export function crc32(data) {
  let crc = 0xffffffff;
  for (const byte of data) {
    crc ^= byte;
    for (let bit = 0; bit < 8; bit++) crc = (crc >>> 1) ^ ((crc & 1) ? 0xedb88320 : 0);
  }
  return (crc ^ 0xffffffff) >>> 0;
}
// Store-only ZIP uses fixed DOS timestamps and an explicit file allowlist, so the
// same source bytes yield the same package without external tools or dependencies.
export async function buildZip(source = dirname(fileURLToPath(import.meta.url))) {
  const local = []; const central = []; let offset = 0;
  for (const file of RUNTIME_FILES) {
    const name = Buffer.from(file); const data = await readFile(resolve(source, file)); const crc = crc32(data);
    const header = Buffer.alloc(30);
    header.writeUInt32LE(0x04034b50); header.writeUInt16LE(20, 4); header.writeUInt16LE(0x21, 12);
    header.writeUInt32LE(crc, 14); header.writeUInt32LE(data.length, 18); header.writeUInt32LE(data.length, 22); header.writeUInt16LE(name.length, 26);
    local.push(header, name, data);
    const entry = Buffer.alloc(46);
    entry.writeUInt32LE(0x02014b50); entry.writeUInt16LE(20, 4); entry.writeUInt16LE(20, 6); entry.writeUInt16LE(0x21, 14);
    entry.writeUInt32LE(crc, 16); entry.writeUInt32LE(data.length, 20); entry.writeUInt32LE(data.length, 24); entry.writeUInt16LE(name.length, 28); entry.writeUInt32LE(offset, 42);
    central.push(entry, name); offset += header.length + name.length + data.length;
  }
  const directory = Buffer.concat(central); const end = Buffer.alloc(22);
  end.writeUInt32LE(0x06054b50); end.writeUInt16LE(RUNTIME_FILES.length, 8); end.writeUInt16LE(RUNTIME_FILES.length, 10); end.writeUInt32LE(directory.length, 12); end.writeUInt32LE(offset, 16);
  return Buffer.concat([...local, directory, end]);
}
if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  const output = process.argv[2] ? resolve(process.argv[2]) : fileURLToPath(new URL('../../frontend/public/assets/ldxp-browser-extension.zip', import.meta.url));
  await mkdir(dirname(output), { recursive: true });
  await writeFile(output, await buildZip());
  process.stdout.write(`Extension package: ${output}\n`);
}

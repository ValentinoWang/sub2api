"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs/promises");
const http = require("node:http");
const os = require("node:os");
const path = require("node:path");
const test = require("node:test");

const { main } = require("./notify-maintenance");

function success(data) {
  return JSON.stringify({ code: 0, message: "success", data });
}

async function startServer(handler) {
  const server = http.createServer(handler);
  await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
  const address = server.address();
  return {
    baseURL: `http://127.0.0.1:${address.port}`,
    close: () => new Promise((resolve, reject) => server.close((error) => (error ? reject(error) : resolve()))),
  };
}

async function readRequest(request) {
  const chunks = [];
  for await (const chunk of request) chunks.push(chunk);
  return JSON.parse(Buffer.concat(chunks).toString("utf8"));
}

function makeOutput() {
  let value = "";
  return { write(chunk) { value += chunk; }, value: () => value };
}

function environment(baseURL, authentication = "api-key") {
  return authentication === "api-key"
    ? { SUB2API_BASE_URL: baseURL, SUB2API_ADMIN_API_KEY: "offline-admin-key" }
    : { SUB2API_BASE_URL: baseURL, SUB2API_ADMIN_JWT: "offline-admin-jwt" };
}

async function temporaryStatePath(t) {
  const directory = await fs.mkdtemp(path.join(os.tmpdir(), "sub2api-maintenance-test-"));
  t.after(() => fs.rm(directory, { recursive: true, force: true }));
  return path.join(directory, "maintenance-announcements.json");
}

test("start creates an all-user active popup and persists its verified ID", async (t) => {
  const requests = [];
  const server = await startServer(async (request, response) => {
    requests.push({ method: request.method, url: request.url, headers: request.headers, body: await readRequest(request) });
    response.writeHead(200, { "content-type": "application/json" });
    response.end(success({ id: 41 }));
  });
  t.after(() => server.close());
  const statePath = await temporaryStatePath(t);
  const output = makeOutput();

  await main(["--event", "start", "--release", "0.1.2", "--eta-minutes", "5"], {
    environment: environment(server.baseURL), statePath, output,
  });

  assert.equal(output.value(), "maintenance_announcement_id=41\n");
  assert.deepEqual(requests[0], {
    method: "POST",
    url: "/api/v1/admin/announcements",
    headers: { ...requests[0].headers, "x-api-key": "offline-admin-key" },
    body: {
      title: "Sub2API 维护通知",
      content: "Sub2API 将进行维护。\n发布版本：0.1.2\n预计影响时长：5 分钟。",
      status: "active",
      notify_mode: "popup",
      targeting: {},
    },
  });
  const saved = JSON.parse(await fs.readFile(statePath, "utf8"));
  assert.equal(saved.announcements["0.1.2"].announcement_id, 41);
  assert.equal(saved.announcements["0.1.2"].status, "start");
});

test("complete archives the start popup and creates a verified recovery popup", async (t) => {
  const requests = [];
  const server = await startServer(async (request, response) => {
    const body = request.method === "GET" ? undefined : await readRequest(request);
    requests.push({ method: request.method, url: request.url, headers: request.headers, body });
    response.writeHead(200, { "content-type": "application/json" });
    if (request.method === "POST") response.end(success({ id: requests.length === 1 ? 51 : 52 }));
    else response.end(success({ id: 51 }));
  });
  t.after(() => server.close());
  const statePath = await temporaryStatePath(t);

  await main(["--event", "start", "--release", "0.1.3"], {
    environment: environment(server.baseURL), statePath, output: makeOutput(),
  });
  const output = makeOutput();
  await main(["--event", "complete", "--release", "0.1.3"], {
    environment: environment(server.baseURL, "jwt"), statePath, output,
  });

  assert.equal(requests[1].method, "PUT");
  assert.equal(requests[1].url, "/api/v1/admin/announcements/51");
  assert.deepEqual(requests[1].body, { status: "archived" });
  assert.equal(requests[1].headers.authorization, "Bearer offline-admin-jwt");
  assert.equal(requests[2].method, "POST");
  assert.equal(requests[2].body.status, "active");
  assert.equal(requests[2].body.notify_mode, "popup");
  assert.deepEqual(requests[2].body.targeting, {});
  assert.equal(output.value(), "maintenance_announcement_id=51 recovery_announcement_id=52\n");
});

test("failed updates the original popup to delayed and persists the same verified ID", async (t) => {
  let requests = 0;
  const server = await startServer(async (request, response) => {
    requests += 1;
    const body = await readRequest(request);
    response.writeHead(200, { "content-type": "application/json" });
    if (requests === 1) {
      response.end(success({ id: 71 }));
      return;
    }
    assert.equal(request.method, "PUT");
    assert.equal(request.url, "/api/v1/admin/announcements/71");
    assert.equal(body.title, "Sub2API 维护延迟");
    assert.match(body.content, /恢复时间另行通知/);
    response.end(success({ id: 71 }));
  });
  t.after(() => server.close());
  const statePath = await temporaryStatePath(t);

  await main(["--event", "start", "--release", "0.1.4"], {
    environment: environment(server.baseURL), statePath, output: makeOutput(),
  });
  const output = makeOutput();
  await main(["--event", "failed", "--release", "0.1.4"], {
    environment: environment(server.baseURL), statePath, output,
  });
  assert.equal(output.value(), "maintenance_announcement_id=71\n");
  const saved = JSON.parse(await fs.readFile(statePath, "utf8"));
  assert.equal(saved.announcements["0.1.4"].status, "failed");
});

test("an update response with a different ID is rejected without changing saved state", async (t) => {
  let requests = 0;
  const server = await startServer(async (request, response) => {
    requests += 1;
    await readRequest(request);
    response.writeHead(200, { "content-type": "application/json" });
    response.end(success({ id: requests === 1 ? 81 : 82 }));
  });
  t.after(() => server.close());
  const statePath = await temporaryStatePath(t);

  await main(["--event", "start", "--release", "0.1.5"], {
    environment: environment(server.baseURL), statePath, output: makeOutput(),
  });
  await assert.rejects(
    main(["--event", "failed", "--release", "0.1.5"], {
      environment: environment(server.baseURL), statePath, output: makeOutput(),
    }),
    /maintenance announcement operation failed/,
  );
  const saved = JSON.parse(await fs.readFile(statePath, "utf8"));
  assert.equal(saved.announcements["0.1.5"].status, "start");
});

const assert = require("node:assert/strict");
const test = require("node:test");

const {
  apiRequest,
  buildCompletePayload,
  buildStartPayload,
  parseArgs,
} = require("./notify-maintenance.js");

test("announcement API receives authentication and JSON payload", async () => {
  const originalFetch = global.fetch;
  const originalJwt = process.env.SUB2API_JWT;
  process.env.SUB2API_JWT = "test-jwt";
  delete process.env.SUB2API_ADMIN_API_KEY;
  global.fetch = async (url, options) => {
    assert.equal(url, "http://sub2api.test/api/v1/admin/announcements");
    assert.equal(options.method, "POST");
    assert.equal(options.headers.Authorization, "Bearer test-jwt");
    assert.deepEqual(JSON.parse(options.body), { title: "maintenance" });
    return new Response(JSON.stringify({ code: 0, data: { id: 101 } }), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    });
  };

  try {
    assert.deepEqual(
      await apiRequest("http://sub2api.test", "POST", "/api/v1/admin/announcements", {
        title: "maintenance",
      }),
      { id: 101 },
    );
  } finally {
    global.fetch = originalFetch;
    if (originalJwt === undefined) delete process.env.SUB2API_JWT;
    else process.env.SUB2API_JWT = originalJwt;
  }
});

test("start notice targets every user and uses popup mode", () => {
  const payload = buildStartPayload("v0.1.166", 5, 1000);
  assert.equal(payload.status, "active");
  assert.equal(payload.notify_mode, "popup");
  assert.deepEqual(payload.targeting, { any_of: [] });
  assert.equal(payload.starts_at, 1000);
  assert.match(payload.content, /5 分钟/);
});

test("completion notice expires after one day", () => {
  const payload = buildCompletePayload("v0.1.166", 2000);
  assert.equal(payload.notify_mode, "popup");
  assert.equal(payload.ends_at, 88400);
  assert.match(payload.content, /服务已恢复/);
});

test("CLI flags are parsed without evaluating their content", () => {
  assert.deepEqual(parseArgs(["--event", "start", "--release", "v0.1.166", "--dry-run"]), {
    event: "start",
    release: "v0.1.166",
    "dry-run": true,
  });
});

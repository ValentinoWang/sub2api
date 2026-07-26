#!/usr/bin/env node

const fs = require("fs");
const path = require("path");

const DEFAULT_STATE = path.resolve(
  process.env.SUB2API_MAINTENANCE_STATE ||
    ".artifacts/sub2api-deployment/maintenance-announcement.json",
);

function parseArgs(argv) {
  const flags = {};
  for (let index = 0; index < argv.length; index += 1) {
    const token = argv[index];
    if (!token.startsWith("--")) throw new Error(`unexpected argument: ${token}`);
    const key = token.slice(2);
    const value = argv[index + 1];
    if (!value || value.startsWith("--")) {
      flags[key] = true;
    } else {
      flags[key] = value;
      index += 1;
    }
  }
  return flags;
}

function requireText(value, name, maxLength = 200) {
  const text = String(value || "").trim();
  if (!text) throw new Error(`${name} is required`);
  if (text.length > maxLength) throw new Error(`${name} is too long`);
  return text;
}

function buildStartPayload(release, etaMinutes, startsAt) {
  return {
    title: "系统维护通知",
    content: `Sub2API 将进行 ${release} 更新，预计服务中断不超过 ${etaMinutes} 分钟。正在进行的流式请求可能断开，请在维护完成后重试。`,
    status: "active",
    notify_mode: "popup",
    targeting: { any_of: [] },
    starts_at: startsAt,
  };
}

function buildCompletePayload(release, startsAt) {
  return {
    title: "系统维护完成",
    content: `Sub2API ${release} 更新已经完成，服务已恢复。如刚才的请求中断，请重新发送。`,
    status: "active",
    notify_mode: "popup",
    targeting: { any_of: [] },
    starts_at: startsAt,
    ends_at: startsAt + 86400,
  };
}

function authHeaders() {
  const apiKey = process.env.SUB2API_ADMIN_API_KEY || "";
  const jwt = process.env.SUB2API_JWT || "";
  if (apiKey) return { "x-api-key": apiKey };
  if (jwt) return { Authorization: `Bearer ${jwt}` };
  throw new Error("SUB2API_ADMIN_API_KEY or SUB2API_JWT is required");
}

async function apiRequest(baseUrl, method, pathname, body) {
  const response = await fetch(`${baseUrl}${pathname}`, {
    method,
    headers: {
      ...authHeaders(),
      Accept: "application/json",
      "Content-Type": "application/json",
    },
    body: JSON.stringify(body),
  });
  const raw = await response.text();
  let parsed;
  try {
    parsed = JSON.parse(raw);
  } catch {
    throw new Error(`${method} ${pathname} returned invalid JSON`);
  }
  if (!response.ok || (parsed.code !== undefined && String(parsed.code) !== "0")) {
    throw new Error(`${method} ${pathname} failed: ${parsed.message || response.statusText}`);
  }
  return parsed.data;
}

function readState(statePath) {
  if (!fs.existsSync(statePath)) return null;
  return JSON.parse(fs.readFileSync(statePath, "utf8"));
}

function writeState(statePath, state) {
  fs.mkdirSync(path.dirname(statePath), { recursive: true });
  fs.writeFileSync(statePath, `${JSON.stringify(state, null, 2)}\n`, { mode: 0o600 });
}

async function main() {
  const flags = parseArgs(process.argv.slice(2));
  const event = requireText(flags.event, "--event", 20);
  if (!["start", "complete", "failed"].includes(event)) {
    throw new Error("--event must be start, complete, or failed");
  }

  const release = requireText(flags.release, "--release", 100);
  const now = Math.floor(Date.now() / 1000);
  const statePath = path.resolve(flags["state-file"] || DEFAULT_STATE);
  const state = readState(statePath);
  const baseUrl = String(process.env.SUB2API_BASE_URL || "").replace(/\/$/, "");

  if (event === "start") {
    const etaMinutes = Number(flags["eta-minutes"] || 5);
    if (!Number.isInteger(etaMinutes) || etaMinutes < 1 || etaMinutes > 180) {
      throw new Error("--eta-minutes must be an integer from 1 to 180");
    }
    if (state && ["start", "failed"].includes(state.phase) && !flags.force) {
      throw new Error("an unfinished maintenance announcement already exists; complete it or use --force");
    }
    const payload = buildStartPayload(release, etaMinutes, now);
    if (flags["dry-run"]) {
      process.stdout.write(`${JSON.stringify(payload, null, 2)}\n`);
      return;
    }
    if (!baseUrl) throw new Error("SUB2API_BASE_URL is required");
    const created = await apiRequest(baseUrl, "POST", "/api/v1/admin/announcements", payload);
    if (!created || !created.id) throw new Error("announcement API did not return an id");
    writeState(statePath, { phase: "start", release, announcement_id: created.id, started_at: now });
    process.stdout.write(`maintenance_announcement_id=${created.id}\n`);
    return;
  }

  if (!state || !state.announcement_id) {
    throw new Error("maintenance announcement state is missing; publish the start notice first");
  }
  if (!baseUrl) throw new Error("SUB2API_BASE_URL is required");

  if (event === "failed") {
    const details = requireText(flags.details || "维护尚未完成，恢复时间另行通知。", "--details", 500);
    await apiRequest(baseUrl, "PUT", `/api/v1/admin/announcements/${state.announcement_id}`, {
      title: "系统维护延期通知",
      content: `Sub2API ${release} 更新未能按计划完成。${details}`,
      status: "active",
      notify_mode: "popup",
      targeting: { any_of: [] },
      ends_at: 0,
    });
    writeState(statePath, { ...state, phase: "failed", release, failed_at: now });
    process.stdout.write(`maintenance_failed_announcement_id=${state.announcement_id}\n`);
    return;
  }

  await apiRequest(baseUrl, "PUT", `/api/v1/admin/announcements/${state.announcement_id}`, {
    status: "archived",
  });
  const created = await apiRequest(
    baseUrl,
    "POST",
    "/api/v1/admin/announcements",
    buildCompletePayload(release, now),
  );
  if (!created || !created.id) throw new Error("completion announcement API did not return an id");
  writeState(statePath, {
    ...state,
    phase: "complete",
    release,
    completed_at: now,
    completion_announcement_id: created.id,
  });
  process.stdout.write(`maintenance_complete_announcement_id=${created.id}\n`);
}

if (require.main === module) {
  main().catch((error) => {
    console.error(error.message);
    process.exit(1);
  });
}

module.exports = { apiRequest, buildCompletePayload, buildStartPayload, parseArgs };

#!/usr/bin/env node
"use strict";

const fs = require("node:fs/promises");
const path = require("node:path");

const STATE_VERSION = 1;
const DEFAULT_ETA_MINUTES = 5;
const DEFAULT_DELAY_DETAILS = "服务恢复时间另行通知。";
const ROOT_DIRECTORY = path.resolve(__dirname, "../../..");
const DEFAULT_STATE_PATH = path.join(
  ROOT_DIRECTORY,
  ".artifacts",
  "sub2api-deployment",
  "maintenance-announcements.json",
);

function fail() {
  return new Error("maintenance announcement operation failed");
}

function parseArguments(argv) {
  const values = {};
  for (let index = 0; index < argv.length; index += 1) {
    const option = argv[index];
    if (!option.startsWith("--")) throw fail();
    const name = option.slice(2);
    if (!new Set(["event", "release", "eta-minutes", "details"]).has(name) || index + 1 >= argv.length) {
      throw fail();
    }
    if (values[name] !== undefined) throw fail();
    values[name] = argv[index + 1];
    index += 1;
  }

  if (!new Set(["start", "complete", "failed"]).has(values.event)) throw fail();
  if (!/^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$/.test(values.release || "")) throw fail();
  if (values["eta-minutes"] !== undefined && !/^[1-9][0-9]{0,3}$/.test(values["eta-minutes"])) throw fail();
  if (values.details !== undefined && (values.details.length === 0 || values.details.length > 1000)) throw fail();

  return {
    event: values.event,
    release: values.release,
    etaMinutes: Number(values["eta-minutes"] || DEFAULT_ETA_MINUTES),
    details: values.details || DEFAULT_DELAY_DETAILS,
  };
}

function readConfiguration(environment) {
  const baseURL = String(environment.SUB2API_BASE_URL || "").trim();
  const apiKey = String(environment.SUB2API_ADMIN_API_KEY || "").trim();
  const jwt = String(environment.SUB2API_ADMIN_JWT || "").trim();
  if (!baseURL || (apiKey && jwt) || (!apiKey && !jwt)) throw fail();

  let parsed;
  try {
    parsed = new URL(baseURL);
  } catch {
    throw fail();
  }
  if (
    (parsed.protocol !== "https:" && parsed.protocol !== "http:") ||
    parsed.username ||
    parsed.password ||
    parsed.search ||
    parsed.hash
  ) {
    throw fail();
  }

  return {
    apiBase: new URL("/api/v1/", parsed).toString(),
    headers: apiKey ? { "x-api-key": apiKey } : { authorization: `Bearer ${jwt}` },
  };
}

function parseAnnouncementID(value) {
  const id = typeof value === "number" ? value : Number(value);
  if (!Number.isSafeInteger(id) || id <= 0) throw fail();
  return id;
}

async function request(configuration, fetchImpl, method, relativePath, body, expectedID) {
  let response;
  try {
    response = await fetchImpl(new URL(relativePath, configuration.apiBase), {
      method,
      headers: { ...configuration.headers, "content-type": "application/json", accept: "application/json" },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
  } catch {
    throw fail();
  }

  if (!response || !response.ok) throw fail();
  let payload;
  try {
    payload = await response.json();
  } catch {
    throw fail();
  }
  if (!payload || payload.code !== 0 || !payload.data || typeof payload.data !== "object") throw fail();
  const id = parseAnnouncementID(payload.data.id);
  if (expectedID !== undefined && id !== expectedID) throw fail();
  return payload.data;
}

function requireAnnouncement(value, expected) {
  if (!value || typeof value !== "object" || Array.isArray(value)) throw fail();
  const id = parseAnnouncementID(value.id);
  if (expected.id !== undefined && id !== expected.id) throw fail();
  if (expected.status !== undefined && value.status !== expected.status) throw fail();
  if (expected.notifyMode !== undefined && value.notify_mode !== expected.notifyMode) throw fail();
  if (expected.allUsers) {
    if (!value.targeting || typeof value.targeting !== "object" || Array.isArray(value.targeting)) throw fail();
    if (Object.keys(value.targeting).length !== 0) throw fail();
  }
  return id;
}

async function verifyAnnouncement(configuration, fetchImpl, id, expected) {
  const value = await request(configuration, fetchImpl, "GET", `admin/announcements/${id}`, undefined, id);
  return requireAnnouncement(value, { ...expected, id });
}

async function readState(statePath) {
  try {
    const raw = await fs.readFile(statePath, "utf8");
    const parsed = JSON.parse(raw);
    if (
      !parsed ||
      parsed.version !== STATE_VERSION ||
      !parsed.announcements ||
      typeof parsed.announcements !== "object" ||
      Array.isArray(parsed.announcements)
    ) {
      throw fail();
    }
    return parsed;
  } catch (error) {
    if (error && error.code === "ENOENT") return { version: STATE_VERSION, announcements: {} };
    throw fail();
  }
}

async function writeState(statePath, state) {
  const directory = path.dirname(statePath);
  const temporaryPath = `${statePath}.${process.pid}.tmp`;
  try {
    await fs.mkdir(directory, { recursive: true, mode: 0o700 });
    await fs.writeFile(temporaryPath, `${JSON.stringify(state, null, 2)}\n`, { encoding: "utf8", mode: 0o600 });
    await fs.chmod(temporaryPath, 0o600);
    await fs.rename(temporaryPath, statePath);
  } catch {
    try {
      await fs.unlink(temporaryPath);
    } catch {
      // No secret is written to this file, and cleanup failure is not actionable here.
    }
    throw fail();
  }
}

function startAnnouncement(input) {
  return {
    title: "Sub2API 维护通知",
    content: `Sub2API 将进行维护。\n发布版本：${input.release}\n预计影响时长：${input.etaMinutes} 分钟。`,
    status: "active",
    notify_mode: "popup",
    targeting: {},
  };
}

function recoveryAnnouncement(input) {
  return {
    title: "Sub2API 服务已恢复",
    content: `Sub2API 维护已完成。\n发布版本：${input.release}\n服务已恢复。`,
    status: "active",
    notify_mode: "popup",
    targeting: {},
  };
}

function delayedAnnouncement(input) {
  return {
    title: "Sub2API 维护延迟",
    content: `Sub2API 维护尚未完成。\n发布版本：${input.release}\n${input.details}`,
    status: "active",
    notify_mode: "popup",
    targeting: {},
  };
}

async function main(argv = process.argv.slice(2), dependencies = {}) {
  const input = parseArguments(argv);
  const configuration = readConfiguration(dependencies.environment || process.env);
  const fetchImpl = dependencies.fetchImpl || globalThis.fetch;
  const statePath = dependencies.statePath || DEFAULT_STATE_PATH;
  const output = dependencies.output || process.stdout;
  if (typeof fetchImpl !== "function" || !output || typeof output.write !== "function") throw fail();

  const state = await readState(statePath);
  const existing = state.announcements[input.release];

  if (input.event === "start") {
    if (existing && existing.status === "start") {
      const id = parseAnnouncementID(existing.announcement_id);
      await verifyAnnouncement(configuration, fetchImpl, id, {
        status: "active",
        notifyMode: "popup",
        allUsers: true,
      });
      output.write(`maintenance_announcement_id=${id}\n`);
      return { maintenanceAnnouncementID: id };
    }
    if (existing && existing.status === "failed") {
      const failedID = parseAnnouncementID(existing.announcement_id);
      const archived = await request(
        configuration,
        fetchImpl,
        "PUT",
        `admin/announcements/${failedID}`,
        { status: "archived" },
        failedID,
      );
      requireAnnouncement(archived, { id: failedID, status: "archived" });
      await verifyAnnouncement(configuration, fetchImpl, failedID, { status: "archived" });
    }
    const created = await request(configuration, fetchImpl, "POST", "admin/announcements", startAnnouncement(input));
    const id = requireAnnouncement(created, {
      status: "active",
      notifyMode: "popup",
      allUsers: true,
    });
    await verifyAnnouncement(configuration, fetchImpl, id, {
      status: "active",
      notifyMode: "popup",
      allUsers: true,
    });
    state.announcements[input.release] = {
      announcement_id: id,
      status: "start",
      started_at: new Date().toISOString(),
    };
    await writeState(statePath, state);
    output.write(`maintenance_announcement_id=${id}\n`);
    return { maintenanceAnnouncementID: id };
  }

  if (!existing) throw fail();
  const id = parseAnnouncementID(existing.announcement_id);

  if (input.event === "complete") {
    const archived = await request(configuration, fetchImpl, "PUT", `admin/announcements/${id}`, { status: "archived" }, id);
    requireAnnouncement(archived, { id, status: "archived" });
    await verifyAnnouncement(configuration, fetchImpl, id, { status: "archived" });
    const recovered = await request(configuration, fetchImpl, "POST", "admin/announcements", recoveryAnnouncement(input));
    const recoveryID = requireAnnouncement(recovered, {
      status: "active",
      notifyMode: "popup",
      allUsers: true,
    });
    await verifyAnnouncement(configuration, fetchImpl, recoveryID, {
      status: "active",
      notifyMode: "popup",
      allUsers: true,
    });
    state.announcements[input.release] = {
      announcement_id: id,
      recovery_announcement_id: recoveryID,
      status: "complete",
      completed_at: new Date().toISOString(),
    };
    await writeState(statePath, state);
    output.write(`maintenance_announcement_id=${id} recovery_announcement_id=${recoveryID}\n`);
    return { maintenanceAnnouncementID: id, recoveryAnnouncementID: recoveryID };
  }

  const delayed = await request(configuration, fetchImpl, "PUT", `admin/announcements/${id}`, delayedAnnouncement(input), id);
  requireAnnouncement(delayed, { id, status: "active", notifyMode: "popup", allUsers: true });
  await verifyAnnouncement(configuration, fetchImpl, id, {
    status: "active",
    notifyMode: "popup",
    allUsers: true,
  });
  state.announcements[input.release] = {
    announcement_id: id,
    status: "failed",
    failed_at: new Date().toISOString(),
  };
  await writeState(statePath, state);
  output.write(`maintenance_announcement_id=${id}\n`);
  return { maintenanceAnnouncementID: id };
}

if (require.main === module) {
  main().catch(() => {
    process.stderr.write("maintenance announcement operation failed\n");
    process.exitCode = 1;
  });
}

module.exports = { DEFAULT_STATE_PATH, main };

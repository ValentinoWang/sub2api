import assert from "node:assert/strict";
import type { Server } from "node:http";
import type { AddressInfo } from "node:net";

import { readConfig } from "../config";
import { launchRuntime, MembershipRunner } from "../runner";
import { createMembershipServer } from "../server";
import { startFixture, type Fixture } from "./fixture";

const token = "test-token-012345678901234567890123456789";

function closeServer(server: Server): Promise<void> {
  server.closeAllConnections();
  return new Promise<void>((resolve, reject) => {
    server.close((error?: Error) => error ? reject(error) : resolve());
  });
}

async function withService(
  runtime: Awaited<ReturnType<typeof launchRuntime>>,
  fixture: Fixture,
  scenario: string,
  body: Record<string, unknown>,
  requestToken = token,
): Promise<{ status: number; result: Record<string, unknown>; fixture: Fixture }> {
  const config = readConfig({
    MEMBERSHIP_BROWSER_LISTEN_HOST: "127.0.0.1",
    MEMBERSHIP_BROWSER_LISTEN_PORT: "0",
    MEMBERSHIP_BROWSER_BEARER_TOKEN: token,
    MEMBERSHIP_BROWSER_ALLOWED_HOSTS: "127.0.0.1",
    MEMBERSHIP_BROWSER_GPT_BASE_URL: `${fixture.baseURL}/${scenario}/gpt`,
    MEMBERSHIP_BROWSER_GPTPRO_BASE_URL: `${fixture.baseURL}/${scenario}/gptpro`,
  });
  const server = createMembershipServer(config, new MembershipRunner(config, runtime));
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  server.unref();
  const address = server.address() as AddressInfo;
  try {
    const response = await fetch(`http://127.0.0.1:${address.port}/execute`, {
      method: "POST",
      headers: { authorization: `Bearer ${requestToken}`, connection: "close", "content-type": "application/json" },
      body: JSON.stringify(body),
    });
    return { status: response.status, result: await response.json() as Record<string, unknown>, fixture };
  } finally {
    await closeServer(server);
  }
}

async function successAndValidation(runtime: Awaited<ReturnType<typeof launchRuntime>>): Promise<void> {
  const fixture = await startFixture("shared");
  try {
  const availability = await withService(runtime, fixture, "success", { operation: "checkAvailability", channel: "gpt", sku: "fixture-sku" });
  assert.equal(availability.status, 200);
  assert.deepEqual(availability.result, { state: "available", error_code: "", available: 4 });

  const credential = await withService(runtime, fixture, "success", {
    operation: "validateCredential", channel: "gpt", sku: "fixture-sku", period_days: 30,
    credential: { mode: "account_id", value: "fixture-secret", account_id: "acct_12345678" },
  });
  assert.equal(credential.result.state, "valid");
  assert.equal(credential.result.account_id, "acct_12345678");
  } finally {
    await fixture.close();
  }
}

async function safeFailures(runtime: Awaited<ReturnType<typeof launchRuntime>>): Promise<void> {
  const fixture = await startFixture("shared");
  try {
  const failure = await withService(runtime, fixture, "failure", { operation: "validateCredential", channel: "gpt", sku: "fixture-sku", period_days: 30, credential: { mode: "account_id", value: "secret", account_id: "acct_12345678" } });
  assert.equal(failure.result.error_code, "ACCOUNT_NOT_ELIGIBLE");

  const captcha = await withService(runtime, fixture, "captcha", { operation: "checkAvailability", channel: "gpt" });
  assert.deepEqual(captcha.result, { state: "not_submitted", error_code: "CAPTCHA_REQUIRED", not_submitted: true });

  const drift = await withService(runtime, fixture, "drift", { operation: "checkAvailability", channel: "gpt" });
  assert.deepEqual(drift.result, { state: "not_submitted", error_code: "PAGE_CHANGED", not_submitted: true });
  } finally {
    await fixture.close();
  }
}

async function submitAndStatus(runtime: Awaited<ReturnType<typeof launchRuntime>>): Promise<void> {
  const fixture = await startFixture("shared");
  try {
  const submitted = await withService(runtime, fixture, "unknown-submit", { operation: "submitRecharge", channel: "gpt", attempt_id: "attempt-123", cdk: "fixture-cdk" });
  assert.deepEqual(submitted.result, { state: "review_required", error_code: "REVIEW_REQUIRED" });

  const queried = await withService(runtime, fixture, "query-success", { operation: "queryStatus", channel: "gpt", attempt_id: "attempt-123", upstream_task_id: "fixture-task", cdk: "fixture-cdk" });
  assert.equal(queried.result.state, "succeeded");
  assert.equal(queried.result.entitlement_verified, true);
  assert.deepEqual(fixture.requests, [{ path: "/api/card-recharge/query", body: '{"cdks":"fixture-cdk"}' }]);

  const productionConfig = readConfig({
    MEMBERSHIP_BROWSER_LISTEN_HOST: "127.0.0.1", MEMBERSHIP_BROWSER_LISTEN_PORT: "0",
    MEMBERSHIP_BROWSER_BEARER_TOKEN: token, MEMBERSHIP_BROWSER_ALLOWED_HOSTS: "supplier.example.invalid",
    MEMBERSHIP_BROWSER_GPT_BASE_URL: "https://supplier.example.invalid/gpt",
    MEMBERSHIP_BROWSER_GPTPRO_BASE_URL: "https://supplier.example.invalid/gptpro",
  });
  const disabled = await new MembershipRunner(productionConfig, runtime).execute({ operation: "submitRecharge", channel: "gpt", attempt_id: "attempt-123", cdk: "fixture-cdk" });
  assert.deepEqual(disabled, { state: "review_required", error_code: "NOT_SUPPORTED", not_submitted: true });
  } finally {
    await fixture.close();
  }
}

async function queryStatusSafety(runtime: Awaited<ReturnType<typeof launchRuntime>>): Promise<void> {
  const fixture = await startFixture("shared");
  try {
    const base = { operation: "queryStatus", attempt_id: "attempt-123", upstream_task_id: "fixture-task", cdk: "fixture-cdk" };
    const pending = await withService(runtime, fixture, "query-pending", { ...base, channel: "gpt" });
    assert.deepEqual(pending.result, { state: "processing", error_code: "PROCESSING" });

    const failed = await withService(runtime, fixture, "query-failed", { ...base, channel: "gpt" });
    assert.deepEqual(failed.result, { state: "failed", error_code: "FULFILLMENT_FAILED" });

    const noEntitlement = await withService(runtime, fixture, "query-no-entitlement", { ...base, channel: "gpt" });
    assert.deepEqual(noEntitlement.result, { state: "review_required", error_code: "REVIEW_REQUIRED" });

    const external = await withService(runtime, fixture, "external-response", { ...base, channel: "gpt" });
    assert.deepEqual(external.result, { state: "review_required", error_code: "REVIEW_REQUIRED" });
    assert.equal(fixture.externalRequests, 1);

    const gptproPending = await withService(runtime, fixture, "query-pending", { ...base, channel: "gptpro" });
    assert.deepEqual(gptproPending.result, { state: "processing", error_code: "PROCESSING" });

    const gptproFailed = await withService(runtime, fixture, "query-failed", { ...base, channel: "gptpro" });
    assert.deepEqual(gptproFailed.result, { state: "failed", error_code: "FULFILLMENT_FAILED" });

    const gptproNoEntitlement = await withService(runtime, fixture, "query-no-entitlement", { ...base, channel: "gptpro" });
    assert.deepEqual(gptproNoEntitlement.result, { state: "review_required", error_code: "REVIEW_REQUIRED" });

    const gptpro = await withService(runtime, fixture, "query-success", { ...base, channel: "gptpro" });
    assert.deepEqual(gptpro.result, { state: "succeeded", error_code: "", entitlement_verified: true });
    assert.deepEqual(fixture.requests.slice(1), [
      { path: "/api/card-recharge/query", body: '{"cdks":"fixture-cdk"}' },
      { path: "/api/card-recharge/query", body: '{"cdks":"fixture-cdk"}' },
      { path: "/api/gpt-source/manual/check", body: '{"cdkey":"FIXTURE-CDK"}' },
      { path: "/api/gpt-source/manual/check", body: '{"cdkey":"FIXTURE-CDK"}' },
      { path: "/api/gpt-source/manual/check", body: '{"cdkey":"FIXTURE-CDK"}' },
      { path: "/api/gpt-source/manual/check", body: '{"cdkey":"FIXTURE-CDK"}' },
    ]);
  } finally {
    await fixture.close();
  }
}

async function isolationAndAuthentication(runtime: Awaited<ReturnType<typeof launchRuntime>>): Promise<void> {
  const fixture = await startFixture("shared");
  try {
  const first = await withService(runtime, fixture, "success", { operation: "checkAvailability", channel: "gpt" });
  const second = await withService(runtime, fixture, "success", { operation: "checkAvailability", channel: "gpt" });
  assert.deepEqual(first.fixture.cookies, ["", ""]);
  assert.deepEqual(second.fixture.cookies, ["", ""]);

  const unsupported = await withService(runtime, fixture, "success", { operation: "cancel", channel: "gpt" });
  assert.deepEqual(unsupported.result, { state: "failed", error_code: "NOT_SUPPORTED", not_submitted: true });

  const rejected = await withService(runtime, fixture, "success", { operation: "checkAvailability", channel: "gpt" }, "wrong-token");
  assert.equal(rejected.status, 401);
  assert.deepEqual(rejected.result, { error_code: "UNAUTHORIZED" });
  } finally {
    await fixture.close();
  }
}

async function main(): Promise<void> {
  const runtime = await launchRuntime();
  try {
    process.stdout.write("membership browser tests: success and validation\n");
    await successAndValidation(runtime);
    process.stdout.write("membership browser tests: safe failures\n");
    await safeFailures(runtime);
    process.stdout.write("membership browser tests: submit and status\n");
    await submitAndStatus(runtime);
    process.stdout.write("membership browser tests: query status safety\n");
    await queryStatusSafety(runtime);
    process.stdout.write("membership browser tests: isolation and authentication\n");
    await isolationAndAuthentication(runtime);
    process.stdout.write("membership browser tests: all offline scenario groups passed\n");
  } finally {
    await runtime.close();
  }
}

void main().catch((error: unknown) => {
  process.stderr.write(`${error instanceof Error ? error.stack ?? error.message : "test failure"}\n`);
  process.exitCode = 1;
});

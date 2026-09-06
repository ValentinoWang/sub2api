import { chromium, type Browser, type Page, type Response } from "playwright";

import { isLoopback, type Config } from "./config";
import type { BrowserInput, BrowserResult, Channel } from "./contracts";
import { notSubmitted, reviewRequired } from "./contracts";

const pageTimeoutMs = 10_000;

export interface BrowserRuntime {
  newPage(): Promise<{ page: Page; close(): Promise<void> }>;
  close(): Promise<void>;
}

export async function launchRuntime(): Promise<BrowserRuntime> {
  const browser: Browser = await chromium.launch({ headless: true });
  return {
    async newPage() {
      // A non-persistent context prevents credentials and cookies from crossing requests.
      const context = await browser.newContext({ acceptDownloads: false, serviceWorkers: "block" });
      return { page: await context.newPage(), close: () => context.close() };
    },
    close: () => browser.close(),
  };
}

function supplierResult(value: string | null): BrowserResult["error_code"] {
  switch (value) {
    case "":
    case "CREDENTIAL_EXPIRED":
    case "ACCOUNT_NOT_ELIGIBLE":
    case "OUT_OF_STOCK":
    case "PROCESSING":
    case "REVIEW_REQUIRED":
    case "FULFILLMENT_FAILED":
    case "PRODUCT_MISMATCH":
      return value;
    default:
      return "REVIEW_REQUIRED";
  }
}

function safeString(value: unknown, maximum: number): string | undefined {
  return typeof value === "string" && value.length <= maximum ? value : undefined;
}

function record(value: unknown): Record<string, unknown> | undefined {
  return value !== null && typeof value === "object" && !Array.isArray(value) ? value as Record<string, unknown> : undefined;
}

function nonEmptyString(value: unknown, maximum = 256): string | undefined {
  const candidate = safeString(value, maximum)?.trim();
  return candidate ? candidate : undefined;
}

function normalized(value: unknown): string | undefined {
  return nonEmptyString(value)?.toUpperCase();
}

function lifecycle(value: unknown): "processing" | "failed" | "succeeded" | undefined {
  switch (nonEmptyString(value)?.toLowerCase()) {
    case "pending":
    case "processing":
    case "queued":
      return "processing";
    case "failed":
    case "failure":
    case "error":
      return "failed";
    case "completed":
    case "complete":
    case "success":
    case "succeeded":
      return "succeeded";
    default:
      return undefined;
  }
}

function hasEntitlementEvidence(value: Record<string, unknown>): boolean {
  const entitlement = record(value.entitlement);
  if (!entitlement || entitlement.active !== true) return false;
  return Boolean(
    nonEmptyString(entitlement.plan ?? entitlement.plan_type ?? entitlement.planType)
    && nonEmptyString(entitlement.expires_at ?? entitlement.expiresAt)
    && nonEmptyString(entitlement.account_id ?? entitlement.accountId),
  );
}

export class MembershipRunner {
  public constructor(private readonly config: Config, private readonly runtime: BrowserRuntime) {}

  public async execute(input: BrowserInput): Promise<BrowserResult> {
    if (input.operation === "cancel" || input.operation === "refresh") {
      return { state: "failed", error_code: "NOT_SUPPORTED", not_submitted: true };
    }
    if (!isInputValid(input)) return reviewRequired();
    if (input.operation === "submitRecharge" && !this.canSubmit(input.channel)) {
      return { state: "review_required", error_code: "NOT_SUPPORTED", not_submitted: true };
    }

    const session = await this.runtime.newPage();
    try {
      const page = session.page;
      await page.goto(this.config.bases[input.channel].href, { waitUntil: "domcontentloaded", timeout: pageTimeoutMs });
      if (!await this.isExpectedPage(page, input.channel)) return notSubmitted("PAGE_CHANGED");
      if (await page.locator("[data-membership-captcha], iframe[src*='captcha' i]").count() > 0) return notSubmitted("CAPTCHA_REQUIRED");
      if (await page.locator("[data-membership-login-required]").count() > 0) return notSubmitted("LOGIN_REQUIRED");

      switch (input.operation) {
        case "checkAvailability":
          return await this.readAvailability(page);
        case "validateCredential":
          return await this.validateCredential(page, input);
        case "submitRecharge":
          return await this.submitRecharge(page, input);
        case "queryStatus":
          return await this.queryStatus(page, input);
      }
    } catch {
      return reviewRequired();
    } finally {
      await session.close();
    }
  }

  private async isExpectedPage(page: Page, channel: Channel): Promise<boolean> {
    const declared = await page.locator("[data-membership-channel]").getAttribute("data-membership-channel");
    if (!this.isWithinSupplierBase(page.url(), channel)) return false;
    if (declared) return declared === channel;
    return channel === "gpt"
      ? await page.locator(".card-channel2-page").count() === 1
      : await page.locator(".gpt-manual-console").count() === 1;
  }

  private async readAvailability(page: Page): Promise<BrowserResult> {
    const raw = await page.locator("[data-testid='availability']").getAttribute("data-available");
    const available = Number.parseInt(raw ?? "", 10);
    return Number.isSafeInteger(available) && available >= 0
      ? { state: "available", error_code: "", available }
      : notSubmitted("PAGE_CHANGED");
  }

  private async validateCredential(page: Page, input: BrowserInput): Promise<BrowserResult> {
    if (!input.credential) return reviewRequired();
    await page.getByTestId("credential").fill(input.credential.value);
    await page.getByTestId("account-id").fill(input.credential.account_id);
    await page.getByTestId("validate").click({ timeout: pageTimeoutMs });
    return this.readSupplierResult(page, input);
  }

  private async submitRecharge(page: Page, input: BrowserInput): Promise<BrowserResult> {
    const cdk = input.cdk;
    if (!cdk) return reviewRequired();
    if (!isLoopback(this.config.bases[input.channel])) return this.submitApprovedSupplier(page, input);
    await page.getByTestId("cdk").fill(cdk);
    await page.getByTestId("submit").click({ timeout: pageTimeoutMs });
    if (!this.isWithinSupplierBase(page.url(), input.channel)) return reviewRequired();
    return this.readSupplierResult(page, input);
  }

  private canSubmit(channel: Channel): boolean {
    const base = this.config.bases[channel];
    return isLoopback(base) || (this.config.productionSubmitEnabled && this.config.confirmedSupplierHost === base.hostname.toLowerCase());
  }

  private async submitApprovedSupplier(page: Page, input: BrowserInput): Promise<BrowserResult> {
    // The page contracts below are observed from the approved /gpt and /gptpro bundles.
    // Credential absence is fail-closed because a browser context is never reused.
    const cdk = input.cdk;
    if (!input.credential || !cdk) return notSubmitted("LOGIN_REQUIRED");
    const channel = input.channel as Channel;
    const endpoint = channel === "gpt" ? "/api/card-recharge/redeem" : "/api/gpt-source/manual/submit";
    const responsePromise = page.waitForResponse((response) => {
      const url = new URL(response.url());
      return response.request().method() === "POST" && url.origin === this.config.bases[channel].origin && url.pathname === endpoint;
    }, { timeout: pageTimeoutMs });
    if (channel === "gpt") {
      await page.locator(".card-channel2-page input").first().fill(cdk);
      if (input.credential.mode === "account_id") {
        await page.getByRole("tab", { name: /稳定版.*Account ID/ }).click();
        await page.locator(".card-channel2-account-row input").fill(input.credential.account_id);
      } else {
        await page.locator(".card-channel2-page textarea").fill(input.credential.value);
      }
      await page.locator("button.card-channel2-submit").click();
    } else {
      await page.getByPlaceholder("请输入排单 CDK").fill(cdk);
      await page.getByRole("button", { name: "查询", exact: true }).click();
      await page.getByPlaceholder("粘贴从 ChatGPT 账号页面获取的完整 Session").fill(input.credential.value);
      await page.getByRole("button", { name: "提交排单", exact: true }).click();
    }
    const response = await responsePromise;
    if (!response.ok()) return reviewRequired();
    const body: unknown = await response.json().catch(() => null);
    const record = body && typeof body === "object" ? body as Record<string, unknown> : null;
    const taskID = safeString(record?.taskId ?? record?.task_id ?? record?.id, 256);
    const success = record?.success;
    if (success === false) return { state: "failed", error_code: "FULFILLMENT_FAILED" };
    return { state: taskID ? "submitted" : "review_required", error_code: taskID ? "" : "REVIEW_REQUIRED", upstream_task_id: taskID };
  }

  private isWithinSupplierBase(candidate: string, channel: Channel): boolean {
    const current = new URL(candidate);
    const expected = this.config.bases[channel];
    const prefix = expected.pathname.endsWith("/") ? expected.pathname : `${expected.pathname}/`;
    return current.origin === expected.origin
      && (current.pathname === expected.pathname || current.pathname.startsWith(prefix))
      && this.config.allowedHosts.has(current.hostname.toLowerCase());
  }

  private async queryStatus(page: Page, input: BrowserInput): Promise<BrowserResult> {
    const cdk = input.cdk;
    if (!cdk) return reviewRequired();
    const channel = input.channel;
    const endpoint = channel === "gpt" ? "/api/card-recharge/query" : "/api/gpt-source/manual/check";
    const responsePromise = this.waitForQueryResponse(page, endpoint).catch(() => undefined);

    if (channel === "gpt") {
      await page.getByRole("button", { name: /查询卡密状态/ }).first().click({ timeout: pageTimeoutMs });
      const dialog = page.getByRole("dialog");
      await dialog.waitFor({ state: "visible", timeout: pageTimeoutMs });
      await dialog.locator("textarea").fill(cdk);
      await dialog.getByRole("button", { name: "查询", exact: true }).click({ timeout: pageTimeoutMs });
    } else {
      await page.getByPlaceholder("请输入排单 CDK").fill(cdk);
      await page.getByRole("button", { name: "查询", exact: true }).click({ timeout: pageTimeoutMs });
    }

    const response = await responsePromise;
    if (!response) return reviewRequired();
    const responseURL = new URL(response.url());
    if (responseURL.origin !== this.config.bases[channel].origin || !this.config.allowedHosts.has(responseURL.hostname.toLowerCase())) {
      return reviewRequired();
    }
    if (!response.ok()) return reviewRequired();
    const body: unknown = await response.json().catch(() => null);
    return channel === "gpt" ? this.parseGPTQuery(body, cdk) : this.parseGPTProQuery(body);
  }

  private async waitForQueryResponse(page: Page, endpoint: string): Promise<Response> {
    return page.waitForResponse((response) => {
      const responseURL = new URL(response.url());
      return response.request().method() === "POST" && responseURL.pathname === endpoint;
    }, { timeout: pageTimeoutMs });
  }

  private parseGPTQuery(body: unknown, cdk: string): BrowserResult {
    const response = record(body);
    const data = record(response?.data);
    if (response?.success !== true || !data || !Array.isArray(data.results)) return reviewRequired();
    const expectedCDK = normalized(cdk);
    const matches = data.results
      .map(record)
      .filter((entry): entry is Record<string, unknown> => Boolean(entry))
      .filter((entry) => normalized(entry.cdk ?? record(entry.info)?.cdk) === expectedCDK);
    if (!expectedCDK || matches.length !== 1) return reviewRequired();

    const entry = matches[0];
    const info = record(entry.info);
    const state = lifecycle(entry.status ?? info?.status ?? info?.task_status ?? info?.taskStatus);
    if (state === "processing") return { state: "processing", error_code: "PROCESSING" };
    if (state === "failed") return { state: "failed", error_code: "FULFILLMENT_FAILED" };
    if (state !== "succeeded" || !info || !hasEntitlementEvidence(info)) return reviewRequired();
    return { state: "succeeded", error_code: "", entitlement_verified: true };
  }

  private parseGPTProQuery(body: unknown): BrowserResult {
    const response = record(body);
    const data = record(response?.data);
    if (response?.success !== true || !data) return reviewRequired();
    const rawStatus = data.status;
    if (rawStatus === -2 || rawStatus === -1 || lifecycle(rawStatus) === "processing") {
      return { state: "processing", error_code: "PROCESSING" };
    }
    if (lifecycle(rawStatus) === "failed") return { state: "failed", error_code: "FULFILLMENT_FAILED" };
    if (rawStatus !== 1 && lifecycle(rawStatus) !== "succeeded") return reviewRequired();
    if (!hasEntitlementEvidence(data)) return reviewRequired();
    return { state: "succeeded", error_code: "", entitlement_verified: true };
  }

  private async readSupplierResult(page: Page, input: BrowserInput): Promise<BrowserResult> {
    const result = page.locator("[data-testid='result']");
    const state = await result.getAttribute("data-state");
    const errorCode = supplierResult(await result.getAttribute("data-error-code"));
    if (!state || !["valid", "submitted", "processing", "succeeded", "failed"].includes(state)) return notSubmitted("PAGE_CHANGED");
    const upstreamTaskID = safeString(await result.getAttribute("data-upstream-task-id"), 256);
    const accountID = safeString(await result.getAttribute("data-account-id"), 128);
    const sku = safeString(await result.getAttribute("data-sku"), 128);
    const rawDays = await result.getAttribute("data-period-days");
    const periodDays = Number.parseInt(rawDays ?? "", 10);
    const entitlementVerified = (await result.getAttribute("data-entitlement-verified")) === "true";
    if (input.operation === "validateCredential" && state === "valid" && (accountID !== input.credential?.account_id || sku !== input.sku || periodDays !== input.period_days)) {
      return notSubmitted("PRODUCT_MISMATCH");
    }
    return {
      state: state as BrowserResult["state"],
      error_code: errorCode,
      upstream_task_id: upstreamTaskID,
      account_id: accountID,
      sku,
      period_days: Number.isSafeInteger(periodDays) ? periodDays : undefined,
      entitlement_verified: entitlementVerified,
    };
  }
}

function isInputValid(input: BrowserInput): boolean {
  if (!input || (input.channel !== "gpt" && input.channel !== "gptpro")) return false;
  if (input.operation === "validateCredential") return Boolean(input.credential && input.sku && input.period_days && input.credential.value.length <= 65_536);
  if (input.operation === "submitRecharge") return Boolean(input.cdk && input.attempt_id);
  if (input.operation === "queryStatus") return Boolean(input.attempt_id && input.upstream_task_id && input.cdk);
  return input.operation === "checkAvailability";
}

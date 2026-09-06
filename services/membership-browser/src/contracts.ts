export type Channel = "gpt" | "gptpro";
export type Operation =
  | "checkAvailability"
  | "validateCredential"
  | "submitRecharge"
  | "queryStatus"
  | "cancel"
  | "refresh";

export interface Credential {
  mode: "account_id" | "session";
  value: string;
  account_id: string;
}

export interface BrowserInput {
  operation: Operation;
  attempt_id?: string;
  channel: Channel;
  sku?: string;
  period_days?: number;
  cdk?: string;
  credential?: Credential;
  upstream_task_id?: string;
}

export interface BrowserResult {
  state: "available" | "valid" | "submitted" | "processing" | "succeeded" | "failed" | "review_required" | "not_submitted";
  error_code: "" | "CREDENTIAL_EXPIRED" | "ACCOUNT_NOT_ELIGIBLE" | "OUT_OF_STOCK" | "PROCESSING" | "REVIEW_REQUIRED" | "FULFILLMENT_FAILED" | "NOT_SUPPORTED" | "CHANNEL_UNAVAILABLE" | "PAGE_CHANGED" | "LOGIN_REQUIRED" | "CAPTCHA_REQUIRED" | "PRODUCT_MISMATCH";
  upstream_task_id?: string;
  available?: number;
  sku?: string;
  period_days?: number;
  account_id?: string;
  not_submitted?: boolean;
  entitlement_verified?: boolean;
}

export const reviewRequired = (): BrowserResult => ({ state: "review_required", error_code: "REVIEW_REQUIRED" });
export const notSubmitted = (errorCode: BrowserResult["error_code"]): BrowserResult => ({
  state: "not_submitted",
  error_code: errorCode,
  not_submitted: true,
});

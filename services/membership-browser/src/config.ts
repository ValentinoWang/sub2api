import { isIP } from "node:net";

import type { Channel } from "./contracts";

export interface Config {
  listenHost: string;
  listenPort: number;
  bearerToken: string;
  allowedHosts: ReadonlySet<string>;
  bases: Record<Channel, URL>;
  productionSubmitEnabled: boolean;
  confirmedSupplierHost: string;
}

function required(environment: NodeJS.ProcessEnv, name: string): string {
  const value = environment[name]?.trim();
  if (!value) throw new Error(`${name} is required`);
  return value;
}

function parseBase(value: string, allowedHosts: ReadonlySet<string>): URL {
  const url = new URL(value);
  if ((url.protocol !== "http:" && url.protocol !== "https:") || url.username || url.password || url.search || url.hash) {
    throw new Error("supplier base URL must be an absolute HTTP(S) URL without credentials, query, or fragment");
  }
  if (!allowedHosts.has(url.hostname.toLowerCase())) {
    throw new Error("supplier base URL host is not allowlisted");
  }
  return url;
}

function parseHost(value: string): string {
  const host = value.trim();
  if (!host || (host !== "localhost" && isIP(host) === 0)) {
    throw new Error("listen host must be an explicit IP address or localhost");
  }
  return host;
}

export function readConfig(environment: NodeJS.ProcessEnv = process.env): Config {
  const rawAllowlist = required(environment, "MEMBERSHIP_BROWSER_ALLOWED_HOSTS");
  const allowedHosts = new Set(rawAllowlist.split(",").map((host) => host.trim().toLowerCase()).filter(Boolean));
  if (allowedHosts.size === 0) throw new Error("MEMBERSHIP_BROWSER_ALLOWED_HOSTS has no host");

  const port = Number.parseInt(required(environment, "MEMBERSHIP_BROWSER_LISTEN_PORT"), 10);
  if (!Number.isInteger(port) || port < 0 || port > 65535) throw new Error("invalid listen port");

  const bearerToken = required(environment, "MEMBERSHIP_BROWSER_BEARER_TOKEN");
  if (bearerToken.length < 32) throw new Error("bearer token must be at least 32 characters");

  return {
    listenHost: parseHost(required(environment, "MEMBERSHIP_BROWSER_LISTEN_HOST")),
    listenPort: port,
    bearerToken,
    allowedHosts,
    bases: {
      gpt: parseBase(required(environment, "MEMBERSHIP_BROWSER_GPT_BASE_URL"), allowedHosts),
      gptpro: parseBase(required(environment, "MEMBERSHIP_BROWSER_GPTPRO_BASE_URL"), allowedHosts),
    },
    productionSubmitEnabled: environment.MEMBERSHIP_BROWSER_PRODUCTION_SUBMIT_ENABLED === "true",
    confirmedSupplierHost: environment.MEMBERSHIP_BROWSER_CONFIRM_SUPPLIER_HOST?.trim().toLowerCase() ?? "",
  };
}

export function isLoopback(url: URL): boolean {
  return url.hostname === "localhost" || url.hostname === "127.0.0.1" || url.hostname === "::1";
}

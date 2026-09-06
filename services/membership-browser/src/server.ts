import { createServer, type IncomingMessage, type Server, type ServerResponse } from "node:http";
import { timingSafeEqual } from "node:crypto";

import type { Config } from "./config";
import type { BrowserInput } from "./contracts";
import { reviewRequired } from "./contracts";
import type { MembershipRunner } from "./runner";

const maxRequestBytes = 70 * 1024;

function writeJSON(response: ServerResponse, status: number, value: unknown): void {
  response.writeHead(status, { "content-type": "application/json; charset=utf-8", "cache-control": "no-store" });
  response.end(JSON.stringify(value));
}

function authorized(request: IncomingMessage, token: string): boolean {
  const header = request.headers.authorization;
  if (!header?.startsWith("Bearer ")) return false;
  const supplied = Buffer.from(header.slice(7));
  const expected = Buffer.from(token);
  return supplied.length === expected.length && timingSafeEqual(supplied, expected);
}

async function readJSON(request: IncomingMessage): Promise<unknown> {
  const chunks: Buffer[] = [];
  let total = 0;
  for await (const piece of request) {
    const chunk = Buffer.isBuffer(piece) ? piece : Buffer.from(piece);
    total += chunk.length;
    if (total > maxRequestBytes) throw new Error("request too large");
    chunks.push(chunk);
  }
  return JSON.parse(Buffer.concat(chunks).toString("utf8"));
}

export function createMembershipServer(config: Config, runner: MembershipRunner): Server {
  return createServer(async (request, response) => {
    if (request.method !== "POST" || request.url !== "/execute") {
      writeJSON(response, 404, { error_code: "NOT_FOUND" });
      return;
    }
    if (!authorized(request, config.bearerToken)) {
      writeJSON(response, 401, { error_code: "UNAUTHORIZED" });
      return;
    }
    if (!request.headers["content-type"]?.toLowerCase().startsWith("application/json")) {
      writeJSON(response, 415, { error_code: "UNSUPPORTED_MEDIA_TYPE" });
      return;
    }
    try {
      writeJSON(response, 200, await runner.execute((await readJSON(request)) as BrowserInput));
    } catch {
      writeJSON(response, 200, reviewRequired());
    }
  });
}

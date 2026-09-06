import { createServer, type Server } from "node:http";

export interface Fixture {
  baseURL: string;
  cookies: string[];
  externalRequests: number;
  requests: Array<{ body: string; path: string }>;
  close(): Promise<void>;
}

function page(channel: string, scenario: string, externalURL: string): string {
  if (scenario === "captcha") return `<main data-membership-channel="${channel}"><div data-membership-captcha="1"></div></main>`;
  if (scenario === "drift") return `<main data-membership-channel="other"></main>`;
  const state = scenario === "failure" ? "failed" : scenario === "query-success" ? "succeeded" : "valid";
  const error = scenario === "failure" ? "ACCOUNT_NOT_ELIGIBLE" : "";
  const entitlement = scenario === "query-success" ? "true" : "false";
  const redirect = scenario === "unknown-submit" ? "history.pushState({}, '', '/unexpected')" : "";
  const queryControls = channel === "gpt"
    ? `<section class="card-channel2-page"><button class="card-channel2-query-panel-primary" type="button">🔍 查询卡密状态</button></section>
      <script>
        document.querySelector('.card-channel2-query-panel-primary').onclick = () => {
          const dialog = document.createElement('div');
          dialog.setAttribute('role', 'dialog');
          dialog.innerHTML = '<textarea></textarea><button type="button">查询</button>';
          dialog.querySelector('button').onclick = () => {
            const body = JSON.stringify({ cdks: dialog.querySelector('textarea').value });
            ${scenario === "external-response"
              ? `fetch('${externalURL}/api/card-recharge/query', { method: 'POST', headers: { 'content-type': 'application/json' }, body });`
              : `fetch('/api/card-recharge/query', { method: 'POST', headers: { 'content-type': 'application/json' }, body });`}
          };
          document.body.append(dialog);
        };
      </script>`
    : `<section class="gpt-manual-console"><input placeholder="请输入排单 CDK"><button type="button">查询</button></section>
      <script>
        document.querySelector('.gpt-manual-console button').onclick = () => {
          const body = JSON.stringify({ cdkey: document.querySelector('.gpt-manual-console input').value.trim().toUpperCase() });
          fetch('/api/gpt-source/manual/check', { method: 'POST', headers: { 'content-type': 'application/json' }, body });
        };
      </script>`;
  return `<!doctype html><main data-membership-channel="${channel}">
    <output data-testid="availability" data-available="4"></output>
    <input data-testid="credential"><input data-testid="account-id"><button data-testid="validate" type="button">validate</button>
    <input data-testid="cdk"><button data-testid="submit" type="button">submit</button>
    <output data-testid="result" data-state="${state}" data-error-code="${error}" data-upstream-task-id="fixture-task" data-account-id="acct_12345678" data-sku="fixture-sku" data-period-days="30" data-entitlement-verified="${entitlement}"></output>
    <script>document.querySelector('[data-testid=submit]').onclick=()=>{${redirect}}</script>
    ${queryControls}
  </main>`;
}

function scenarioFromCookie(cookie: string | undefined): string {
  return cookie?.match(/(?:^|;\s*)fixture-scenario=([^;]+)/)?.[1] ?? "";
}

function queryResponse(channel: string, scenario: string, body: string): unknown {
  const parsed = JSON.parse(body) as { cdk?: string; cdkey?: string; cdks?: string };
  const cdk = parsed.cdk ?? parsed.cdkey ?? parsed.cdks ?? "";
  const lifecycle = scenario === "query-pending" ? "pending"
    : scenario === "query-failed" ? "failed"
      : "completed";
  const entitlement = scenario === "query-success"
    ? { active: true, plan: "chatgpt_plus", expires_at: "2026-10-01T00:00:00.000Z", account_id: "acct_12345678" }
    : undefined;
  if (channel === "gpt") {
    return { success: true, data: { summary: {}, results: [{ cdk, status: lifecycle, info: { entitlement } }] } };
  }
  const status = scenario === "query-pending" ? -2 : scenario === "query-success" || scenario === "query-no-entitlement" ? 1 : lifecycle;
  return { success: true, data: { status, entitlement } };
}

function closeServer(server: Server): Promise<void> {
  server.closeAllConnections();
  return new Promise<void>((resolve, reject) => {
    server.close((error?: Error) => error ? reject(error) : resolve());
  });
}

export async function startFixture(scenario: string): Promise<Fixture> {
  const cookies: string[] = [];
  const requests: Array<{ body: string; path: string }> = [];
  let externalRequests = 0;
  const external: Server = createServer((request, response) => {
    const corsHeaders = {
      "access-control-allow-headers": "content-type",
      "access-control-allow-methods": "POST",
      "access-control-allow-origin": "*",
      connection: "close",
    };
    if (request.method === "OPTIONS") {
      response.writeHead(204, corsHeaders);
      response.end();
      return;
    }
    externalRequests += 1;
    response.writeHead(200, { ...corsHeaders, "content-type": "application/json" });
    response.end(JSON.stringify({ success: true, data: { status: "completed" } }));
  });
  await new Promise<void>((resolve) => external.listen(0, "127.0.0.1", resolve));
  external.unref();
  const externalAddress = external.address();
  if (!externalAddress || typeof externalAddress === "string") throw new Error("external fixture did not bind TCP");
  const externalURL = `http://127.0.0.1:${externalAddress.port}`;
  const server: Server = createServer(async (request, response) => {
    cookies.push(request.headers.cookie ?? "");
    if (request.method === "POST" && (request.url === "/api/card-recharge/query" || request.url === "/api/gpt-source/manual/check")) {
      const chunks: Buffer[] = [];
      for await (const chunk of request) chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk));
      const body = Buffer.concat(chunks).toString("utf8");
      requests.push({ body, path: request.url });
      const channel = request.url.includes("card-recharge") ? "gpt" : "gptpro";
      response.writeHead(200, { connection: "close", "content-type": "application/json" });
      response.end(JSON.stringify(queryResponse(channel, scenarioFromCookie(request.headers.cookie), body)));
      return;
    }
    const parts = request.url?.split("/").filter(Boolean) ?? [];
    const scenarioPath = parts.length >= 3 ? parts[1] : parts[0];
    const channel = parts.at(-1);
    response.writeHead(200, { connection: "close", "content-type": "text/html; charset=utf-8", "set-cookie": [`fixture=isolated; Path=/; HttpOnly`, `fixture-scenario=${scenarioPath ?? scenario}; Path=/; HttpOnly`] });
    response.end(page(channel ?? "gpt", scenarioPath ?? scenario, externalURL));
  });
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  server.unref();
  const address = server.address();
  if (!address || typeof address === "string") throw new Error("fixture did not bind TCP");
  return {
    baseURL: `http://127.0.0.1:${address.port}/${scenario}`,
    cookies,
    get externalRequests() { return externalRequests; },
    requests,
    close: async () => {
      await Promise.all([closeServer(server), closeServer(external)]);
    },
  };
}

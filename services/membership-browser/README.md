# Membership Browser Sidecar

This private service exposes only `POST /execute` and requires a Bearer token.
It uses a fresh, non-persistent Playwright browser context for each request.
It does not record video, HAR, traces, profile data, downloads, browser storage,
or request content. It has no credential-file integration and never logs request
bodies, credentials, CDKs, tokens, or supplier response text.

The accepted operations are `checkAvailability`, `validateCredential`,
`submitRecharge`, `queryStatus`, `cancel`, and `refresh`. `cancel` and `refresh`
return `NOT_SUPPORTED`. A CAPTCHA, login requirement, page-contract change, bad
result, redirect outside the supplier allowlist, or browser failure returns a
safe result instead of exposing page content.

`submitRecharge` is disabled for every non-loopback supplier unless both
`MEMBERSHIP_BROWSER_PRODUCTION_SUBMIT_ENABLED=true` and
`MEMBERSHIP_BROWSER_CONFIRM_SUPPLIER_HOST` exactly matches the configured
supplier hostname. The browser still refuses a real submission unless the
request contains an explicit CDK and credential, because each request has a
fresh context. Tests exercise submission only against the local fixture.

## Configuration

```text
MEMBERSHIP_BROWSER_LISTEN_HOST=127.0.0.1
MEMBERSHIP_BROWSER_LISTEN_PORT=18081
MEMBERSHIP_BROWSER_BEARER_TOKEN=<at-least-32-random-characters>
MEMBERSHIP_BROWSER_ALLOWED_HOSTS=membership.example.invalid
MEMBERSHIP_BROWSER_GPT_BASE_URL=https://membership.example.invalid/gpt
MEMBERSHIP_BROWSER_GPTPRO_BASE_URL=https://membership.example.invalid/gptpro
MEMBERSHIP_BROWSER_PRODUCTION_SUBMIT_ENABLED=false
MEMBERSHIP_BROWSER_CONFIRM_SUPPLIER_HOST=
```

Both base URLs are startup configuration, must use an allowlisted hostname, and
cannot contain credentials, a query string, or a fragment. Request data cannot
select a URL or a domain. Bind the service to a private address and provide its
token through the process supervisor's secret mechanism rather than command
arguments or this repository.

## Verification

```bash
cd services/membership-browser
npm install
npm run typecheck
npm test
```

The test suite starts a local fixture supplier and covers successful availability
and credential validation, a supplier failure, CAPTCHA, page drift, an unknown
post-click result, unsupported operations, Bearer authentication, and isolated
browser contexts. `queryStatus` exercises the real GPT query-card dialog and
the GPT Pro CDK query button, then accepts only the corresponding same-origin
response. Pending and failed responses are handled explicitly; a completed
response without separate entitlement evidence, an unexpected response shape,
or a cross-origin response requires review. It never contacts a real supplier
or submits a recharge.

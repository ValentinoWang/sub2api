#!/usr/bin/env python3
"""Read-only cost research. Python 3.11+, standard library; no trading/reset calls."""
from __future__ import annotations
import argparse
import datetime as dt
from decimal import Decimal
import hashlib
from html.parser import HTMLParser
import json
import math
import os
from pathlib import Path
import random
import re
import subprocess
from urllib import request, parse, robotparser

UTC = dt.timezone.utc
MAX_BODY = 4_000_000
UA = 'Sub2API-CostResearch/0.1'
RADAR = 'https://codexradar.com/en/'


def now() -> str:
    return dt.datetime.now(UTC).isoformat()


class NoRedirect(request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        raise ValueError('Redirect refused; inspect the destination manually')


def get(url: str, token: str | None = None) -> bytes:
    p = parse.urlsplit(url)
    if p.scheme != 'https' or not p.hostname or p.username or p.password or p.fragment:
        raise ValueError('An explicit HTTPS URL without embedded credentials is required')
    headers = {'User-Agent': UA, 'Accept': 'application/json,text/html'}
    if token:
        headers['Authorization'] = 'Bearer ' + token
    with request.build_opener(NoRedirect()).open(request.Request(url, headers=headers), timeout=20) as res:
        raw = res.read(MAX_BODY + 1)
    if len(raw) > MAX_BODY:
        raise ValueError('Response too large; no partial parse is allowed')
    return raw


def check_robots(url: str) -> None:
    p = parse.urlsplit(url)
    robots_url = f'{p.scheme}://{p.netloc}/robots.txt'
    # Fail closed, including an unavailable robots policy. A site-provided export is an alternative.
    policy = robotparser.RobotFileParser()
    policy_text = get(robots_url).decode('utf-8', errors='replace')
    if re.search(r'<(?:!doctype\s+html|html)\b', policy_text[:2048], re.I):
        raise ValueError('robots.txt returned HTML, not a valid crawler policy')
    policy.parse(policy_text.splitlines())
    if not policy.can_fetch(UA, url):
        raise ValueError('Public page collection is not allowed by robots policy')


class VisibleText(HTMLParser):
    def __init__(self):
        super().__init__()
        self.skip = 0
        self.parts: list[str] = []

    def handle_starttag(self, tag, attrs):
        if tag in ('script', 'style'):
            self.skip += 1

    def handle_endtag(self, tag):
        if tag in ('script', 'style'):
            self.skip = max(0, self.skip - 1)

    def handle_data(self, data):
        if not self.skip:
            self.parts.append(data)


def radar_parse(html: str) -> dict:
    parser = VisibleText()
    parser.feed(html)
    text = re.sub(r'\s+', ' ', ' '.join(parser.parts))
    start = re.search(r'Quota Radar|额度雷达', text)
    section = text[start.start():] if start else ''
    section = re.split(r'Fast Radar|Fast 雷达', section)[0]
    rows = []
    for tier, amount, basis in re.findall(
            r'(20x Pro|5x Pro|Plus)\s*\$([\d,]+\.\d{2})\s*(Distributed Radar|estimated|分布式雷达|推算)', section):
        rows.append({'tier': tier, 'weekly_reference_usd': float(amount.replace(',', '')),
                     'basis': basis, 'model': None, 'price_version': None})
    stamp = re.search(r'(\d{1,2}月\d{1,2}日\s*\d{1,2}:\d{2}[^ ]*|Updated [A-Za-z]+ \d+, \d+:\d+)', section)
    count = re.search(r'(?:Of\s+|可核验的\s*)(\d+)\s*(?:verifiable resets|次历史重置)', text)
    return {'schema_version': 1, 'collected_at': now(), 'source': RADAR,
            'source_updated_label': stamp.group(0) if stamp else None,
            'weekly_quota': rows, 'seven_day_average': None,
            'two_calendar_month_reset_mean': None, 'history_complete': False,
            'all_history_reset_count': int(count.group(1)) if count else None,
            'requested_models': {'gpt-6-astra': None, 'gpt-5.6-sol': None},
            'status': 'REVIEW_REQUIRED' if rows else 'DYNAMIC_OR_SCHEMA_CHANGED',
            'warnings': ['7d quota is not a 7-day moving average',
                         'Tier estimates are not account observations',
                         'All-history counts are not two-month counts',
                         'Missing model/price version prevents model-specific costing']}


def private_projection(payload: dict, kind: str) -> dict:
    data = payload.get('data', payload)
    if not isinstance(data, dict):
        raise ValueError('Unexpected admin response; retain no private raw body')
    if kind == 'quota':
        allowed = ('plan_type', 'rate_limit', 'additional_rate_limits',
                   'rate_limit_reset_credits', 'fetched_at')
    else:
        allowed = ('supported', 'platform_type', 'rates', 'warnings', 'upstream_group')
    # Never keep the original response, token summaries, emails, credentials or raw fields.
    def clean(value):
        if isinstance(value, dict):
            return {k: clean(v) for k, v in value.items()
                    if not any(x in k.lower() for x in ('secret', 'token', 'email', 'credential', 'raw'))
                    or k.endswith('_per_token')}
        if isinstance(value, list):
            return [clean(v) for v in value]
        return value
    return {k: clean(data[k]) for k in allowed if k in data}


def vnstat_parse(data: dict, iface: str) -> dict:
    if str(data.get('jsonversion')) != '2':
        raise ValueError('Only vnStat JSON v2 byte units are supported')
    matches = [x for x in data.get('interfaces', []) if x.get('name') == iface]
    if len(matches) != 1:
        raise ValueError('Choose one explicit billing interface')
    item = matches[0]
    return {'collected_at': now(), 'interface': iface, 'unit': 'byte',
            'db_created': item.get('created'), 'db_updated': item.get('updated'),
            'traffic': item.get('traffic'), 'invoice_cost': None,
            'note': 'Daily/monthly/total are alternative aggregations, never sum them'}


def traffic_cost(rx: int, tx: int, included_bytes: int, mode: str,
                 currency_per_gb: str, gb_bytes: int = 1_000_000_000) -> Decimal:
    if min(rx, tx, included_bytes) < 0 or gb_bytes <= 0:
        raise ValueError('Invalid byte counters or GB definition')
    if mode not in ('egress', 'both'):
        raise ValueError('Only volume tariffs; bandwidth/95th-percentile need separate billing')
    rate = Decimal(currency_per_gb)
    if not rate.is_finite() or rate < 0:
        raise ValueError('Invalid tariff')
    used = tx if mode == 'egress' else rx + tx
    return Decimal(max(0, used - included_bytes)) / Decimal(gb_bytes) * rate


def reset_gain(before: float, after: float, consumed_between: float = 0) -> float:
    if not all(math.isfinite(x) for x in (before, after, consumed_between)):
        raise ValueError('Non-finite quota')
    if not 0 <= before <= 1 or not 0 <= after <= 1 or consumed_between < 0:
        raise ValueError('Quota fractions, not percentages, are required')
    # Same account, pool, capacity version; natural rollover must be classified separately.
    return max(0., after - before + consumed_between)


def seven_day_mean(rows: list[dict], end_date: str) -> dict:
    end = dt.date.fromisoformat(end_date)  # end-exclusive, date boundaries in one declared timezone
    expected = {(end - dt.timedelta(days=i)).isoformat() for i in range(1, 8)}
    selected = [r for r in rows if r.get('date') in expected]
    if len(selected) != 7 or {r['date'] for r in selected} != expected:
        return {'value': None, 'status': 'INCOMPLETE_OR_DUPLICATE'}
    if any(r.get('value') is None or any(not r.get(k) for k in ('pool', 'model', 'price_version')) for r in selected):
        return {'value': None, 'status': 'MISSING'}
    if len({(r.get('pool'), r.get('model'), r.get('price_version')) for r in selected}) != 1:
        return {'value': None, 'status': 'INCOMPARABLE'}
    values = [float(r['value']) for r in selected]
    if any(not math.isfinite(x) or x < 0 for x in values):
        raise ValueError('Invalid daily estimate')
    return {'value': sum(values) / 7, 'status': 'COMPLETE_7_DAILY_ESTIMATES'}


def two_month_rate(events: list[dict], as_of: str, coverage_complete: bool) -> dict:
    end = dt.date.fromisoformat(as_of)
    month_index = end.year * 12 + end.month - 1 - 2
    year, month = divmod(month_index, 12)
    import calendar
    start = dt.date(year, month + 1, min(end.day, calendar.monthrange(year, month + 1)[1]))
    result = {'start_inclusive': str(start), 'end_exclusive': str(end),
              'coverage_days': (end - start).days, 'events': None, 'per_week': None,
              'status': 'INCOMPLETE'}
    if not coverage_complete:
        return result
    ids = set()
    for e in events:
        if e.get('kind') != 'direct_reset_applied' or not e.get('verified'):
            continue
        day = dt.date.fromisoformat(e['date'])
        if start <= day < end:
            ids.add(e['event_id'])
    result.update(events=len(ids), per_week=7 * len(ids) / (end - start).days, status='COMPLETE')
    return result


def forecast(config: dict) -> dict:
    """One stationary event stream only; scenario posterior predictive, not an invoice."""
    def positive(key):
        val = float(config[key])
        if not math.isfinite(val) or val <= 0:
            raise ValueError('Positive finite ' + key + ' required')
        return val
    if config.get('own_exposure_complete') is not True:
        raise ValueError('Complete own event exposure must be explicitly confirmed')
    base = positive('weekly_reference_capacity')
    days = positive('billing_days')
    cost = positive('cash_and_allocated_cost')
    a, b = positive('prior_event_shape'), positive('prior_exposure_weeks')
    weeks = positive('own_observed_weeks')
    n = config['own_event_count']
    if not isinstance(n, int) or n < 0:
        raise ValueError('Event count must be a nonnegative integer')
    deltas = config['verified_net_refill_fractions']
    if not deltas or len(deltas) != n or any(not 0 <= d <= 1 for d in deltas):
        raise ValueError('Need measured net gains (including confirmed zero gains), not announcements')
    use = float(config['useful_utilization'])
    if not math.isfinite(use) or not 0 < use <= 1:
        raise ValueError('Utilization must be in (0,1]')
    rng = random.Random(config.get('seed', 7))
    samples = []
    for _ in range(4000):
        rate = rng.gammavariate(a + n, 1 / (b + weeks))
        # Exponential inter-arrival simulation avoids numerical underflow at large Poisson means.
        clock, gains = 0., 0.
        weights = [rng.expovariate(1) for _ in deltas]  # Bayesian bootstrap for bounded severity
        while rate > 0:
            clock += rng.expovariate(rate)
            if clock >= days / 7:
                break
            gains += rng.choices(deltas, weights=weights, k=1)[0]
        delivered = base * (days / 7 + gains) * use
        samples.append(cost / delivered)
    samples.sort()
    return {'unit': 'cash currency / frozen-reference API-value unit',
            'cost_p10': samples[399], 'cost_p50': samples[1999], 'cost_p90': samples[3599],
            'event_rate_posterior_mean_per_week': (a+n)/(b+weeks),
            'status': 'CONDITIONAL_SCENARIO', 'seed': config.get('seed', 7),
            'assumptions': ['Fixed calibrated base capacity and fixed utilization',
                            'One stationary event type; do not mix grants, redemptions and direct resets',
                            'Billing-day/7 prorating is a forecast, not an exact cycle replay',
                            'No within-window concurrency scheduler; no cold-start point estimate']}


def save(path: str, value) -> None:
    target = Path(path)
    target.parent.mkdir(parents=True, exist_ok=True)
    # Owner-only output; callers choose a private directory OUTSIDE the checkout.
    fd = os.open(target, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    with os.fdopen(fd, 'w') as stream:
        json.dump(value, stream, ensure_ascii=False, indent=2, allow_nan=False)
        stream.write('\n')
    os.chmod(target, 0o600)


def main() -> None:
    p = argparse.ArgumentParser(description=__doc__)
    sub = p.add_subparsers(dest='command', required=True)
    r = sub.add_parser('radar'); r.add_argument('--html'); r.add_argument('--output', required=True)
    m = sub.add_parser('market'); m.add_argument('--model', required=True); m.add_argument('--output', required=True)
    a = sub.add_parser('account'); a.add_argument('--origin', required=True); a.add_argument('--account-id', type=int, required=True)
    a.add_argument('--kind', choices=['quota', 'rates'], required=True); a.add_argument('--authorized', action='store_true'); a.add_argument('--output', required=True)
    t = sub.add_parser('traffic'); t.add_argument('--interface', required=True); t.add_argument('--vnstat-json'); t.add_argument('--output', required=True)
    f = sub.add_parser('forecast'); f.add_argument('--config', required=True); f.add_argument('--output', required=True)
    args = p.parse_args()
    if args.command == 'radar':
        if args.html:
            raw = Path(args.html).read_bytes()
        else:
            check_robots(RADAR); raw = get(RADAR)
        result = radar_parse(raw.decode('utf-8'))
        result['transport'] = 'provided_html' if args.html else 'https_get'
        result['raw_sha256'] = hashlib.sha256(raw).hexdigest()
    elif args.command == 'market':
        if not re.fullmatch(r'[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.:-]+', args.model):
            raise ValueError('Expected provider/model slug')
        url = 'https://openrouter.ai/api/v1/models/' + args.model + '/endpoints'
        raw = get(url, os.getenv('OPENROUTER_API_KEY'))
        payload = json.loads(raw)
        result = {'collected_at': now(), 'source': url, 'published_not_local_probe': True,
                  'data': payload.get('data'), 'raw_sha256': hashlib.sha256(raw).hexdigest()}
    elif args.command == 'account':
        origin = parse.urlsplit(args.origin)
        if not args.authorized or args.account_id <= 0 or origin.path not in ('', '/') or origin.query:
            raise ValueError('Explicit authorization, positive account ID and bare origin required')
        token = os.environ['SUB2API_ADMIN_TOKEN']
        suffix = (f'/openai/{args.account_id}/quota' if args.kind == 'quota'
                  else f'/accounts/{args.account_id}/upstream-billing/rates')
        data = json.loads(get(args.origin.rstrip('/') + '/api/v1/admin' + suffix, token))
        result = {'collected_at': now(), 'local_account_id': args.account_id,
                  'kind': args.kind, 'data': private_projection(data, args.kind)}
    elif args.command == 'traffic':
        if not re.fullmatch(r'[\w.:-]+', args.interface):
            raise ValueError('Invalid interface name')
        raw = (Path(args.vnstat_json).read_text() if args.vnstat_json else
               subprocess.run(['vnstat', '--json', '-i', args.interface], check=True,
                              capture_output=True, text=True, timeout=15).stdout)
        result = vnstat_parse(json.loads(raw), args.interface)
    else:
        result = forecast(json.loads(Path(args.config).read_text()))
    save(args.output, result)


if __name__ == '__main__':
    try:
        main()
    except (ValueError, KeyError, OSError, subprocess.SubprocessError) as exc:
        # No upstream response or credentials in diagnostics.
        raise SystemExit(f'Collection/model stopped ({type(exc).__name__}). Check inputs, permissions and connectivity.') from None

export default {
  codexHarvest: {
    title: 'Codex tickets',
    description: 'Harvest 292 tickets for ChatGPT OAuth accounts in the background: choose the harvest proxy pool, set speed and limits, and review tickets, per-proxy results and the harvest log.',
    refresh: 'Refresh',
    loadError: 'Failed to load ticket harvesting status',
    status: {
      enabled: 'Harvesting on',
      disabled: 'Harvesting off',
      failClosed: 'Gated models blocked without a ticket',
      failOpen: 'Requests forwarded without a ticket',
      running: 'Round in progress',
      nextRound: 'Next round {time}',
      lastRound: 'Last round {time}',
      requests: 'Round requests {used}/{budget}',
      currentProxy: 'Using {name}',
      openSettings: 'Toggle in system settings'
    },
    idle: {
      disabled: 'Harvesting is off; no requests are sent.',
      empty_pool: 'The harvest proxy pool is empty; no requests are sent. Select proxies below and save.',
      pool_unavailable: 'Could not read the proxy pool; this round was skipped.',
      all_ready: 'Every account has a usable ticket; nothing to refresh this round.',
      round_budget: 'This round used its request budget; remaining accounts wait for the next round.'
    },
    settingsError: 'Harvest settings could not be read; the last valid settings stay in effect.',
    pool: {
      title: 'Harvest proxy pool',
      description: 'Harvesting only uses the proxies selected here. Production traffic still uses each account\'s own proxy. Add proxies or import a subscription in Proxy management first.',
      manageLink: 'Open proxy management',
      empty: 'There are no active proxies in proxy management yet.',
      selected: '{count} selected',
      columns: {
        select: 'Use',
        name: 'Proxy',
        address: 'Address',
        exit: 'Exit',
        latency: 'Latency',
        actions: 'Actions'
      },
      groups: 'Add by group',
      groupsHint: 'Click a group to add or remove its active proxies. 🎫 打票出口 holds residential nodes from the subscription; 💼 业务出口 holds datacenter nodes.',
      residential: 'Residential',
      datacenter: 'Datacenter',
      checkExit: 'Check exit',
      checking: 'Checking…',
      checkFailed: 'Exit check failed',
      unknownExit: 'Not checked',
      unusable: 'Unusable in pool',
      reasons: {
        deleted: 'Deleted',
        inactive: 'Inactive',
        expired: 'Expired'
      }
    },
    speed: {
      title: 'Speed and limits',
      description: 'Tickets refresh at 150s and stop being injected at 210s; every preset keeps a round inside that 60s window. Switching proxies never restores hourly quota.',
      presets: {
        slow: 'Slow',
        standard: 'Standard',
        fast: 'Fast',
        burst: 'Burst',
        custom: 'Custom'
      },
      fields: {
        round_interval_seconds: 'Round interval (s)',
        probe_interval_seconds: 'Request gap (s)',
        attempt_timeout_seconds: 'Attempt timeout (s)',
        cooldown_seconds: 'Failure cooldown (s)',
        max_requests_per_round: 'Requests per round',
        max_proxy_attempts: 'Proxies per account',
        max_requests_per_account_hour: 'Requests per account per hour'
      },
      hints: {
        round_interval_seconds: 'How often accounts needing a ticket are checked.',
        probe_interval_seconds: 'Minimum gap between any two harvest requests.',
        attempt_timeout_seconds: 'Longest wait for a single harvest request.',
        cooldown_seconds: 'Wait after a failed account/model before retrying; a longer upstream Retry-After wins.',
        max_requests_per_round: 'Total requests across all accounts in one round.',
        max_proxy_attempts: 'How many proxies one account/model may try in a round.',
        max_requests_per_account_hour: 'Harvest requests one account may send per hour; the count restarts with the service.'
      },
      range: 'Range {min}–{max}',
      save: 'Save pool and speed',
      saving: 'Saving…',
      saved: 'Saved; harvesting uses the new settings now',
      saveError: 'Failed to save'
    },
    accounts: {
      title: 'Account tickets',
      empty: 'No ChatGPT OAuth accounts can hold tickets.',
      columns: {
        account: 'Account',
        tickets: 'Tickets',
        hour: 'Requests this hour',
        manual: 'Manual harvest'
      },
      ready: 'Ready · {seconds}s left',
      waiting: 'Waiting for a ticket',
      blocked: 'Blocked without ticket',
      cooldown: 'Cooling until {time}',
      proxy: 'Issued via {name}',
      notSchedulable: 'Not schedulable',
      manualStart: 'Harvest now',
      manualRunning: 'Harvesting ({attempts} sent)',
      manualResult: 'Last manual run: {result} ({attempts} sent)'
    },
    manual: {
      title: 'Manual harvest · {name}',
      description: 'Harvests for this account in the background and stops at the first valid ticket. Every request counts toward the hourly limit.',
      models: 'Models',
      maxAttempts: 'Attempts per model (1–20)',
      interval: 'Seconds between attempts (1–60)',
      start: 'Start',
      cancel: 'Cancel',
      started: 'Started; follow progress in the harvest log',
      error: 'Could not start manual harvest',
      results: {
        harvested: 'All harvested',
        partial: 'Partly harvested',
        exhausted: 'No ticket within the attempts',
        hour_budget: 'Hourly limit reached',
        account_error: 'Account rejected upstream',
        no_proxy: 'All proxies cooling down',
        cancelled: 'Cancelled'
      }
    },
    nodes: {
      title: 'Proxy results',
      description: 'Harvest outcomes per proxy, account and model. Proxies with recent tickets and higher success rates are tried first.',
      empty: 'No harvest records yet.',
      resetAll: 'Reset all',
      reset: 'Reset',
      resetConfirm: 'Clear all proxy results? Harvesting will explore proxies again.',
      resetDone: 'Reset',
      resetError: 'Failed to reset',
      columns: {
        proxy: 'Proxy',
        account: 'Account',
        model: 'Model',
        counts: 'OK / miss / network / account',
        streak: 'Failure streak',
        lastSuccess: 'Last success',
        cooldown: 'Cooling until',
        latency: 'Latency',
        lastResult: 'Last result',
        actions: 'Actions'
      }
    },
    events: {
      title: 'Harvest log',
      description: 'The latest 200 events, kept across restarts. Ticket contents and proxy passwords are never shown.',
      empty: 'No harvest events yet.',
      manual: 'Manual',
      columns: {
        time: 'Time',
        account: 'Account',
        model: 'Model',
        proxy: 'Proxy',
        result: 'Result',
        detail: 'Detail'
      }
    },
    results: {
      success: 'Valid ticket',
      stored: 'Stored',
      invalid_state: 'Invalid ticket',
      rate_limited: 'Rate limited',
      account_error: 'Account rejected',
      network_error: 'Network error',
      upstream_error: 'Upstream error',
      response_incomplete_or_error: 'Incomplete response',
      token_error: 'Token failed',
      hour_budget: 'Hourly limit reached',
      no_proxy: 'No usable proxy'
    },
    selection: {
      ticket_sticky: 'Reusing ticket proxy',
      recent_success: 'Recent success',
      explore: 'Exploring'
    }
  }
}

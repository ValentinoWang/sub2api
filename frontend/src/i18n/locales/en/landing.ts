export default {
  batchImageGuide: {
    title: 'Batch Image Generation',
    description: 'Submit multiple prompts in one job and download the generated images when complete'
  },
  // Home Page
  home: {
    viewOnGithub: 'View on GitHub',
    viewDocs: 'View Documentation',
    docs: 'Docs',
    switchToLight: 'Switch to Light Mode',
    switchToDark: 'Switch to Dark Mode',
    dashboard: 'Dashboard',
    login: 'Login',
    getStarted: 'Get Started',
    goToDashboard: 'Go to Dashboard',
    hero: {
      badge: 'Next-gen AI API Gateway',
      status: 'online'
    },
    flow: {
      client: 'You · rest',
      clientSub: 'sleep / slack off / chill',
      gateway: 'Unified Gateway',
      gatewaySub: 'auth · routing · billing',
      pool: 'Account Pool',
      upstream: 'AI · build',
      upstreamSub: 'code / run tasks / ship'
    },
    sell: {
      kicker: 'AI API Relay',
      latency: 'Site access latency',
      latencyDesc: 'Live browser-to-/health RTT; it is not upstream model time',
      stable: 'Never goes dark',
      stableDesc: 'Account pool auto-failover, online 24/7',
      relay: 'One base URL',
      relayDesc: 'Claude / GPT / Gemini behind a single endpoint',
      billing: 'Pay as you go',
      billingDesc: 'Usage-based billing with quotas'
    },
    station: {
      you: 'you · rest',
      core: '.lol relay',
      latencyLive: 'site RTT',
      probing: 'probing…',
      unavailable: 'n/a',
      stamp: 'NEVER GOES DARK'
    },
    address: {
      title: 'API Base URL',
      hint: 'Paste it as the Base URL in your client. Anthropic & OpenAI compatible.',
      copy: 'Copy',
      copied: 'Copied'
    },
    nightShift: {
      title: 'Night shift log',
      subtitle: 'You sleep. The agent keeps working through the relay.'
    },
    meme: {
      tagline: 'You rest. AI builds.',
      taglineSub: 'Sleep on it. The tech grows itself overnight.',
      youRest: 'you · rest',
      aiBuild: 'ai · build',
      nightShift: 'night shift log',
      ctaTitle: 'Ready to rest?',
      ctaDesc: 'Leave the building to AI.',
      footer: 'You rest. AI builds.'
    },
    terminal: {
      online: 'online',
      routing: 'Smart routing · picking best upstream',
      streaming: 'Streaming response',
      youSleep: 'going to sleep, you take it from here 💤',
      agentAck: 'ack. tonight: build · test · ship',
      building: 'building',
      done: '12 commits · 48 tests · PR ready',
      goodMorning: 'morning ☀️'
    },
    // User-focused value proposition
    heroSubtitle: 'One Key, All AI Models',
    heroDescription: 'No need to manage multiple subscriptions. Access Claude, GPT, Gemini and more with a single API key',
    tags: {
      subscriptionToApi: 'Subscription to API',
      stickySession: 'Session Persistence',
      realtimeBilling: 'Pay As You Go'
    },
    // Pain points section
    painPoints: {
      title: 'Sound Familiar?',
      items: {
        expensive: {
          title: 'High Subscription Costs',
          desc: 'Paying for multiple AI subscriptions that add up every month'
        },
        complex: {
          title: 'Account Chaos',
          desc: 'Managing scattered accounts and API keys across different platforms'
        },
        unstable: {
          title: 'Service Interruptions',
          desc: 'Single accounts hitting rate limits and disrupting your workflow'
        },
        noControl: {
          title: 'No Usage Control',
          desc: "Can't track where your money goes or limit team member usage"
        }
      }
    },
    // Solutions section
    solutions: {
      title: 'We Solve These Problems',
      subtitle: 'Three simple steps to stress-free AI access'
    },
    features: {
      unifiedGateway: 'One-Click Access',
      unifiedGatewayDesc: 'Get a single API key to call all connected AI models. No separate applications needed.',
      multiAccount: 'Always Reliable',
      multiAccountDesc: 'Smart routing across multiple upstream accounts with automatic failover. Say goodbye to errors.',
      balanceQuota: 'Pay What You Use',
      balanceQuotaDesc: 'Usage-based billing with quota limits. Full visibility into team consumption.'
    },
    // Comparison section
    comparison: {
      title: 'Why Choose Us?',
      headers: {
        feature: 'Comparison',
        official: 'Official Subscriptions',
        us: 'Our Platform'
      },
      items: {
        pricing: {
          feature: 'Pricing',
          official: 'Fixed monthly fee, pay even if unused',
          us: 'Pay only for what you use'
        },
        models: {
          feature: 'Model Selection',
          official: 'Single provider only',
          us: 'Switch between models freely'
        },
        management: {
          feature: 'Account Management',
          official: 'Manage each service separately',
          us: 'Unified key, one dashboard'
        },
        stability: {
          feature: 'Stability',
          official: 'Single account rate limits',
          us: 'Multi-account pool, auto-failover'
        },
        control: {
          feature: 'Usage Control',
          official: 'Not available',
          us: 'Quotas & detailed analytics'
        }
      }
    },
    providers: {
      title: 'Supported AI Models',
      description: 'One API, Multiple Choices',
      supported: 'Supported',
      soon: 'Soon',
      claude: 'Claude',
      gemini: 'Gemini',
      antigravity: 'Antigravity',
      more: 'More'
    },
    // CTA section
    cta: {
      title: 'Ready to Get Started?',
      description: 'Sign up now and get free trial credits to experience seamless AI access',
      button: 'Sign Up Free'
    },
    footer: {
      allRightsReserved: 'All rights reserved.'
    }
  },

  // Key Usage Query Page
  keyUsage: {
    title: 'API Key Usage',
    subtitle: 'Enter your API Key to view real-time spending and usage status',
    placeholder: 'sk-ant-mirror-xxxxxxxxxxxx',
    query: 'Query',
    querying: 'Querying...',
    privacyNote: 'Your Key is processed locally in the browser and will not be stored',
    dateRange: 'Date Range:',
    dateRangeToday: 'Today',
    dateRange7d: '7 Days',
    dateRange30d: '30 Days',
    dateRange90d: '90 Days',
    dateRangeCustom: 'Custom',
    apply: 'Apply',
    used: 'Used',
    detailInfo: 'Detail Information',
    tokenStats: 'Token Statistics',
    dailyDetail: 'Daily Detail',
    modelStats: 'Model Usage Statistics',
    // Table headers
    date: 'Date',
    model: 'Model',
    requests: 'Requests',
    inputTokens: 'Input Tokens',
    outputTokens: 'Output Tokens',
    cacheCreationTokens: 'Cache Creation',
    cacheReadTokens: 'Cache Read',
    cacheWriteTokens: 'Cache Write',
    totalTokens: 'Total Tokens',
    cost: 'Cost',
    // Status
    quotaMode: 'Key Quota Mode',
    walletBalance: 'Wallet Balance',
    // Ring card titles
    totalQuota: 'Total Quota',
    limit5h: '5-Hour Limit',
    limitDaily: 'Daily Limit',
    limit7d: '7-Day Limit',
    limitWeekly: 'Weekly Limit',
    limitMonthly: 'Monthly Limit',
    // Detail rows
    remainingQuota: 'Remaining Quota',
    expiresAt: 'Expires At',
    todayExpires: '(expires today)',
    daysLeft: '({days} days)',
    usedQuota: 'Used Quota',
    resetNow: 'Resetting soon',
    subscriptionType: 'Subscription Type',
    subscriptionExpires: 'Subscription Expires',
    // Usage stat cells
    todayRequests: 'Today Requests',
    todayInputTokens: 'Today Input',
    todayOutputTokens: 'Today Output',
    todayTokens: 'Today Tokens',
    todayCacheCreation: 'Today Cache Creation',
    todayCacheRead: 'Today Cache Read',
    todayCost: 'Today Cost',
    rpmTpm: 'RPM / TPM',
    totalRequests: 'Total Requests',
    totalInputTokens: 'Total Input',
    totalOutputTokens: 'Total Output',
    totalTokensLabel: 'Total Tokens',
    totalCacheCreation: 'Total Cache Creation',
    totalCacheRead: 'Total Cache Read',
    totalCost: 'Total Cost',
    avgDuration: 'Avg Duration',
    // Messages
    enterApiKey: 'Please enter an API Key',
    querySuccess: 'Query successful',
    queryFailed: 'Query failed',
    queryFailedRetry: 'Query failed, please try again later',
    noDailyUsage: 'No daily usage data',
  },

  // Setup Wizard
  setup: {
    title: 'rest2build Setup',
    description: 'Configure your rest2build instance',
    database: {
      title: 'Database Configuration',
      description: 'Connect to your PostgreSQL database',
      host: 'Host',
      port: 'Port',
      username: 'Username',
      password: 'Password',
      databaseName: 'Database Name',
      sslMode: 'SSL Mode',
      passwordPlaceholder: 'Password',
      ssl: {
        disable: 'Disable',
        require: 'Require',
        verifyCa: 'Verify CA',
        verifyFull: 'Verify Full'
      }
    },
    redis: {
      title: 'Redis Configuration',
      description: 'Connect to your Redis server',
      host: 'Host',
      port: 'Port',
      username: 'Username (optional)',
      password: 'Password (optional)',
      database: 'Database',
      usernamePlaceholder: 'Leave empty for default user',
      passwordPlaceholder: 'Password',
      enableTls: 'Enable TLS',
      enableTlsHint: 'Use TLS when connecting to Redis (public CA certs)'
    },
    admin: {
      title: 'Admin Account',
      description: 'Create your administrator account',
      email: 'Email',
      password: 'Password',
      confirmPassword: 'Confirm Password',
      passwordPlaceholder: 'Min 8 characters',
      confirmPasswordPlaceholder: 'Confirm password',
      passwordMismatch: 'Passwords do not match'
    },
    ready: {
      title: 'Ready to Install',
      description: 'Review your configuration and complete setup',
      database: 'Database',
      redis: 'Redis',
      adminEmail: 'Admin Email'
    },
    status: {
      testing: 'Testing...',
      success: 'Connection Successful',
      testConnection: 'Test Connection',
      installing: 'Installing...',
      completeInstallation: 'Complete Installation',
      completed: 'Installation completed!',
      redirecting: 'Redirecting to login page...',
      restarting: 'Service is restarting, please wait...',
      timeout: 'Service restart is taking longer than expected. Please refresh the page manually.'
    }
  },

  // Marketing / positioning (public pages, home sections, footer)
  marketing: {
    lab: 'Rest2Build AI Integration Lab',
    positioning: 'Multi-model API trial access & developer integration support',
    nonOfficialShort: 'Independent third party · not official · no affiliation',
    disclaimer:
      'Rest2Build is an independent third-party technical service. It is not part of, endorsed by, or affiliated with OpenAI, Anthropic, or any other model vendor. Model availability is subject to real-time status.',
    nav: {
      guides: 'Guides',
      codex: 'Codex CLI',
      claudeCode: 'Claude Code',
      openaiCompat: 'OpenAI-compatible API',
      publicBenefit: 'Free tier',
      business: 'Business',
      security: 'Security',
      benchmarks: 'Latency test',
      share: 'Share card',
      status: 'Service status',
      verify: 'Store verification',
      models: 'Models',
      keyUsage: 'Key usage',
      legal: 'Legal',
      publicInfo: 'Public info'
    },
    modes: {
      kicker: 'Two ways in',
      title: 'Managed gateway, or bring your own key',
      managed: {
        title: 'Managed Gateway',
        desc: 'Use a service key we issue: one endpoint, model switching, failover, quotas. Upstream credentials live only in our server-side key store and are never handed to users.',
        points: ['Per-user key, revocable anytime', 'OpenAI / Anthropic wire compatible', 'Usage and quota visible in real time']
      },
      byok: {
        title: 'BYOK Support',
        desc: 'Use your own official account or API key. We only help configure and debug Codex CLI, Claude Code, Cursor, Cline and friends. Credentials stay on your machine.',
        points: ['We never touch passwords', 'No cookies / OAuth tokens stored', 'No relaying through other people’s subscriptions']
      }
    },
    lines: {
      kicker: 'Two separate tracks',
      title: 'Free tier is free. Business is business.',
      benefit: {
        title: 'Free trial access',
        desc: 'A fixed allowance for first-time developers, students and open-source projects. No payment, no invoice, no review or referral required.',
        cta: 'Free tier rules'
      },
      business: {
        title: 'Business services',
        desc: 'Real pricing, contracts, usage statements, and invoices that match the work actually delivered.',
        cta: 'Business services'
      }
    },
    trust: {
      kicker: 'Security principles',
      title: 'We never touch your account',
      items: [
        'No resale of upstream API keys',
        'We never ask for passwords',
        'No cookies / OAuth tokens stored',
        'One key per user',
        'Revocable anytime',
        'Use BYOK for sensitive code'
      ]
    },
    faq: {
      kicker: 'FAQ',
      title: 'Frequently asked',
      items: [
        {
          q: 'Is this an official service?',
          a: 'No. Rest2Build is an independent third-party service with no authorization, partnership or endorsement from OpenAI, Anthropic or any other vendor.'
        },
        {
          q: 'What key do I get?',
          a: 'A service key issued by us that only works against this gateway. Upstream vendor keys are never shared, and we do not sell or share upstream accounts.'
        },
        {
          q: 'Can you top up GPT Pro / Claude Pro for me?',
          a: 'People search for this a lot. We do not sell accounts, log in for you or collect passwords. We only guide you through the official subscription page on your own account and help configure the client.'
        },
        {
          q: 'How do I get the free allowance? Do I need to leave a review?',
          a: 'First-time developers, students and open-source projects can apply. Allowance, models and validity are published before issue. No review, share or referral is required, and renewal is not guaranteed.'
        },
        {
          q: 'Can a company get an invoice?',
          a: 'Yes, for real, paid technical or software services only, matching the actual work and amount. Free-tier usage is never invoiced.'
        },
        {
          q: 'Do you keep my prompts?',
          a: 'The gateway records only the metadata needed for billing and debugging. See the security page for retention and revocation; use BYOK for sensitive code.'
        }
      ]
    },
    footer: {
      verifyHint: 'Only Xianyu store',
      builtOn: 'Built on the open-source Sub2API'
    },
    pages: {
      common: {
        backHome: 'Back to home',
        login: 'Sign in',
        getStarted: 'Get started',
        baseUrl: 'Base URL',
        copy: 'Copy',
        copied: 'Copied',
        updatedAt: 'Last updated',
        byokTitle: 'BYOK: your own official account',
        managedTitle: 'Managed: a key issued by this site',
        apiKeyPlaceholder: '<YOUR_SERVICE_KEY>'
      },
      publicBenefit: {
        title: 'Free tier rules',
        subtitle: 'For first-time developers, students and open-source projects. No payment, no invoice, no review required.',
        sections: [
          {
            h: 'Who can apply',
            items: ['Developers integrating for the first time', 'Students (school email or ID)', 'Maintainers of public open-source projects', 'Non-commercial indie projects']
          },
          {
            h: 'Allowance and validity',
            p: 'One fixed allowance per person. Amount, models and validity are published when issued. It does not auto-renew; you may re-apply subject to availability.'
          },
          {
            h: 'We do not require',
            items: ['Reviews, favourites or shares', 'Referrals', 'Any paid product', 'Any payment, and we never invoice free-tier usage']
          },
          {
            h: 'How to apply',
            p: 'Register, then tell us via the Xianyu store or the contact at the bottom of the page which tool you use, what for, and whether you have your own official account. Approved allowances land directly in your account.'
          },
          {
            h: 'Limits',
            p: 'The free tier does not promise permanence, unlimited use or availability of any specific model. Accounts breaching the terms lose the allowance.'
          }
        ]
      },
      business: {
        title: 'Business services & invoicing',
        subtitle: 'Real pricing, contracts, usage statements and invoices that match the work.',
        sections: [
          {
            h: 'What we deliver',
            items: ['Gateway onboarding and configuration', 'OpenAI-compatible migration testing', 'SDK compatibility, error, timeout and rate-limit diagnosis', 'Quota, key and team management', 'Incident and migration support']
          },
          {
            h: 'Deliverables',
            items: ['Scope confirmation', 'Model and data-handling notes', 'Periodic usage and cost statements', 'Service contract', 'Invoice matching the actual service']
          },
          {
            h: 'Invoicing principle',
            p: 'Only real, paid technical or software services are invoiced, with line items matching the contract. Free-tier usage is never invoiced; categories are confirmed with finance and tax.'
          },
          {
            h: 'Before you reach out',
            items: ['Expected volume and use case', 'Tooling (Codex / Claude Code / SDK)', 'Data sensitivity and whether BYOK is required', 'Invoicing entity details']
          }
        ]
      },
      security: {
        title: 'Security & data policy',
        subtitle: 'Public notes on keys, logs and retention.',
        sections: [
          {
            h: 'Keys',
            items: ['Every user gets an individual service key, revocable or rotatable from the console', 'Upstream vendor keys live only in the server-side key store and are never handed out', 'We never accept passwords, cookies or OAuth tokens']
          },
          {
            h: 'Logs and retention',
            items: ['The gateway records billing and debugging metadata: time, model, token counts, status, latency', 'Request bodies are not used for training; debug logs are purged on a fixed schedule', 'Usage is queryable and exportable per key, model and day']
          },
          {
            h: 'Recommendations',
            items: ['Use BYOK for sensitive code or data', 'Use separate keys per project with quota caps', 'Revoke and reissue immediately if a key leaks']
          },
          {
            h: 'Limits',
            p: 'This is an independent third-party service. Model availability, upstream policies and regional restrictions follow each vendor’s public terms and real-time status.'
          }
        ]
      },
      verify: {
        title: 'Xianyu store verification',
        subtitle: 'We operate exactly one Xianyu store. Any other store with a similar name is unrelated to us.',
        storeLabel: 'Only store name',
        platform: 'Xianyu',
        tips: [
          'The store listing names this domain and this page so both can be cross-checked',
          'We never sell accounts, log in for you, ask for passwords or resell upstream keys',
          'Free-tier access never requires reviews, shares or referrals',
          'Report anyone asking you to pay via WeChat or to hand over a password'
        ],
        contactLabel: 'Contact'
      },
      codex: {
        title: 'Codex CLI setup, configuration and troubleshooting',
        subtitle: 'Prefer your own official account; alternatively configure this gateway as a custom provider.',
        intro: 'Codex CLI supports custom model providers via config.toml. Pick one of the two modes below.',
        byok: 'Run codex and sign in with your own OpenAI account. We never log in for you, collect passwords, or relay through someone else’s subscription.',
        managedIntro: 'Configure this gateway as a custom provider and inject the key via an environment variable:',
        steps: [
          { h: '1. Write the config', p: 'Merge the block below into ~/.codex/config.toml.' },
          { h: '2. Inject the key', p: 'Export it in your shell; never commit it to a repository.' },
          { h: '3. Verify', p: 'Run codex, send a trivial prompt, and confirm it shows up on the usage page.' }
        ],
        troubleshooting: {
          h: 'Common issues',
          items: ['401: key invalid or revoked; reissue and retry', '404 model not found: check the models page for the exact name', '429: rate limit or quota hit; retry later or adjust quota', 'Timeouts: check the latency test page first']
        }
      },
      claudeCode: {
        title: 'Claude Code with your own account or API key',
        subtitle: 'Default to your own Claude subscription or Anthropic API key; gateway mode only when you explicitly choose it.',
        intro: 'Claude Code reads the API base URL and credential from environment variables.',
        byok: 'Run claude and sign in with your own account, or set your own ANTHROPIC_API_KEY. Credentials stay on your machine.',
        managedIntro: 'If you choose a key issued by this site, set:',
        steps: [
          { h: '1. Set the variables', p: 'Add them to your shell profile or export in the current session.' },
          { h: '2. Start Claude Code', p: 'Run claude and confirm the status line shows this site as the API base.' },
          { h: '3. Verify', p: 'Send a prompt and check the usage page.' }
        ],
        troubleshooting: {
          h: 'Common issues',
          items: ['Auth failed: ANTHROPIC_AUTH_TOKEN must be a key from this site, not an upstream key', 'Model unavailable: confirm the name on the models page', 'Back to official: unset the two variables']
        }
      },
      openaiCompat: {
        title: 'OpenAI-compatible API & migration testing',
        subtitle: 'Point base_url at this site; the rest of your code mostly stays the same.',
        intro: 'The gateway speaks both the OpenAI and Anthropic wire formats. Swap base_url and the key in your SDK.',
        steps: [
          { h: '1. Minimal request', p: 'Confirm the key and network with curl first.' },
          { h: '2. Swap base_url', p: 'OpenAI SDK targets /v1; the Anthropic SDK targets the root URL.' },
          { h: '3. Compare', p: 'Use the latency test page and the usage page to compare latency, error codes and token counts.' }
        ],
        troubleshooting: {
          h: 'Migration checklist',
          items: ['Exercise both streaming and non-streaming', 'Confirm tool / function calling fields pass through', 'Confirm timeout and retry policy', 'Record P50 / P95 latency as a baseline']
        }
      },
      benchmarks: {
        title: 'Latency self-test',
        subtitle: 'Fire requests from your browser at this site and compute success rate and P50 / P95 live.',
        intro: 'This is a visitor-side measurement, not a number we publish: it reflects only your current network and time of day.',
        run: 'Run test',
        running: 'Running…',
        again: 'Run again',
        target: 'Target',
        samples: 'Samples',
        success: 'Success',
        p50: 'P50',
        p95: 'P95',
        min: 'Min',
        max: 'Max',
        testedAt: 'Tested at',
        empty: 'Not run yet',
        serverSide: 'Server-side monitoring →',
        method: 'Method: 20 sequential GET /health requests (no-store); round-trip time of HTTP 200 responses.'
      }
    }
  },

  // Share card generator (/share)
  share: {
    title: 'Share card',
    subtitle: 'Generate a .lol-styled image with your live latency. Good for marketplace listings, group chats and social feeds.',
    size: 'Size',
    note: 'One line',
    notePlaceholder: 'e.g. the agent shipped 12 commits overnight',
    latency: 'Measured latency',
    remeasure: 'Re-measure',
    download: 'Download PNG',
    copyImage: 'Copy image',
    copied: 'Copied',
    scan: 'Scan to open',
    hint: 'Rendered entirely in your browser; nothing is uploaded. The latency figure is measured from your current network.'
  },
  // Affiliate promo assets (user affiliate page)
  affiliateAssets: {
    title: 'Promo assets',
    description: 'The invite QR and invite posters are for referrals; the separate marketplace cover contains no QR code.',
    qrTitle: 'Invite QR code',
    qrHint: 'Right-click or long-press to save',
    copyTitle: 'Copy-ready text',
    copy: 'Copy',
    copied: 'Copied',
    posterTitle: 'Poster',
    posterHint: 'Invite posters include a QR code; the marketplace cover does not.',
    posterDownload: 'Download poster',
    posterSquare: 'Square invite poster',
    posterWide: 'Wide invite poster (chats / feeds)',
    xianyuCover: 'Marketplace cover (no QR)',
    xianyuHeadline: 'AI API access service',
    xianyuSubline: 'Claude / GPT / Gemini from one endpoint',
    xianyuNote: 'Independent third-party service, usage-based access, revocable keys',
    xianyuMeta: 'Marketplace cover · no QR code',
    templates: [
      'You rest, AI builds. One base URL for Claude / GPT / Gemini, live-measured latency, per-user keys you can revoke anytime. Sign up with my link: {link}',
      'Using Codex CLI / Claude Code? {domain} is an independent multi-model relay with BYOK support that never touches your account. Invite link: {link}',
      'Students and open-source maintainers can apply for a free allowance, no reviews or referrals required. Rules are public on the site: {link}'
    ]
  },

  // Public status page (/status)
  statusPage: {
    title: 'Service status',
    subtitle: 'Read-only summary of scheduled server-side probes (channel monitor), auto-refreshing every 60 seconds.',
    loading: 'Loading…',
    error: 'The status endpoint is temporarily unavailable.',
    disabled: 'The public status page is not enabled on this site.',
    empty: 'No enabled monitors.',
    generatedAt: 'Generated',
    autoRefresh: 'refreshes every 60 s',
    since: 'since',
    columns: { service: 'Service', status: 'Status', success: 'Success', samples: 'Samples', lastChecked: 'Last probe' },
    overall: { operational: 'All operational', degraded: 'Partially degraded', failed: 'Incident', unknown: 'No data' },
    status: { operational: 'Operational', degraded: 'Degraded', failed: 'Failed', error: 'Error', unknown: 'Unknown' },
    method: 'Success rate and P50 / P95 come from scheduled server-side probes of each upstream; sample counts and window start are reported alongside. They do not represent your region or time of day; use the latency self-test for that.'
  },

  // Common
}

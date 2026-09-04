export default {
  batchImageGuide: {
    title: '图片批量生成',
    description: '一次提交多条提示词，任务完成后可统一下载图片结果'
  },
  // Home Page
  home: {
    viewOnGithub: '在 GitHub 上查看',
    viewDocs: '查看文档',
    docs: '文档',
    switchToLight: '切换到浅色模式',
    switchToDark: '切换到深色模式',
    dashboard: '控制台',
    login: '登录',
    getStarted: '立即开始',
    goToDashboard: '进入控制台',
    hero: {
      badge: '新一代 AI API 网关',
      status: 'online'
    },
    flow: {
      client: '你 · rest',
      clientSub: '睡觉 / 摸鱼 / 休息',
      gateway: '统一网关',
      gatewaySub: '鉴权 · 路由 · 计费',
      pool: '账号池',
      upstream: 'AI · build',
      upstreamSub: '写代码 / 跑任务 / 交付'
    },
    sell: {
      kicker: 'AI API 中转站',
      latency: '低延迟',
      latencyDesc: '就近接入，延迟实时可测',
      stable: '永不跑路',
      stableDesc: '账号池自动切换，7×24 在线',
      relay: '一个地址',
      relayDesc: 'Claude / GPT / Gemini 一站接入',
      billing: '按量计费',
      billingDesc: '用多少付多少，配额可控'
    },
    station: {
      you: '你 · rest',
      core: '.lol 中转站',
      latencyLive: '实时延迟',
      probing: '测量中…',
      unavailable: '暂不可测',
      stamp: '永不跑路'
    },
    address: {
      title: 'API 接入地址',
      hint: '填到客户端的 Base URL 即可，兼容 Anthropic / OpenAI 协议',
      copy: '复制',
      copied: '已复制'
    },
    nightShift: {
      title: '夜班日志',
      subtitle: '你睡了，agent 还在通过中转站干活'
    },
    meme: {
      tagline: '人去 rest，AI 去 build。',
      taglineSub: '睡一觉，科技自己长出来了。',
      youRest: 'you · rest',
      aiBuild: 'ai · build',
      nightShift: '夜班日志',
      ctaTitle: '准备好去 rest 了吗？',
      ctaDesc: '剩下的交给 AI 去 build。',
      footer: '人去 rest，AI 去 build。'
    },
    terminal: {
      online: 'online',
      routing: '智能路由 · 选择最优上游',
      streaming: '流式响应中',
      youSleep: '去睡了，剩下的交给你 💤',
      agentAck: '收到。今晚排期：build · test · ship',
      building: '构建中',
      done: '12 commits · 48 tests · PR ready',
      goodMorning: '早 ☀️'
    },
    // 新增：面向用户的价值主张
    heroSubtitle: '一个密钥，畅用多个 AI 模型',
    heroDescription: '无需管理多个订阅账号，一站式接入 Claude、GPT、Gemini 等主流 AI 服务',
    tags: {
      subscriptionToApi: '订阅转 API',
      stickySession: '会话保持',
      realtimeBilling: '按量计费'
    },
    // 用户痛点区块
    painPoints: {
      title: '你是否也遇到这些问题？',
      items: {
        expensive: {
          title: '订阅费用高',
          desc: '每个 AI 服务都要单独订阅，每月支出越来越多'
        },
        complex: {
          title: '多账号难管理',
          desc: '不同平台的账号、密钥分散各处，管理起来很麻烦'
        },
        unstable: {
          title: '服务不稳定',
          desc: '单一账号容易触发限制，影响正常使用'
        },
        noControl: {
          title: '用量无法控制',
          desc: '不知道钱花在哪了，也无法限制团队成员的使用'
        }
      }
    },
    // 解决方案区块
    solutions: {
      title: '我们帮你解决',
      subtitle: '简单三步，开始省心使用 AI'
    },
    features: {
      unifiedGateway: '一键接入',
      unifiedGatewayDesc: '获取一个 API 密钥，即可调用所有已接入的 AI 模型，无需分别申请。',
      multiAccount: '稳定可靠',
      multiAccountDesc: '智能调度多个上游账号，自动切换和负载均衡，告别频繁报错。',
      balanceQuota: '用多少付多少',
      balanceQuotaDesc: '按实际使用量计费，支持设置配额上限，团队用量一目了然。'
    },
    // 优势对比
    comparison: {
      title: '为什么选择我们？',
      headers: {
        feature: '对比项',
        official: '官方订阅',
        us: '本平台'
      },
      items: {
        pricing: {
          feature: '付费方式',
          official: '固定月费，用不完也付',
          us: '按量付费，用多少付多少'
        },
        models: {
          feature: '模型选择',
          official: '单一服务商',
          us: '多模型随意切换'
        },
        management: {
          feature: '账号管理',
          official: '每个服务单独管理',
          us: '统一密钥，一站管理'
        },
        stability: {
          feature: '服务稳定性',
          official: '单账号易触发限制',
          us: '多账号池，自动切换'
        },
        control: {
          feature: '用量控制',
          official: '无法限制',
          us: '可设配额、查明细'
        }
      }
    },
    providers: {
      title: '已支持的 AI 模型',
      description: '一个 API，多种选择',
      supported: '已支持',
      soon: '即将推出',
      claude: 'Claude',
      gemini: 'Gemini',
      antigravity: 'Antigravity',
      more: '更多'
    },
    // CTA 区块
    cta: {
      title: '准备好开始了吗？',
      description: '注册即可获得免费试用额度，体验一站式 AI 服务',
      button: '免费注册'
    },
    footer: {
      allRightsReserved: '保留所有权利。'
    }
  },

  // Key Usage Query Page
  keyUsage: {
    title: 'API Key 用量查询',
    subtitle: '输入您的 API Key 以查看实时消费金额与使用状态',
    placeholder: 'sk-ant-mirror-xxxxxxxxxxxx',
    query: '查询',
    querying: '查询中...',
    privacyNote: '您的 Key 仅在浏览器本地处理，不会被存储',
    dateRange: '统计范围:',
    dateRangeToday: '今日',
    dateRange7d: '7 天',
    dateRange30d: '30 天',
    dateRange90d: '90 天',
    dateRangeCustom: '自定义',
    apply: '应用',
    used: '已使用',
    detailInfo: '详细信息',
    tokenStats: 'Token 统计',
    dailyDetail: '按日明细',
    modelStats: '模型用量统计',
    // Table headers
    date: '日期',
    model: '模型',
    requests: '请求数',
    inputTokens: '输入 Tokens',
    outputTokens: '输出 Tokens',
    cacheCreationTokens: '缓存创建',
    cacheReadTokens: '缓存读取',
    cacheWriteTokens: '缓存写入',
    totalTokens: '总 Tokens',
    cost: '费用',
    // Status
    quotaMode: 'Key 限额模式',
    walletBalance: '钱包余额',
    // Ring card titles
    totalQuota: '总额度',
    limit5h: '5 小时限额',
    limitDaily: '日限额',
    limit7d: '7 天限额',
    limitWeekly: '周限额',
    limitMonthly: '月限额',
    // Detail rows
    remainingQuota: '剩余额度',
    expiresAt: '过期时间',
    todayExpires: '(今日到期)',
    daysLeft: '({days} 天)',
    usedQuota: '已用额度',
    resetNow: '即将重置',
    subscriptionType: '订阅类型',
    subscriptionExpires: '订阅到期',
    // Usage stat cells
    todayRequests: '今日请求',
    todayInputTokens: '今日输入',
    todayOutputTokens: '今日输出',
    todayTokens: '今日 Tokens',
    todayCacheCreation: '今日缓存创建',
    todayCacheRead: '今日缓存读取',
    todayCost: '今日费用',
    rpmTpm: 'RPM / TPM',
    totalRequests: '累计请求',
    totalInputTokens: '累计输入',
    totalOutputTokens: '累计输出',
    totalTokensLabel: '累计 Tokens',
    totalCacheCreation: '累计缓存创建',
    totalCacheRead: '累计缓存读取',
    totalCost: '累计费用',
    avgDuration: '平均耗时',
    // Messages
    enterApiKey: '请输入 API Key',
    querySuccess: '查询成功',
    queryFailed: '查询失败',
    queryFailedRetry: '查询失败，请稍后重试',
    noDailyUsage: '暂无按日用量数据',
  },

  // Setup Wizard
  setup: {
    title: 'rest2build 安装向导',
    description: '配置您的 rest2build 实例',
    database: {
      title: '数据库配置',
      description: '连接到您的 PostgreSQL 数据库',
      host: '主机',
      port: '端口',
      username: '用户名',
      password: '密码',
      databaseName: '数据库名称',
      sslMode: 'SSL 模式',
      passwordPlaceholder: '密码',
      ssl: {
        disable: '禁用',
        require: '要求',
        verifyCa: '验证 CA',
        verifyFull: '完全验证'
      }
    },
    redis: {
      title: 'Redis 配置',
      description: '连接到您的 Redis 服务器',
      host: '主机',
      port: '端口',
      username: '用户名（可选）',
      password: '密码（可选）',
      database: '数据库',
      usernamePlaceholder: '默认用户留空',
      passwordPlaceholder: '密码',
      enableTls: '启用 TLS',
      enableTlsHint: '连接 Redis 时使用 TLS（公共 CA 证书）'
    },
    admin: {
      title: '管理员账户',
      description: '创建您的管理员账户',
      email: '邮箱',
      password: '密码',
      confirmPassword: '确认密码',
      passwordPlaceholder: '至少 8 个字符',
      confirmPasswordPlaceholder: '确认密码',
      passwordMismatch: '密码不匹配'
    },
    ready: {
      title: '准备安装',
      description: '检查您的配置并完成安装',
      database: '数据库',
      redis: 'Redis',
      adminEmail: '管理员邮箱'
    },
    status: {
      testing: '测试中...',
      success: '连接成功',
      testConnection: '测试连接',
      installing: '安装中...',
      completeInstallation: '完成安装',
      completed: '安装完成！',
      redirecting: '正在跳转到登录页面...',
      restarting: '服务正在重启，请稍候...',
      timeout: '服务重启时间超出预期，请手动刷新页面。'
    }
  },

  // Marketing / positioning (public pages, home sections, footer)
  marketing: {
    lab: 'Rest2Build AI 接入实验室',
    positioning: '多模型 API 公益体验与开发接入支持',
    nonOfficialShort: '独立第三方 · 非官方 · 无授权关系',
    disclaimer:
      'Rest2Build 为独立第三方技术服务，不属于 OpenAI、Anthropic 或其他模型厂商，不代表相关厂商提供保证或背书。模型可用性以实时状态为准。',
    nav: {
      guides: '接入指南',
      codex: 'Codex CLI',
      claudeCode: 'Claude Code',
      openaiCompat: 'OpenAI 兼容接口',
      publicBenefit: '公益额度',
      business: '企业服务',
      security: '安全策略',
      benchmarks: '实测工具',
      share: '分享卡片',
      status: '服务状态',
      verify: '店铺验证',
      models: '模型列表',
      keyUsage: '用量查询',
      legal: '法律文件',
      publicInfo: '公开资料'
    },
    modes: {
      kicker: '两种接入方式',
      title: '托管网关，或者用你自己的 Key',
      managed: {
        title: 'Managed Gateway',
        desc: '使用我们签发的服务密钥：统一接口、模型切换、失败回退、用量额度。上游密钥只存在服务端密钥管理系统，不会发给任何用户。',
        points: ['独立密钥，可随时撤销', 'OpenAI / Anthropic 格式兼容', '用量与额度实时可查']
      },
      byok: {
        title: 'BYOK 接入支持',
        desc: '使用你自己的官方账号或 API Key。我们只提供 Codex CLI、Claude Code、Cursor、Cline 等工具的配置与排障，凭证保存在你本机，不经过我们的服务器。',
        points: ['不接触账号密码', '不保存 Cookie / OAuth Token', '不用他人订阅凭证替你转发']
      }
    },
    lines: {
      kicker: '两条线，分开走',
      title: '公益归公益，服务归服务',
      benefit: {
        title: '公益体验',
        desc: '面向首次接入的开发者、学生和开源项目的固定额度。不收钱、不开票、不要求好评、转发或拉新。',
        cta: '查看公益规则'
      },
      business: {
        title: '企业技术服务',
        desc: '真实收费、对公合同、用量明细，按实际发生的技术服务开具发票。',
        cta: '查看企业服务'
      }
    },
    trust: {
      kicker: '安全原则',
      title: '我们不碰你的账号',
      items: [
        '不出售上游 API Key',
        '不索取账号密码',
        '不保存 Cookie / OAuth Token',
        '每位用户独立密钥',
        '密钥可随时撤销',
        '敏感代码请使用 BYOK'
      ]
    },
    faq: {
      kicker: 'FAQ',
      title: '常见问题',
      items: [
        {
          q: '这是官方服务吗？',
          a: '不是。Rest2Build 是独立第三方技术服务，与 OpenAI、Anthropic 或其他模型厂商没有授权、合作或背书关系。'
        },
        {
          q: '我拿到的是什么密钥？',
          a: '是我们签发的服务密钥，只能访问本站网关。上游厂商的密钥永远不会发给用户，也不接受用户购买或共享上游账号。'
        },
        {
          q: '能帮我"GPT Pro 代充值 / Claude 代充"吗？',
          a: '很多用户会搜索这个词。本店不出售账号、不代登、不获取密码，只提供你自有账号的官方订阅开通和客户端配置指导，付款在官方页面由你自己完成。'
        },
        {
          q: '公益额度怎么领？要好评吗？',
          a: '首次接入的开发者、学生和开源项目可以申请固定额度。额度、可用模型和有效期在发放前公示。不要求好评、收藏、转发或拉新，也不承诺长期续期。'
        },
        {
          q: '企业可以开发票吗？',
          a: '可以，但只针对真实发生并收费的技术服务或软件服务，按实际业务内容和金额开具。公益额度不开票，也不开与实际业务不符的项目。'
        },
        {
          q: '你们会保存我的输入吗？',
          a: '网关只记录计费与排障所需的元数据。数据保留、日志和撤销策略见安全策略页；对敏感代码建议使用 BYOK 模式。'
        }
      ]
    },
    footer: {
      verifyHint: '唯一闲鱼店铺',
      builtOn: '基于开源项目 Sub2API 构建'
    },
    pages: {
      common: {
        backHome: '返回首页',
        login: '登录',
        getStarted: '立即开始',
        baseUrl: '接入地址',
        copy: '复制',
        copied: '已复制',
        updatedAt: '最近更新',
        byokTitle: 'BYOK：用你自己的官方账号',
        managedTitle: 'Managed：用本站签发的密钥',
        apiKeyPlaceholder: '<你的服务密钥>'
      },
      publicBenefit: {
        title: '公益额度规则',
        subtitle: '面向首次接入的开发者、学生与开源项目。不收钱、不开票、不要求好评。',
        sections: [
          {
            h: '谁可以申请',
            items: ['首次接入本站的开发者', '在读学生（提供学籍或学校邮箱）', '公开的开源项目维护者', '独立开发者的非商业项目']
          },
          {
            h: '额度与有效期',
            p: '每人一份固定额度，额度、可用模型和有效期以领取时公示为准。用完不自动续期；确有需要可再次申请，是否通过取决于当期名额。'
          },
          {
            h: '我们不要求',
            items: ['不要求好评、收藏、转发', '不要求拉新或推荐', '不绑定任何付费商品', '不收取任何费用，也不开具发票']
          },
          {
            h: '申请方式',
            p: '注册账号后，通过闲鱼店铺或页面底部的联系方式说明使用工具、用途和是否自有官方账号。审核通过后额度直接发到你的账户，可在控制台查看用量。'
          },
          {
            h: '边界',
            p: '公益额度不承诺永久、不限量或特定模型永远可用。模型可用性以实时状态为准；违反使用条款的账号会被撤销额度。'
          }
        ]
      },
      business: {
        title: '企业技术服务与发票说明',
        subtitle: '真实收费、对公合同、用量明细，按实际发生的业务开票。',
        sections: [
          {
            h: '服务内容',
            items: ['多模型 API 网关接入与配置', 'OpenAI 兼容接口迁移测试', 'SDK 兼容性、错误码、超时与限流诊断', '用量额度、密钥与团队成员管理', '故障排查与迁移支持']
          },
          {
            h: '交付物',
            items: ['技术服务范围确认', '模型与数据处理说明', '按周期出具的用量及费用明细', '服务合同（对公）', '按实际业务开具的发票']
          },
          {
            h: '发票原则',
            p: '只对真实发生并收费的技术服务或软件服务开票，发票项目与合同、交付内容一致。不为公益额度开票，不开与实际业务不符的类目，具体类目以财务与税务确认为准。'
          },
          {
            h: '咨询前请准备',
            items: ['预计调用量与使用场景', '使用工具（Codex / Claude Code / SDK 等）', '数据敏感程度与是否需要 BYOK', '开票主体信息']
          }
        ]
      },
      security: {
        title: '安全与数据策略',
        subtitle: '密钥、日志与数据保留的公开说明。',
        sections: [
          {
            h: '密钥',
            items: ['每位用户获得独立的服务密钥，可随时在控制台撤销或轮换', '上游厂商密钥只存在服务端密钥管理系统，不会以任何形式发给用户', '不接收用户的账号密码、Cookie 或 OAuth Token']
          },
          {
            h: '日志与保留',
            items: ['网关记录计费与排障所需的元数据：时间、模型、token 用量、状态码、耗时', '请求正文不用于训练，排障日志按固定周期清理', '用量明细可在控制台按密钥、模型、日期查询与导出']
          },
          {
            h: '建议',
            items: ['处理敏感代码或数据时使用 BYOK 模式，凭证只保存在你本机', '为不同项目使用不同密钥并设置额度上限', '密钥泄露时立即撤销并重新签发']
          },
          {
            h: '边界',
            p: '本站为独立第三方服务，模型可用性、上游政策与地区限制以各厂商公开信息与实时状态为准。'
          }
        ]
      },
      verify: {
        title: '闲鱼店铺验证',
        subtitle: '我们只有一家闲鱼店铺。任何其他同名或近似店铺均与本站无关。',
        storeLabel: '唯一店铺名称',
        platform: '闲鱼',
        tips: [
          '店铺详情中会写明本站域名与本页面地址，可互相对照',
          '本店不出售账号、不代登、不索取密码，也不出售上游 API Key',
          '公益额度不要求好评、转发或拉新',
          '收到"加微信付款""提供密码代登"等要求，请直接举报'
        ],
        contactLabel: '联系方式'
      },
      codex: {
        title: 'Codex CLI 接入、配置与常见错误排查',
        subtitle: '优先使用你自己的官方账号；也可以把本站网关配置为自定义 provider。',
        intro: 'Codex CLI 支持通过 config.toml 自定义 model provider。以下两种方式二选一。',
        byok: '直接运行 codex，按官方流程登录你自己的 OpenAI 账号。我们不代登、不接收密码，也不会用他人的订阅凭证替你转发请求。',
        managedIntro: '把本站网关配置为自定义 provider，密钥通过环境变量注入：',
        steps: [
          { h: '1. 写入配置', p: '把下面的内容合并到 ~/.codex/config.toml。' },
          { h: '2. 注入密钥', p: '在 shell 中导出环境变量，不要把密钥写进仓库。' },
          { h: '3. 验证', p: '运行 codex，发一条简单指令，控制台的用量页应出现这次请求。' }
        ],
        troubleshooting: {
          h: '常见问题',
          items: ['401：密钥无效或已撤销，重新签发后再试', '404 model not found：模型名不在可用列表，查看模型列表页', '429：触发限流或额度上限，稍后重试或调整配额', '超时：先在实测工具页确认网络延迟']
        }
      },
      claudeCode: {
        title: 'Claude Code 自有账号与 API Key 接入指南',
        subtitle: '默认使用你自己的 Claude 订阅或 Anthropic API Key；网关模式仅在你明确选择时使用。',
        intro: 'Claude Code 通过环境变量识别 API 地址与凭证。',
        byok: '运行 claude 并按官方流程登录你自己的账号，或设置你自己的 ANTHROPIC_API_KEY。凭证保存在你本机，不经过本站。',
        managedIntro: '如果你选择使用本站签发的密钥，设置以下环境变量：',
        steps: [
          { h: '1. 设置环境变量', p: '写入 shell 配置文件或在当前会话导出。' },
          { h: '2. 启动 Claude Code', p: '运行 claude，确认状态栏显示的 API 地址是本站。' },
          { h: '3. 验证', p: '发一条指令，控制台用量页应出现对应记录。' }
        ],
        troubleshooting: {
          h: '常见问题',
          items: ['认证失败：检查 ANTHROPIC_AUTH_TOKEN 是否为本站密钥而非上游 Key', '模型不可用：查看模型列表页确认名称', '想切回官方：清除上述两个环境变量即可']
        }
      },
      openaiCompat: {
        title: 'OpenAI 兼容接口与迁移测试',
        subtitle: '把 base_url 指向本站，其余代码基本不用改。',
        intro: '网关同时提供 OpenAI 与 Anthropic 两种协议格式，SDK 只需替换 base_url 和密钥。',
        steps: [
          { h: '1. 最小请求', p: '先用 curl 确认密钥与网络。' },
          { h: '2. 替换 SDK 的 base_url', p: 'OpenAI SDK 指向 /v1，Anthropic SDK 指向根地址。' },
          { h: '3. 对比', p: '用实测工具页与控制台用量页核对延迟、错误码与 token 计数。' }
        ],
        troubleshooting: {
          h: '迁移检查清单',
          items: ['流式（stream）与非流式都跑一遍', '确认 tool calling / function calling 字段透传', '确认超时与重试策略', '记录 P50 / P95 延迟作为基线']
        }
      },
      benchmarks: {
        title: '延迟实测工具',
        subtitle: '从你的浏览器向本站发起请求，实时计算成功率与 P50 / P95 延迟。',
        intro: '这是访客侧的实测，不是我们单方面公布的数据：结果只代表你当前的网络与时段，不代表其他地区或时间的表现。',
        run: '开始实测',
        running: '测试中…',
        again: '再测一次',
        target: '目标',
        samples: '样本数',
        success: '成功率',
        p50: 'P50',
        p95: 'P95',
        min: '最小',
        max: '最大',
        testedAt: '测试时间',
        empty: '尚未运行',
        serverSide: '服务端定时监控 →',
        method: '方法：串行发起 20 次 GET /health（no-store），统计 HTTP 200 的往返耗时。'
      }
    }
  },

  // Share card generator (/share)
  share: {
    title: '分享卡片',
    subtitle: '一键生成 .lol 风格图片，带上你此刻的实测延迟。适合闲鱼详情、群聊和朋友圈。',
    size: '尺寸',
    note: '一句话',
    notePlaceholder: '例如：今晚 agent 交付了 12 个 commits',
    latency: '实测延迟',
    remeasure: '重新测',
    download: '下载 PNG',
    copyImage: '复制图片',
    copied: '已复制',
    scan: '扫码直达',
    hint: '图片完全在你的浏览器里生成，不会上传。延迟数字来自你当前网络的实测。'
  },
  // Affiliate promo assets (user affiliate page)
  affiliateAssets: {
    title: '推广素材',
    description: '二维码、可复制文案和海报都带你的邀请码，扫码注册自动绑定。',
    qrTitle: '邀请二维码',
    qrHint: '右键或长按保存',
    copyTitle: '可复制文案',
    copy: '复制',
    copied: '已复制',
    posterTitle: '海报',
    posterHint: '.lol 风格分享图，右下角是你的邀请二维码',
    posterDownload: '下载海报',
    posterSquare: '正方形（闲鱼主图）',
    posterWide: '横版（社群 / 朋友圈）',
    templates: [
      '人去 rest，AI 去 build。一个地址接 Claude / GPT / Gemini，延迟实时可测，独立密钥可随时撤销。用我的邀请链接注册：{link}',
      '在用 Codex CLI / Claude Code 的朋友：{domain} 是独立第三方多模型接入站，BYOK 也支持，不碰你的账号。邀请链接：{link}',
      '学生 / 开源项目可以申请公益额度，不要求好评和拉新。规则和申请方式都在站内公开：{link}'
    ]
  },

  // Public status page (/status)
  statusPage: {
    title: '服务状态',
    subtitle: '由服务端定时探测（渠道监控）汇总的只读状态，每 60 秒自动刷新。',
    loading: '加载中…',
    error: '状态接口暂时不可用。',
    disabled: '站点尚未开放公开状态页。',
    empty: '暂无启用的监控项。',
    generatedAt: '生成于',
    autoRefresh: '每 60 秒刷新',
    since: '自',
    columns: { service: '服务', status: '状态', success: '成功率', samples: '样本', lastChecked: '最后探测' },
    overall: { operational: '全部正常', degraded: '部分降级', failed: '存在故障', unknown: '暂无数据' },
    status: { operational: '正常', degraded: '降级', failed: '故障', error: '错误', unknown: '未知' },
    method: '成功率与 P50 / P95 来自服务端对各上游的定时探测，样本数与窗口起点随数据一起给出；不代表你所在地区或时段的体验，实测请用延迟实测工具。'
  },

  // Common
}

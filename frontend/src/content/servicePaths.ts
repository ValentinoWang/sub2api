export const servicePaths = {
  zh: {
    title: '两种服务，各自通向你的工作流',
    description: '用 API 连接开发工具，或为自有账号办理订阅充值。按实际需求选择。',
    lanes: [
      { id: 'api', icon: 'server', title: 'API 模型接入', subtitle: '独立密钥 · 按量使用', steps: ['配置服务密钥', '多模型路由与额度管理', 'Codex / Claude Code 开发'], action: '查看接入指南', href: '/codex-cli' },
      { id: 'account', icon: 'creditCard', title: '账号订阅充值', subtitle: '自有账号 · 商品与订单可查', steps: ['核对套餐、周期与资料', '按订单处理并查看进度', '在自有账号核对权益'], action: '查看充值商品', href: 'https://www.ai.rest2build.lol/memberships' }
    ],
    note: '账号池用于网关内部调度，不作为共享账号出售。账号充值按商品与订单办理；账号订阅权益和本站 API 余额分别管理。',
    practiceTitle: '从能接入，到会使用，再到团队交付',
    resources: [
      { icon: 'terminal', title: '公益 Skills', description: '把配置和重复操作整理成可复用步骤，从现有接入指南开始。', action: '查看配置实践', href: '/codex-cli' },
      { icon: 'book', title: 'AI 使用经验', description: '遇到认证、模型或旧对话问题，按真实案例定位并恢复。', action: '查找排障经验', href: '/experiences' },
      { icon: 'checkCircle', title: 'Harness 工程', description: '将任务拆解、人工复核、测试和验收纳入团队开发流程。', action: '了解团队合作', href: '/business-invoice' }
    ]
  },
  en: {
    title: 'Two services for your workflow',
    description: 'Connect development tools through an API, or top up a subscription on your own account.',
    lanes: [
      { id: 'api', icon: 'server', title: 'API model access', subtitle: 'Your service key · Usage billing', steps: ['Configure a service key', 'Model routing and quota management', 'Develop with Codex / Claude Code'], action: 'Integration guide', href: '/codex-cli' },
      { id: 'account', icon: 'creditCard', title: 'Account subscriptions', subtitle: 'Your account · Product and order details', steps: ['Check plan, period and requirements', 'Track your order', 'Verify benefits in your account'], action: 'Account top-up products', href: 'https://www.ai.rest2build.lol/memberships' }
    ],
    note: 'Account pools support internal gateway routing and are not sold as shared accounts. Top-ups follow the product and order terms; account benefits and gateway API balance are separate.',
    practiceTitle: 'Connect, learn, and deliver as a team',
    resources: [
      { icon: 'terminal', title: 'Community Skills', description: 'Reusable configuration and repeatable tasks, starting with existing integration guides.', action: 'Configuration practice', href: '/codex-cli' },
      { icon: 'book', title: 'AI experience sharing', description: 'Real cases to diagnose authentication, model selection and conversation continuity.', action: 'Find troubleshooting advice', href: '/experiences' },
      { icon: 'checkCircle', title: 'Harness engineering', description: 'Task breakdown, human review, tests and acceptance as part of team development.', action: 'Team cooperation', href: '/business-invoice' }
    ]
  }
} as const

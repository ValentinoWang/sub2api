export default {
  codexHarvest: {
    title: '打票管理',
    description: '为 ChatGPT OAuth 账号后台采集 292 门票：选择打票代理池、设置速度与限额，查看各账号门票、代理成绩和打票流水。',
    refresh: '刷新',
    loadError: '加载打票状态失败',
    status: {
      enabled: '打票已开启',
      disabled: '打票未开启',
      failClosed: '无票时拦截门控模型',
      failOpen: '无票时照常转发',
      running: '本轮进行中',
      nextRound: '下一轮 {time}',
      lastRound: '上一轮 {time}',
      requests: '本轮请求 {used}/{budget}',
      currentProxy: '正在使用 {name}',
      openSettings: '在系统设置中开关'
    },
    idle: {
      disabled: '打票未开启，后台不会发出请求。',
      empty_pool: '打票代理池为空，后台不会发出请求。请在下方选入代理并保存。',
      pool_unavailable: '读取代理池失败，本轮已跳过。',
      all_ready: '所有账号的门票都可用，本轮无需补票。',
      round_budget: '本轮请求预算已用完，剩余账号下一轮再补。'
    },
    settingsError: '打票设置读取失败，正在沿用上一次有效的设置。',
    pool: {
      title: '打票代理池',
      description: '打票只使用这里选中的代理。日常业务仍走账号自己的代理。代理请先在「代理管理」中添加或导入订阅。',
      manageLink: '前往代理管理',
      empty: '代理管理中还没有可用的代理。',
      selected: '已选 {count} 个',
      columns: {
        select: '选用',
        name: '代理',
        address: '地址',
        exit: '出口',
        latency: '延迟',
        actions: '操作'
      },
      groups: '按分组选入',
      groupsHint: '点击分组，把其中当前可用的代理加入或移出打票代理池。🎫 打票出口是订阅里的住宅/家宽节点，💼 业务出口是机房节点。',
      residential: '住宅',
      datacenter: '机房',
      checkExit: '检测出口',
      checking: '检测中…',
      checkFailed: '出口检测失败',
      unknownExit: '未检测',
      unusable: '池中不可用',
      reasons: {
        deleted: '已删除',
        inactive: '已停用',
        expired: '已过期'
      }
    },
    speed: {
      title: '速度与限额',
      description: '门票 150 秒开始刷新、210 秒停止注入，每个预设的轮间隔都短于这 60 秒窗口。换代理不会恢复每小时额度。',
      presets: {
        slow: '慢速',
        standard: '标准',
        fast: '快速',
        burst: '突发',
        custom: '自定义'
      },
      fields: {
        round_interval_seconds: '每轮间隔（秒）',
        probe_interval_seconds: '请求间隔（秒）',
        attempt_timeout_seconds: '单次超时（秒）',
        cooldown_seconds: '失败冷却（秒）',
        max_requests_per_round: '每轮请求上限',
        max_proxy_attempts: '每账号换代理次数',
        max_requests_per_account_hour: '每账号每小时上限'
      },
      hints: {
        round_interval_seconds: '每隔多久检查一次需要补票的账号。',
        probe_interval_seconds: '任意两次打票请求之间的最短间隔。',
        attempt_timeout_seconds: '单次打票请求的最长等待时间。',
        cooldown_seconds: '一个账号模型打票失败后等待多久再试；上游要求更久时按上游。',
        max_requests_per_round: '一轮内所有账号合计最多发出的请求数。',
        max_proxy_attempts: '同一账号模型在一轮内最多试几个代理。',
        max_requests_per_account_hour: '每个账号一小时内最多发出的打票请求，重启后重新计数。'
      },
      range: '范围 {min}–{max}',
      save: '保存代理池与速度',
      saving: '保存中…',
      saved: '已保存，后台会立即按新设置运行',
      saveError: '保存失败'
    },
    accounts: {
      title: '账号门票',
      empty: '没有可打票的 ChatGPT OAuth 账号。',
      columns: {
        account: '账号',
        tickets: '门票',
        hour: '本小时请求',
        manual: '手动打票'
      },
      ready: '可用 · 剩余 {seconds} 秒',
      waiting: '等待补票',
      blocked: '无票拦截中',
      cooldown: '冷却至 {time}',
      proxy: '出票代理 {name}',
      notSchedulable: '不可调度',
      manualStart: '手动打票',
      manualRunning: '手动打票中（已发 {attempts} 次）',
      manualResult: '上次手动打票：{result}（{attempts} 次）'
    },
    manual: {
      title: '手动打票 · {name}',
      description: '在后台为这个账号连续打票，拿到合格门票即停止。每次请求都计入每小时上限。',
      models: '模型',
      maxAttempts: '每个模型最多尝试次数（1–20）',
      interval: '两次尝试间隔（秒，1–60）',
      start: '开始',
      cancel: '取消',
      started: '已开始，进度见打票流水',
      error: '无法开始手动打票',
      results: {
        harvested: '全部拿到',
        partial: '部分拿到',
        exhausted: '次数用完未拿到',
        hour_budget: '每小时上限已用完',
        account_error: '账号被上游拒绝',
        no_proxy: '代理都在冷却中',
        cancelled: '已取消'
      }
    },
    nodes: {
      title: '代理成绩',
      description: '按代理、账号、模型统计打票结果，排序时优先选近期出过票、成功率高的代理。',
      empty: '还没有打票记录。',
      resetAll: '全部重置',
      reset: '重置',
      resetConfirm: '确定清空所有代理成绩吗？清空后会重新轮换探索。',
      resetDone: '已重置',
      resetError: '重置失败',
      columns: {
        proxy: '代理',
        account: '账号',
        model: '模型',
        counts: '成功 / 未命中 / 网络 / 账号',
        streak: '连续失败',
        lastSuccess: '最近成功',
        cooldown: '冷却至',
        latency: '延迟',
        lastResult: '最近结果',
        actions: '操作'
      }
    },
    events: {
      title: '打票流水',
      description: '最近 200 条，重启后保留。不显示门票内容和代理密码。',
      empty: '暂无打票流水。',
      manual: '手动',
      columns: {
        time: '时间',
        account: '账号',
        model: '模型',
        proxy: '代理',
        result: '结果',
        detail: '说明'
      }
    },
    results: {
      success: '合格门票',
      stored: '已入库',
      invalid_state: '票据不合格',
      rate_limited: '上游限流',
      account_error: '账号被拒',
      network_error: '网络错误',
      upstream_error: '上游错误',
      response_incomplete_or_error: '响应未完成',
      token_error: '令牌失败',
      hour_budget: '小时额度用完',
      no_proxy: '无可用代理'
    },
    selection: {
      ticket_sticky: '沿用出票代理',
      recent_success: '近期出过票',
      explore: '轮换探索'
    }
  }
}

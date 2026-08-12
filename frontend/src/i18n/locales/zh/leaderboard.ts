export default {
    title: 'Token 排行榜',
    description: '按 Token、请求数或费用查看全站用量排名，仅展示 Top 20。',
    periodLabel: '排行周期',
    filters: '排行榜筛选',
    sortBy: '排序',
    billingMode: '计费模式',
    requestType: '请求类型',
    all: '全部',
    sort: {
      tokens: 'Token 总量',
      requests: '请求数',
      cost: '标准费用',
      actualCost: '实际扣费',
      accountCost: '账号成本'
    },
    billing: {
      token: 'Token',
      perRequest: '按次',
      image: '图片',
      video: '视频'
    },
    request: {
      sync: '同步',
      stream: '流式',
      wsV2: 'WebSocket',
      cyber: '安全拦截',
      live: 'Live'
    },
    period: {
      day: '今日',
      week: '近 7 天',
      month: '近 30 天'
    },
    rank: '排名',
    user: '用户',
    requests: '请求',
    totalTokens: '总 Token',
    inputTokensShort: '输入',
    outputTokensShort: '输出',
    cacheTokensShort: '缓存',
    imageOutputShort: '生图输出',
    cost: '费用',
    actualCost: '实际扣费',
    accountCost: '账号成本',
    lastActive: '最近活跃',
    currentUser: '我',
    me: '我',
    top: 'Top {count}',
    generatedAt: '更新于 {time}',
    refresh: '刷新',
    refreshing: '刷新中…',
    theme: {
      light: '浅色',
      dark: '深色'
    },
    emptyTitle: '暂无排名数据',
    emptyDescription: '当前周期暂无可展示的使用记录。',
    errorTitle: '排行榜加载失败',
    errorDescription: '请刷新或切换周期后重试。',
    retry: '重试'
  }

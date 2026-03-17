Title: Moonshot AI 开放平台 - Kimi K2.5 大模型 API 服务

URL Source: https://platform.moonshot.cn/docs/api/balance

Markdown Content:
查询余额 - Moonshot AI 开放平台 - Kimi K2.5 大模型 API 服务
===============

🎉 充值返券活动限时返场，助你畅享OpenClaw，快来体验吧！[了解更多](https://platform.moonshot.cn/docs/promotion)

[开放平台](https://platform.moonshot.cn/)[联系销售](https://platform.moonshot.cn/contact-sales)[Blog](https://platform.moonshot.cn/blog)[文档](https://platform.moonshot.cn/docs/overview)[开发工作台](https://platform.moonshot.cn/playground)[用户中心](https://platform.moonshot.cn/console)

⌘K

⌘K

*   [欢迎](https://platform.moonshot.cn/docs/overview)
*   [使用手册](https://platform.moonshot.cn/docs/introduction)
*   API 接口说明

    *   [Chat](https://platform.moonshot.cn/docs/api/chat)
    *   [Tool Use](https://platform.moonshot.cn/docs/api/tool-use)
    *   [Partial Mode](https://platform.moonshot.cn/docs/api/partial)
    *   [文件接口](https://platform.moonshot.cn/docs/api/files)
    *   [计算 Token](https://platform.moonshot.cn/docs/api/estimate)
    *   [查询余额](https://platform.moonshot.cn/docs/api/balance)

*   [🎉 促销活动](https://platform.moonshot.cn/docs/promotion)
*   产品定价

    *   [模型推理定价](https://platform.moonshot.cn/docs/pricing/chat)
    *   [联网搜索定价](https://platform.moonshot.cn/docs/pricing/tools)
    *   [充值与限速](https://platform.moonshot.cn/docs/pricing/limits)
    *   [常见问题](https://platform.moonshot.cn/docs/pricing/faq)

*   入门指南

    *   [Kimi K2.5 多模态模型](https://platform.moonshot.cn/docs/guide/kimi-k2-5-quickstart)
    *   [Kimi K2](https://platform.moonshot.cn/docs/guide/kimi-k2-quickstart)
    *   [使用思考模型](https://platform.moonshot.cn/docs/guide/use-kimi-k2-thinking-model)
    *   [开始使用 Kimi API](https://platform.moonshot.cn/docs/guide/start-using-kimi-api)
    *   [使用 OpenClaw 连接 Kimi K2.5 模型](https://platform.moonshot.cn/docs/guide/use-kimi-in-openclaw)
    *   [从 OpenAI 迁移到 Kimi API](https://platform.moonshot.cn/docs/guide/migrating-from-openai-to-kimi)
    *   [调试工具使用说明](https://platform.moonshot.cn/docs/guide/use-moonpalace)
    *   [多轮对话指南](https://platform.moonshot.cn/docs/guide/engage-in-multi-turn-conversations-using-kimi-api)
    *   [使用视觉模型](https://platform.moonshot.cn/docs/guide/use-kimi-vision-model)
    *   [自动断线重连](https://platform.moonshot.cn/docs/guide/auto-reconnect)
    *   [流式输出指南](https://platform.moonshot.cn/docs/guide/utilize-the-streaming-output-feature-of-kimi-api)
    *   [Tool Calls 能力说明](https://platform.moonshot.cn/docs/guide/use-kimi-api-to-complete-tool-calls)
    *   [使用联网搜索工具](https://platform.moonshot.cn/docs/guide/use-web-search)
    *   [JSON Mode 使用说明](https://platform.moonshot.cn/docs/guide/use-json-mode-feature-of-kimi-api)
    *   [Partial Mode 使用说明](https://platform.moonshot.cn/docs/guide/use-partial-mode-feature-of-kimi-api)
    *   [文件问答指南](https://platform.moonshot.cn/docs/guide/use-kimi-api-for-file-based-qa)
    *   [开发工作台调试模型指南](https://platform.moonshot.cn/docs/guide/use-playground-to-debug-the-model)
    *   [在编程工具中使用 Kimi K2 模型](https://platform.moonshot.cn/docs/guide/agent-support)
    *   [ModelScope MCP 服务器配置指南](https://platform.moonshot.cn/docs/guide/configure-the-modelscope-mcp-server)
    *   [Kimi 官方工具集成说明](https://platform.moonshot.cn/docs/guide/use-official-tools)
    *   [Kimi CLI 使用指南](https://platform.moonshot.cn/docs/guide/kimi-cli-support)
    *   [Kimi K2 模型搭建 Agent 指南](https://platform.moonshot.cn/docs/guide/use-kimi-k2-to-setup-agent)
    *   [基准评估最佳实践](https://platform.moonshot.cn/docs/guide/benchmark-best-practice)
    *   [Prompt 最佳实践](https://platform.moonshot.cn/docs/guide/prompt-best-practice)
    *   [组织管理最佳实践](https://platform.moonshot.cn/docs/guide/org-best-practice)
    *   [常见问题及解决方案](https://platform.moonshot.cn/docs/guide/faq)

*   条款与协议

    *   [平台服务协议](https://platform.moonshot.cn/docs/agreement/modeluse)
    *   [用户服务协议](https://platform.moonshot.cn/docs/agreement/userservice)
    *   [用户隐私协议](https://platform.moonshot.cn/docs/agreement/userprivacy)
    *   [充值协议](https://platform.moonshot.cn/docs/agreement/payment)

*   [Moonshot ↗ (opens in a new tab)](https://www.moonshot.cn/)
*   [Changelog ↗ (opens in a new tab)](https://platform.moonshot.cn/blog/posts/changelog)
*   [联系客服](https://platform.moonshot.cn/docs/api/balance) 
*   [开发者交流群](https://platform.moonshot.cn/docs/api/balance) 
*   [官方公众号](https://platform.moonshot.cn/docs/api/balance) 
*   [Global | platform.moonshot.ai↗ (opens in a new tab)](https://platform.moonshot.ai/)

目录

*   [请求地址](https://platform.moonshot.cn/docs/api/balance#%E8%AF%B7%E6%B1%82%E5%9C%B0%E5%9D%80)
*   [调用示例](https://platform.moonshot.cn/docs/api/balance#%E8%B0%83%E7%94%A8%E7%A4%BA%E4%BE%8B)
*   [返回内容](https://platform.moonshot.cn/docs/api/balance#%E8%BF%94%E5%9B%9E%E5%86%85%E5%AE%B9)
*   [返回内容说明](https://platform.moonshot.cn/docs/api/balance#%E8%BF%94%E5%9B%9E%E5%86%85%E5%AE%B9%E8%AF%B4%E6%98%8E)

回到顶部

文档

API 接口说明

查询余额

查询余额
====

请求地址[](https://platform.moonshot.cn/docs/api/balance#%E8%AF%B7%E6%B1%82%E5%9C%B0%E5%9D%80)
------------------------------------------------------------------------------------------

`GET https://api.moonshot.cn/v1/users/me/balance`

调用示例[](https://platform.moonshot.cn/docs/api/balance#%E8%B0%83%E7%94%A8%E7%A4%BA%E4%BE%8B)
------------------------------------------------------------------------------------------

`curl https://api.moonshot.cn/v1/users/me/balance -H "Authorization: Bearer $MOONSHOT_API_KEY"`

返回内容[](https://platform.moonshot.cn/docs/api/balance#%E8%BF%94%E5%9B%9E%E5%86%85%E5%AE%B9)
------------------------------------------------------------------------------------------

```
{
  "code": 0,
  "data": {
    "available_balance": 49.58894,
    "voucher_balance": 46.58893,
    "cash_balance": 3.00001
  },
  "scode": "0x0",
  "status": true
}
```

返回内容说明[](https://platform.moonshot.cn/docs/api/balance#%E8%BF%94%E5%9B%9E%E5%86%85%E5%AE%B9%E8%AF%B4%E6%98%8E)
--------------------------------------------------------------------------------------------------------------

| 字段 | 说明 | 类型 | 单位 |
| --- | --- | --- | --- |
| available_balance | 可用余额，包括现金余额和代金券余额, 当它小于等于 0 时, 用户不可调用推理 API | float | 人民币元（CNY） |
| voucher_balance | 代金券余额, 不会为负数 | float | 人民币元（CNY） |
| cash_balance | 现金余额, 可能为负数, 代表用户欠费, 当它为负数时, `available_balance` 为 `voucher_balance` 的值 | float | 人民币元（CNY） |

Last updated on 2026年2月9日

[计算 Token](https://platform.moonshot.cn/docs/api/estimate "计算 Token")[🎉 促销活动](https://platform.moonshot.cn/docs/promotion "🎉 促销活动")

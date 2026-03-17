Title: Moonshot AI 开放平台 - Kimi K2.5 大模型 API 服务

URL Source: https://platform.moonshot.cn/docs/guide/auto-reconnect

Markdown Content:
自动断线重连 - Moonshot AI 开放平台 - Kimi K2.5 大模型 API 服务
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
*   [联系客服](https://platform.moonshot.cn/docs/guide/auto-reconnect) 
*   [开发者交流群](https://platform.moonshot.cn/docs/guide/auto-reconnect) 
*   [官方公众号](https://platform.moonshot.cn/docs/guide/auto-reconnect) 
*   [Global | platform.moonshot.ai↗ (opens in a new tab)](https://platform.moonshot.ai/)

回到顶部

文档

入门指南

自动断线重连

自动断线重连
======

因为并发限制、复杂的网络环境等情况，一些时候我们的连接可能因为一些预期外的状况而中断，通常这种偶发的中断并不会持续很久，我们希望在这种情况下业务依然可以稳定运行，使用简单的代码即可实现断线重连的需求。

```
from openai import OpenAI
import time
 
client = OpenAI(
    api_key = "$MOONSHOT_API_KEY",
    base_url = "https://api.moonshot.cn/v1",
)
 
def chat_once(msgs):
    response = client.chat.completions.create(
        model = "kimi-k2-turbo-preview",
        messages = msgs,
        temperature = 0.6,
    )
    return response.choices[0].message.content
 
def chat(input: str, max_attempts: int = 100) -> str:
    messages = [
	    {"role": "system", "content": "你是 Kimi，由 Moonshot AI 提供的人工智能助手，你更擅长中文和英文的对话。你会为用户提供安全，有帮助，准确的回答。同时，你会拒绝一切涉及恐怖主义，种族歧视，黄色暴力等问题的回答。Moonshot AI 为专有名词，不可翻译成其他语言。"},
    ]
 
	# 我们将用户最新的问题构造成一个 message（role=user），并添加到 messages 的尾部
    messages.append({
		"role": "user",
		"content": input,	
	})
    st_time = time.time()  
    for i in range(max_attempts):
        print(f"Attempts: {i+1}/{max_attempts}")
        try:
            response = chat_once(messages)
            ed_time = time.time()
            print("Query Succuess!")
            print(f"Query Time: {ed_time-st_time}")
            return response
        except Exception as e:
            print(e)
            time.sleep(1)
            continue
 
    print("Query Failed.")
    return
 
print(chat("你好，请给我讲一个童话故事。"))
```

上面的代码实现了一个简单的断线重连功能，最多重复 100 次，每次连接之间等待 1s，你也可以根据具体的需求更改这些数值以及满足重试的条件。

Last updated on 2026年2月9日

[使用视觉模型](https://platform.moonshot.cn/docs/guide/use-kimi-vision-model "使用视觉模型")[流式输出指南](https://platform.moonshot.cn/docs/guide/utilize-the-streaming-output-feature-of-kimi-api "流式输出指南")

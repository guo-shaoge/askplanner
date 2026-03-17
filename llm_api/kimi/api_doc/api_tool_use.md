Title: Moonshot AI 开放平台 - Kimi K2.5 大模型 API 服务

URL Source: https://platform.moonshot.cn/docs/api/tool-use

Markdown Content:
工具调用 - Moonshot AI 开放平台 - Kimi K2.5 大模型 API 服务
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
*   [联系客服](https://platform.moonshot.cn/docs/api/tool-use) 
*   [开发者交流群](https://platform.moonshot.cn/docs/api/tool-use) 
*   [官方公众号](https://platform.moonshot.cn/docs/api/tool-use) 
*   [Global | platform.moonshot.ai↗ (opens in a new tab)](https://platform.moonshot.ai/)

目录

*   [工具配置](https://platform.moonshot.cn/docs/api/tool-use#%E5%B7%A5%E5%85%B7%E9%85%8D%E7%BD%AE)

回到顶部

文档

API 接口说明

Tool Use

工具调用
====

学会使用工具是智能的一个重要特征，在 Kimi 大模型中我们同样如此。Tool Use 或者 Function Calling 是 Kimi 大模型的一个重要功能，在调用 API 使用模型服务时，您可以在 Messages 中描述工具或函数，并让 Kimi 大模型智能地选择输出一个包含调用一个或多个函数所需的参数的 JSON 对象，实现让 Kimi 大模型链接使用外部工具的目的。

下面是一个简单的工具调用的例子：

```
{
  "model": "kimi-k2-turbo-preview",
  "messages": [
    {
      "role": "user",
      "content": "编程判断 3214567 是否是素数。"
    }
  ],
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "CodeRunner",
        "description": "代码执行器，支持运行 python 和 javascript 代码",
        "parameters": {
          "properties": {
            "language": {
              "type": "string",
              "enum": ["python", "javascript"]
            },
            "code": {
              "type": "string",
              "description": "代码写在这里"
            }
          },
          "type": "object"
        }
      }
    }
  ]
}
```

![Image 2: 上面例子的示意图](https://platform.moonshot.cn/_next/image?url=%2F_next%2Fstatic%2Fmedia%2Ftooluse_whiteboard_example.b33c4a6f.png&w=3840&q=75)

其中在 tools 字段，我们可以增加一组可选的工具列表。

每个工具列表必须包括一个类型，在 function 结构体中我们需要包括 name（它的需要遵守这样的正则表达式作为规范: ^[a-zA-Z_][a-zA-Z0-9-_]63$），这个名字如果是一个容易理解的英文可能会更加被模型所接受。以及一段 description 或者 enum，其中 description 部分介绍它能做什么功能，方便模型来判断和选择。 function 结构体中必须要有个 parameters 字段，parameters 的 root 必须是一个 object，内容是一个 json schema 的子集（之后我们会给出具体文档介绍相关技术细节）。 tools 的 function 个数目前不得超过 128 个。

和别的 API 一样，我们可以通过 Chat API 调用它。

python curl node.js

```
from openai import OpenAI
 
client = OpenAI(
    api_key = "$MOONSHOT_API_KEY",
    base_url = "https://api.moonshot.cn/v1",
)
 
completion = client.chat.completions.create(
    model = "kimi-k2-turbo-preview",
    messages = [
        {"role": "system", "content": "你是 Kimi，由 Moonshot AI 提供的人工智能助手，你更擅长中文和英文的对话。你会为用户提供安全，有帮助，准确的回答。同时，你会拒绝一切涉及恐怖主义，种族歧视，黄色暴力等问题的回答。Moonshot AI 为专有名词，不可翻译成其他语言。"},
        {"role": "user", "content": "编程判断 3214567 是否是素数。"}
    ],
    tools = [{
        "type": "function",
        "function": {
            "name": "CodeRunner",
            "description": "代码执行器，支持运行 python 和 javascript 代码",
            "parameters": {
                "properties": {
                    "language": {
                        "type": "string",
                        "enum": ["python", "javascript"]
                    },
                    "code": {
                        "type": "string",
                        "description": "代码写在这里"
                    }
                },
            "type": "object"
            }
        }
    }],
    temperature = 0.6,
)
 
print(completion.choices[0].message)
```

### 工具配置[](https://platform.moonshot.cn/docs/api/tool-use#%E5%B7%A5%E5%85%B7%E9%85%8D%E7%BD%AE)

你也可以使用一些 Agent 平台例如 [Coze (opens in a new tab)](https://coze.cn/)、[Bisheng (opens in a new tab)](https://github.com/dataelement/bisheng)、[Dify (opens in a new tab)](https://github.com/langgenius/dify/) 和 [LangChain (opens in a new tab)](https://github.com/langchain-ai/langchain) 等框架来创建和管理这些工具，并配合 Kimi 大模型设计更加复杂的工作流。

Last updated on 2026年2月9日

[Chat](https://platform.moonshot.cn/docs/api/chat "Chat")[Partial Mode](https://platform.moonshot.cn/docs/api/partial "Partial Mode")

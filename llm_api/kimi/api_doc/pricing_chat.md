Title: Moonshot AI 开放平台 - Kimi K2.5 大模型 API 服务

URL Source: https://platform.moonshot.cn/docs/pricing/chat

Markdown Content:
模型推理价格说明
--------

[](https://platform.moonshot.cn/docs/pricing/chat#%E8%AE%A1%E8%B4%B9%E5%9F%BA%E6%9C%AC%E6%A6%82%E5%BF%B5)
---------------------------------------------------------------------------------------------------------

### [](https://platform.moonshot.cn/docs/pricing/chat#%E8%AE%A1%E8%B4%B9%E5%8D%95%E5%85%83)

Token：代表常见的字符序列，每个汉字使用的 Token 数目可能是不同的。例如，单个汉字"夔"可能会被分解为若干 Token 的组合，而像"中国"这样短且常见的短语则可能会使用单个 Token。大致来说，对于一段通常的中文文本，1 个 Token 大约相当于 1.5-2 个汉字。具体每次调用实际产生的 Tokens 数量可以通过调用[计算 Token API](https://platform.moonshot.cn/docs/api/misc#%E8%AE%A1%E7%AE%97-token) 来获得。

#### [](https://platform.moonshot.cn/docs/pricing/chat#%E8%AE%A1%E8%B4%B9%E9%80%BB%E8%BE%91)

Chat Completion 接口收费：我们对 Input 和 Output 均实行按量计费。如果您上传并抽取文档内容，并将抽取的文档内容作为 Input 传输给模型，那么文档内容也将按量计费。文件相关接口（文件内容抽取/文件存储）接口**限时免费**，即您只上传并抽取文档，这个API本身不会产生费用。

[](https://platform.moonshot.cn/docs/pricing/chat#%E4%BA%A7%E5%93%81%E5%AE%9A%E4%BB%B7)
---------------------------------------------------------------------------------------

### [](https://platform.moonshot.cn/docs/pricing/chat#%E5%A4%9A%E6%A8%A1%E6%80%81%E6%A8%A1%E5%9E%8B-kimi-k25-)

| 模型 | 计费单位 | 输入价格 （缓存命中） | 输入价格 （缓存未命中） | 输出价格 | 模型上下文长度 |
| --- | --- | --- | --- | --- | --- |
| kimi-k2.5 | 1M tokens | ￥0.70 | ￥4.00 | ￥21.00 | 262,144 tokens |

*   kimi-k2.5 是 Kimi 迄今最全能的模型，原生的多模态架构设计，同时支持视觉与文本输入、思考与非思考模式、对话与 Agent 任务
*   模型上下文长度 256k，支持长思考擅长深度推理
*   支持自动上下文缓存功能，ToolCalls、JSON Mode、Partial Mode、联网搜索功能等能力

### [](https://platform.moonshot.cn/docs/pricing/chat#%E7%94%9F%E6%88%90%E6%A8%A1%E5%9E%8B-kimi-k2)

| 模型 | 计费单位 | 输入价格 （缓存命中） | 输入价格 （缓存未命中） | 输出价格 | 模型上下文长度 |
| --- | --- | --- | --- | --- | --- |
| kimi-k2-0905-preview | 1M tokens | ￥1.00 | ￥4.00 | ￥16.00 | 262,144 tokens |
| kimi-k2-0711-preview | 1M tokens | ￥1.00 | ￥4.00 | ￥16.00 | 131,072 tokens |
| kimi-k2-turbo-preview 推荐 | 1M tokens | ￥1.00 | ￥8.00 | ￥58.00 | 262,144 tokens |
| kimi-k2-thinking | 1M tokens | ￥1.00 | ￥4.00 | ￥16.00 | 262,144 tokens |
| kimi-k2-thinking-turbo | 1M tokens | ￥1.00 | ￥8.00 | ￥58.00 | 262,144 tokens |

*   kimi-k2 是一款具备超强代码和 Agent 能力的 MoE 架构基础模型，总参数 1T，激活参数 32B。在通用知识推理、编程、数学、Agent 等主要类别的基准性能测试中，K2 模型的性能超过其他主流开源模型
*   kimi-k2-0905-preview 模型上下文长度 256k，在 kimi-k2-0711-preview 能力的基础上，具备更强的 Agentic Coding 能力、更突出的前端代码的美观度和实用性、以及更好的上下文理解能力
*   kimi-k2-turbo-preview 模型上下文长度 256k，是 kimi k2 的高速版本模型，始终对标最新版本的 kimi-k2 模型（kimi-k2-0905-preview）。模型参数与 kimi-k2 一致，但输出速度已提至每秒 60 tokens，最高可达每秒 100 tokens
*   kimi-k2-0711-preview 模型上下文长度为 128k
*   kimi-k2-thinking 模型上下文长度 256k，是具有通用 Agentic 能力和推理能力的思考模型，它擅长深度推理[使用须知](https://platform.moonshot.cn/docs/guide/use-kimi-k2-thinking-model)
*   kimi-k2-thinking-turbo 模型上下文长度 256k，是 kimi-k2-thinking 模型的高速版，适用于需要深度推理和追求极致高速的场景
*   支持 ToolCalls、JSON Mode、Partial Mode、联网搜索功能等，不支持视觉功能
*   支持自动上下文缓存功能，缓存命中的 tokens 将按照输入价格（缓存命中）单价收费，您可以在[控制台](https://platform.moonshot.cn/console)中查看"context caching"类型的费用明细

### [](https://platform.moonshot.cn/docs/pricing/chat#%E7%94%9F%E6%88%90%E6%A8%A1%E5%9E%8B-moonshot-v1)

| 模型 | 计费单位 | 输入价格 | 输出价格 | 模型上下文长度 |
| --- | --- | --- | --- | --- |
| moonshot-v1-8k | 1M tokens | ￥2.00 | ￥10.00 | 8,192 tokens |
| moonshot-v1-32k | 1M tokens | ￥5.00 | ￥20.00 | 32,768 tokens |
| moonshot-v1-128k | 1M tokens | ￥10.00 | ￥30.00 | 131,072 tokens |
| moonshot-v1-8k-vision-preview | 1M tokens | ￥2.00 | ￥10.00 | 8,192 tokens |
| moonshot-v1-32k-vision-preview | 1M tokens | ￥5.00 | ￥20.00 | 32,768 tokens |
| moonshot-v1-128k-vision-preview | 1M tokens | ￥10.00 | ￥30.00 | 131,072 tokens |

此处 1M = 1,000,000，表格中的价格代表每消耗 1M tokens 的价格。

Last updated on 2026年2月9日

[🎉 促销活动](https://platform.moonshot.cn/docs/promotion "🎉 促销活动")[联网搜索定价](https://platform.moonshot.cn/docs/pricing/tools "联网搜索定价")

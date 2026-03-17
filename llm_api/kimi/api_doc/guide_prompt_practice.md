Title: Moonshot AI 开放平台 - Kimi K2.5 大模型 API 服务

URL Source: https://platform.moonshot.cn/docs/guide/prompt-best-practice

Markdown Content:
Prompt 最佳实践
-----------

> System Prompt最佳实践：system prompt（系统提示）指的是模型在生成文本或响应之前所接收的初始输入或指令，这个提示对于模型的运作至关[重要 (opens in a new tab)](https://kimi.moonshot.cn/share/col3fn2lnl95v16j0g2g)

[](https://platform.moonshot.cn/docs/guide/prompt-best-practice#%E7%BC%96%E5%86%99%E6%B8%85%E6%99%B0%E7%9A%84%E8%AF%B4%E6%98%8E)
--------------------------------------------------------------------------------------------------------------------------------

*   为什么需要向模型输出清晰的说明？

> 模型无法读懂你的想法，如果输出内容太长，可要求模型简短回复。如果输出内容太简单，可要求模型进行专家级写作。如果你不喜欢输出的格式，请向模型展示你希望看到的格式。模型越少猜测你的需求，你越有可能得到满意的结果。

### [](https://platform.moonshot.cn/docs/guide/prompt-best-practice#%E5%9C%A8%E8%AF%B7%E6%B1%82%E4%B8%AD%E5%8C%85%E5%90%AB%E6%9B%B4%E5%A4%9A%E7%BB%86%E8%8A%82%E5%8F%AF%E4%BB%A5%E8%8E%B7%E5%BE%97%E6%9B%B4%E7%9B%B8%E5%85%B3%E7%9A%84%E5%9B%9E%E7%AD%94)

> 为了获得高度相关的输出，请保证在输入请求中提供所有重要细节和背景。

### [](https://platform.moonshot.cn/docs/guide/prompt-best-practice#%E5%9C%A8%E8%AF%B7%E6%B1%82%E4%B8%AD%E8%A6%81%E6%B1%82%E6%A8%A1%E5%9E%8B%E6%89%AE%E6%BC%94%E4%B8%80%E4%B8%AA%E8%A7%92%E8%89%B2%E5%8F%AF%E4%BB%A5%E8%8E%B7%E5%BE%97%E6%9B%B4%E5%87%86%E7%A1%AE%E7%9A%84%E8%BE%93%E5%87%BA)

> 在 API 请求的'messages' 字段中增加指定模型在回复中使用的角色。

### [](https://platform.moonshot.cn/docs/guide/prompt-best-practice#%E5%9C%A8%E8%AF%B7%E6%B1%82%E4%B8%AD%E4%BD%BF%E7%94%A8%E5%88%86%E9%9A%94%E7%AC%A6%E6%9D%A5%E6%98%8E%E7%A1%AE%E6%8C%87%E5%87%BA%E8%BE%93%E5%85%A5%E7%9A%84%E4%B8%8D%E5%90%8C%E9%83%A8%E5%88%86)

> 例如使用三重引号/XML标签/章节标题等定界符可以帮助区分需要不同处理的文本部分。

### [](https://platform.moonshot.cn/docs/guide/prompt-best-practice#%E6%98%8E%E7%A1%AE%E5%AE%8C%E6%88%90%E4%BB%BB%E5%8A%A1%E6%89%80%E9%9C%80%E7%9A%84%E6%AD%A5%E9%AA%A4)

> 任务建议明确一系列步骤。明确写出这些步骤可以使模型更容易遵循并获得更好的输出。

### [](https://platform.moonshot.cn/docs/guide/prompt-best-practice#%E5%90%91%E6%A8%A1%E5%9E%8B%E6%8F%90%E4%BE%9B%E8%BE%93%E5%87%BA%E7%A4%BA%E4%BE%8B)

> 向模型提供一般指导的示例描述，通常比展示任务的所有排列让模型的输出更加高效。例如，如果你打算让模型复制一种难以明确描述的风格，来回应用户查询。这被称为“few-shot”提示。

### [](https://platform.moonshot.cn/docs/guide/prompt-best-practice#%E6%8C%87%E5%AE%9A%E6%9C%9F%E6%9C%9B%E6%A8%A1%E5%9E%8B%E8%BE%93%E5%87%BA%E7%9A%84%E9%95%BF%E5%BA%A6)

> 你可以要求模型生成特定目标长度的输出。目标输出长度可以用文数、句子数、段落数、项目符号等来指定。但请注意，指示模型生成特定数量的文字并不具有高精度。模型更擅长生成特定数量的段落或项目符号的输出。

[](https://platform.moonshot.cn/docs/guide/prompt-best-practice#%E6%8F%90%E4%BE%9B%E5%8F%82%E8%80%83%E6%96%87%E6%9C%AC)
-----------------------------------------------------------------------------------------------------------------------

### [](https://platform.moonshot.cn/docs/guide/prompt-best-practice#%E6%8C%87%E5%AF%BC%E6%A8%A1%E5%9E%8B%E4%BD%BF%E7%94%A8%E5%8F%82%E8%80%83%E6%96%87%E6%9C%AC%E6%9D%A5%E5%9B%9E%E7%AD%94%E9%97%AE%E9%A2%98)

> 如果您可以提供一个包含与当前查询相关的可信信息的模型，那么就可以指导模型使用所提供的信息来回答问题

[](https://platform.moonshot.cn/docs/guide/prompt-best-practice#%E6%8B%86%E5%88%86%E5%A4%8D%E6%9D%82%E7%9A%84%E4%BB%BB%E5%8A%A1)
--------------------------------------------------------------------------------------------------------------------------------

### [](https://platform.moonshot.cn/docs/guide/prompt-best-practice#%E9%80%9A%E8%BF%87%E5%88%86%E7%B1%BB%E6%9D%A5%E8%AF%86%E5%88%AB%E7%94%A8%E6%88%B7%E6%9F%A5%E8%AF%A2%E7%9B%B8%E5%85%B3%E7%9A%84%E6%8C%87%E4%BB%A4)

> 对于需要大量独立指令集来处理不同情况的任务来说，对查询类型进行分类，并使用该分类来明确需要哪些指令可能会帮助输出。

### [](https://platform.moonshot.cn/docs/guide/prompt-best-practice#%E5%AF%B9%E4%BA%8E%E8%BD%AE%E6%AC%A1%E8%BE%83%E9%95%BF%E7%9A%84%E5%AF%B9%E8%AF%9D%E5%BA%94%E7%94%A8%E7%A8%8B%E5%BA%8F%E6%80%BB%E7%BB%93%E6%88%96%E8%BF%87%E6%BB%A4%E4%B9%8B%E5%89%8D%E7%9A%84%E5%AF%B9%E8%AF%9D)

> 由于模型有固定的上下文长度显示，所以用户与模型助手之间的对话不能无限期地继续。

针对这个问题，一种解决方案是总结对话中的前几个回合。一旦输入的大小达到预定的阈值，就会触发一个查询来总结先前的对话部分，先前对话的摘要同样可以作为系统消息的一部分包含在内。或者，整个对话过程中的先前对话可以被异步总结。

### [](https://platform.moonshot.cn/docs/guide/prompt-best-practice#%E5%88%86%E5%9D%97%E6%A6%82%E6%8B%AC%E9%95%BF%E6%96%87%E6%A1%A3%E5%B9%B6%E9%80%92%E5%BD%92%E6%9E%84%E5%BB%BA%E5%AE%8C%E6%95%B4%E6%91%98%E8%A6%81)

> 要总结一本书的内容，我们可以使用一系列的查询来总结文档的每个章节。部分摘要可以汇总并总结，产生摘要的摘要。这个过程可以递归进行，直到整本书都被总结完毕。如果需要使用前面的章节来理解后面的部分，那么可以在总结书中给定点的内容时，包括对给定点之前的章节的摘要。

Last updated on 2026年2月9日

[基准评估最佳实践](https://platform.moonshot.cn/docs/guide/benchmark-best-practice "基准评估最佳实践")[组织管理最佳实践](https://platform.moonshot.cn/docs/guide/org-best-practice "组织管理最佳实践")

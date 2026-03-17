set -e
set -x
# API 接口文档
curl -s "https://r.jina.ai/https://platform.moonshot.cn/docs/api/chat" -o "api_chat.md"
curl -s "https://r.jina.ai/https://platform.moonshot.cn/docs/api/tool-use" -o "api_tool_use.md"
curl -s "https://r.jina.ai/https://platform.moonshot.cn/docs/api/partial" -o "api_partial.md"
curl -s "https://r.jina.ai/https://platform.moonshot.cn/docs/api/files" -o "api_files.md"
curl -s "https://r.jina.ai/https://platform.moonshot.cn/docs/api/estimate" -o "api_estimate.md"
curl -s "https://r.jina.ai/https://platform.moonshot.cn/docs/api/balance" -o "api_balance.md"

# 计费说明文档
curl -s "https://r.jina.ai/https://platform.moonshot.cn/docs/pricing/chat" -o "pricing_chat.md"
curl -s "https://r.jina.ai/https://platform.moonshot.cn/docs/pricing/tools" -o "pricing_tools.md"

# 开发指南文档
curl -s "https://r.jina.ai/https://platform.moonshot.cn/docs/guide/engage-in-multi-turn-conversations-using-kimi-api" -o "guide_multi_turn.md"
curl -s "https://r.jina.ai/https://platform.moonshot.cn/docs/guide/auto-reconnect" -o "guide_auto_reconnect.md"
curl -s "https://r.jina.ai/https://platform.moonshot.cn/docs/guide/utilize-the-streaming-output-feature-of-kimi-api" -o "guide_streaming.md"
curl -s "https://r.jina.ai/https://platform.moonshot.cn/docs/guide/use-kimi-api-to-complete-tool-calls" -o "guide_tool_calls.md"
curl -s "https://r.jina.ai/https://platform.moonshot.cn/docs/guide/use-web-search" -o "guide_web_search.md"
curl -s "https://r.jina.ai/https://platform.moonshot.cn/docs/guide/prompt-best-practice" -o "guide_prompt_practice.md"

echo "🎉 所有 Kimi 官方文档已成功转换为 Markdown 并下载到当前目录！"

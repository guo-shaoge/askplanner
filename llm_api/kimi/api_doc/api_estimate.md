Title: Moonshot AI 开放平台 - Kimi K2.5 大模型 API 服务

URL Source: https://platform.moonshot.cn/docs/api/estimate

Markdown Content:
文档

API 接口说明

计算 Token

计算 Token
--------

该接口用于计算请求某个请求（包括纯文本输入和视觉输入）的token数。

[](https://platform.moonshot.cn/docs/api/estimate#%E8%AF%B7%E6%B1%82%E5%9C%B0%E5%9D%80)
---------------------------------------------------------------------------------------

`POST https://api.moonshot.cn/v1/tokenizers/estimate-token-count`

[](https://platform.moonshot.cn/docs/api/estimate#%E8%AF%B7%E6%B1%82%E5%86%85%E5%AE%B9)
---------------------------------------------------------------------------------------

estimate-token-count 的输入结构体和 chat completion 基本一致。

[](https://platform.moonshot.cn/docs/api/estimate#%E7%A4%BA%E4%BE%8B)
---------------------------------------------------------------------

```
{
    "model": "kimi-k2-turbo-preview",
    "messages": [
        {
            "role": "system",
            "content": "你是 Kimi，由 Moonshot AI 提供的人工智能助手，你更擅长中文和英文的对话。你会为用户提供安全，有帮助，准确的回答。同时，你会拒绝一切涉及恐怖主义，种族歧视，黄色暴力等问题的回答。Moonshot AI 为专有名词，不可翻译成其他语言。"
        },
        { "role": "user", "content": "你好，我叫李雷，1+1等于多少？" }
    ]
}
```

[](https://platform.moonshot.cn/docs/api/estimate#%E5%AD%97%E6%AE%B5%E8%AF%B4%E6%98%8E)
---------------------------------------------------------------------------------------

| 字段 | 说明 | 类型 | 取值 |
| --- | --- | --- | --- |
| messages | 包含迄今为止对话的消息列表。 | List[Dict] | 这是一个结构体的列表，每个元素类似如下：`json{"role": "user", "content": "你好"}` role 只支持 `system`,`user`,`assistant` 其一，content 不得为空 |
| model | Model ID， 可以通过 List Models 获取 | string | 目前是 `kimi-k2.5`, `kimi-k2-0905-preview`,`kimi-k2-0711-preview`, `kimi-k2-turbo-preview`,`moonshot-v1-8k`,`moonshot-v1-32k`,`moonshot-v1-128k`, `moonshot-v1-auto`,`moonshot-v1-8k-vision-preview`,`moonshot-v1-32k-vision-preview`,`moonshot-v1-128k-vision-preview` 其一 |

[](https://platform.moonshot.cn/docs/api/estimate#%E8%B0%83%E7%94%A8%E7%A4%BA%E4%BE%8B)
---------------------------------------------------------------------------------------

*   纯文本调用

```
curl 'https://api.moonshot.cn/v1/tokenizers/estimate-token-count' \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $MOONSHOT_API_KEY" \
  -d '{
    "model": "kimi-k2-turbo-preview",
    "messages": [
        {
            "role": "system",
            "content": "你是 Kimi，由 Moonshot AI 提供的人工智能助手，你更擅长中文和英文的对话。你会为用户提供安全，有帮助，准确的回答。同时，你会拒绝一切涉及恐怖主义，种族歧视，黄色暴力等问题的回答。Moonshot AI 为专有名词，不可翻译成其他语言。"
        },
        {
            "role": "user",
            "content": "你好，我叫李雷，1+1等于多少？"
        }
    ]
}'
```

*   包含视觉的调用

```
import os
import base64
import json
import requests
 
api_key = os.environ.get("MOONSHOT_API_KEY")
endpoint = "https://api.moonshot.cn/v1/tokenizers/estimate-token-count"
image_path = "image.png"
 
with open(image_path, "rb") as f:
    image_data = f.read()
 
# 我们使用标准库 base64.b64encode 函数将图片编码成 base64 格式的 image_url
image_url = f"data:image/{os.path.splitext(image_path)[1]};base64,{base64.b64encode(image_data).decode('utf-8')}"
 
payload = {
    "model": "kimi-k2.5",
    "messages": [
        {
            "role": "system",
            "content": "你是 Kimi，由 Moonshot AI 提供的人工智能助手，你更擅长中文和英文的对话。你会为用户提供安全，有帮助，准确的回答。同时，你会拒绝一切涉及恐怖主义，种族歧视，黄色暴力等问题的回答。Moonshot AI 为专有名词，不可翻译成其他语言。"
        },
        {
            "role": "user",
            "content": [
                {
                    "type": "image_url", # <-- 使用 image_url 类型来上传图片，内容为使用 base64 编码过的图片内容
                    "image_url": {
                        "url": image_url,
                    },
                },
                {
                    "type": "text",
                    "text": "请描述图片的内容。", # <-- 使用 text 类型来提供文字指令，例如“描述图片内容”
                },
            ],
        }
    ]
}
 
response = requests.post(
    endpoint,
    headers={
        "Authorization": f"Bearer {api_key}",
        "Content-Type": "application/json"
    },
    data=json.dumps(payload)
)
 
print(response.json())
```

[](https://platform.moonshot.cn/docs/api/estimate#%E8%BF%94%E5%9B%9E%E5%86%85%E5%AE%B9)
---------------------------------------------------------------------------------------

```
{
    "data": {
        "total_tokens": 80
    }
}
```

当没有 error 字段，可以取 data.total_tokens 作为计算结果

Last updated on 2026年2月9日

[文件接口](https://platform.moonshot.cn/docs/api/files "文件接口")[查询余额](https://platform.moonshot.cn/docs/api/balance "查询余额")

# Vertex / Gemini Embeddings（new-api）

本仓库的最小构建对外暴露三个与 Embedding 相关的入口：

- `POST /v1/embeddings`：OpenAI embeddings 风格（默认 **仅文本**；对 `gemini-embedding-2-preview` 额外支持 `extra_body.google.*` 扩展传多模态/配置）。
- `POST /v1beta/models/{model}:embedContent`：Gemini/Vertex `embedContent` 风格（**单条**，可多模态，例如 `gemini-embedding-2-preview`）。
- `POST /v1beta/models/{model}:batchEmbedContents`：Gemini `batchEmbedContents` 风格（**批量**，文本/多模态均可）。

> 注意：`POST /v1/embeddings` 的标准 `input` 仍只支持文本（string / string[]）。
> 如果你需要在 `POST /v1/embeddings` 中传图片/音频/视频/PDF（仅 `gemini-embedding-2-preview`），可以使用本文档下方的 `extra_body.google.requests` 扩展（非 OpenAI 标准）。

## 1) `gemini-embedding-001`（OpenAI Style / 文本）

### Python（requests）

```python
import requests

BASE_URL = "http://localhost:3000"
API_KEY = "sk-your-token"

resp = requests.post(
    f"{BASE_URL}/v1/embeddings",
    headers={
        "Authorization": f"Bearer {API_KEY}",
        "Content-Type": "application/json",
    },
    json={
        "model": "gemini-embedding-001",
        "input": "hello world",
    },
    timeout=60,
)
print(resp.status_code, resp.text)
```

### JS（fetch）

```js
const BASE_URL = "http://localhost:3000";
const API_KEY = "sk-your-token";

const resp = await fetch(`${BASE_URL}/v1/embeddings`, {
  method: "POST",
  headers: {
    "Authorization": `Bearer ${API_KEY}`,
    "Content-Type": "application/json",
  },
  body: JSON.stringify({
    model: "gemini-embedding-001",
    input: "hello world",
  }),
});

console.log(resp.status, await resp.text());
```

## 1.5) `gemini-embedding-2-preview`（/v1/embeddings + extra_body / 可选多模态）

标准用法（纯文本）与 `gemini-embedding-001` 相同：

```json
{
  "model": "gemini-embedding-2-preview",
  "input": ["hello", "world"],
  "dimensions": 1024
}
```

如果你希望仍然走 `POST /v1/embeddings`，但同时传入 Gemini 的多模态 `content.parts[]` / `embedContentConfig` 等数据，可以使用扩展字段：

```json
{
  "model": "gemini-embedding-2-preview",
  "extra_body": {
    "google": {
      "requests": [
        {
          "content": { "role": "user", "parts": [{ "text": "The dog is cute" }] }
        },
        {
          "content": {
            "role": "user",
            "parts": [
              { "text": "embed this image" },
              { "inlineData": { "mimeType": "image/png", "data": "<base64>" } }
            ]
          }
        }
      ]
    }
  }
}
```

说明：

- 当 `extra_body.google.requests` 存在时，网关会优先使用它来构造上游 `:batchEmbedContents` 请求；此时顶层 `input` 可以省略或仅作为兼容字段保留。
- 对 `gemini-embedding-2-preview`，网关会复用与 `:embedContent` 相同的多模态校验逻辑，并在需要时将 `fileData.fileUri`（仅 `http(s)://`）下载并转换为 `inlineData` 后透传上游。

## 2) `gemini-embedding-2-preview`（Vertex embedContent / 多模态）

请求/响应体按 Vertex `embedContent` 标准透传（网关只做必要的校验与文件处理）。

### 支持的输入与限制（网关侧校验）

- 输入：文本、图片、音频、视频、PDF（通过 `content.parts[].text / inlineData / fileData` 传入）
- 输出：嵌入向量（响应体透传上游）
- `outputDimensionality`：1~3072（超出直接 400）
- 图片：
  - MIME：`image/png`、`image/jpeg`
  - 每个提示最多 6 张
  - **默认大小限制 20MB/张**（可通过系统设置 `embedding_limits.embedding2_image_max_mb` 调整）
- 文档（PDF）：
  - MIME：`application/pdf`
  - 每个提示最多 1 个
  - 页数限制：最多 6 页（best-effort 解析）
- 视频：
  - MIME：`video/mp4`、`video/mpeg`
  - 每个提示最多 1 个
  - 时长：含音频 ≤ 80s；不含音频 ≤ 120s
- 音频：
  - MIME：`audio/mp3`、`audio/wav`
  - 每个提示最多 1 个
  - 时长：≤ 80s
- 不支持 `gs://`（Google Cloud Storage）；如使用 `fileData.fileUri`，仅支持 `http(s)://`。
- 不做 MIME 转换：**用户必须提供正确的 MIME**（网关只按白名单校验）。
- 配置字段推荐使用 `embedContentConfig`（Vertex 标准）；网关也兼容旧字段 `config` / `embed_content_config`（会自动转为 `embedContentConfig` 透传上游）。

### Python（requests）- inlineData（推荐，适合本地文件）

```python
import base64
import pathlib
import requests

BASE_URL = "http://localhost:3000"
API_KEY = "sk-your-token"

img_b64 = base64.b64encode(pathlib.Path("demo.jpg").read_bytes()).decode("utf-8")

payload = {
  "content": {
    "role": "user",
    "parts": [
      {"text": "embed this image"},
      {"inlineData": {"mimeType": "image/jpeg", "data": img_b64}},
    ],
  },
  "embedContentConfig": {
    "outputDimensionality": 1024
  }
}

resp = requests.post(
  f"{BASE_URL}/v1beta/models/gemini-embedding-2-preview:embedContent",
  headers={"Authorization": f"Bearer {API_KEY}", "Content-Type": "application/json"},
  json=payload,
  timeout=120,
)
print(resp.status_code, resp.text)
```

### JS（fetch）- fileData（适合公网可访问 URL）

```js
const BASE_URL = "http://localhost:3000";
const API_KEY = "sk-your-token";

const payload = {
  content: {
    role: "user",
    parts: [
      {
        fileData: {
          mimeType: "application/pdf",
          fileUri: "https://example.com/sample.pdf"
        }
      }
    ]
  },
  embedContentConfig: {
    outputDimensionality: 1024
  }
};

const resp = await fetch(`${BASE_URL}/v1beta/models/gemini-embedding-2-preview:embedContent`, {
  method: "POST",
  headers: {
    "Authorization": `Bearer ${API_KEY}`,
    "Content-Type": "application/json",
  },
  body: JSON.stringify(payload),
});

console.log(resp.status, await resp.text());
```

## 3) `:batchEmbedContents`（Gemini Batch / 批量）

当你希望一次请求拿到多条 embedding（或需要批量多模态）时，可以使用：

- `POST /v1beta/models/{model}:batchEmbedContents`

网关会透传 Gemini batch embedding 的请求/响应体；其中 `requests[].content.parts[]` 支持 `text`、`inlineData`/`inline_data`、`fileData`/`file_data` 等字段。

对于 `gemini-embedding-2-preview`，网关会对每个 batch request 复用与 `:embedContent` 相同的多模态校验逻辑，并在需要时将 `fileData.fileUri`（仅 `http(s)://`）下载并转换为 `inlineData` 透传上游。

# Vertex / Gemini Embeddings（new-api）

本仓库的最小构建对外暴露两个与 Embedding 相关的入口：

- `POST /v1/embeddings`：OpenAI embeddings 风格（**仅文本**，例如 `gemini-embedding-001`）。
- `POST /v1beta/models/{model}:embedContent`：Vertex `embedContent` 风格（**多模态**，用于 `gemini-embedding-2-preview`）。

> 注意：`gemini-embedding-2-preview` 是多模态 embedding 模型，**不支持**在 `POST /v1/embeddings` 使用；请改用 `:embedContent`。

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
  "config": {
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
  config: {
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

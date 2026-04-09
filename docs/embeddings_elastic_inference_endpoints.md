# Elastic Inference Endpoints (EIS) Channel（new-api）

本仓库最小构建已支持 `Elastic Inference Endpoints` 渠道（Channel Type `60`），用于统一接入 Elasticsearch 的 `_inference` 推理接口，覆盖：

- `POST /v1/embeddings`（最终返回 OpenAI embeddings 格式）
- `POST /v1/rerank`（兼容 `dto.RerankResponse`）

## 1) 渠道配置

- 渠道类型：`Elastic Inference Endpoints`
- Base URL：你的 Elasticsearch / Elastic Cloud 集群入口，例如：`https://xxxx.us-central1.gcp.cloud.es.io`
- Key：Elastic `ApiKey`（可直接粘贴 `ApiKey ...` 或只填 key 内容）

网关发往上游的鉴权头：

- `Authorization: ApiKey <key>`

注意：不需要在 Base URL 末尾添加 `/v1`。

## 2) 上游 URL 拼接规则

用户只需要在请求里填写 `model`（推理端点 ID / 托管模型名），网关会按模型类型自动拼接上游路径：

- Embeddings：`POST {base_url}/_inference/text_embedding/{inference_id}`
- Rerank：`POST {base_url}/_inference/rerank/{inference_id}`

其中 `inference_id` 的规则：

- 若 `model` 以 `.` 开头：原样使用（例如 `.google-gemini-embedding-001`）
- 若 `model` 命中内置托管模型清单：自动补 `.` 前缀（例如 `google-gemini-embedding-001` -> `.google-gemini-embedding-001`）
- 其他情况：按原样作为 inference endpoint id（用于自定义 endpoint）

## 3) 请求示例

### 3.1 Embeddings

```bash
curl -X POST 'http://localhost:3000/v1/embeddings' \
  -H 'Authorization: Bearer sk-your-token' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "google-gemini-embedding-001",
    "input": ["text1", "text2"],
    "input_type": "ingest"
  }'
```

说明：

- 网关会将请求体转换为上游所需的 `{ input, input_type }`，不会增加额外字段。
- 若 `input_type` 未提供，默认使用 `"ingest"`。

### 3.2 Rerank

```bash
curl -X POST 'http://localhost:3000/v1/rerank' \
  -H 'Authorization: Bearer sk-your-token' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "jina-reranker-v3",
    "query": "Organic skincare products for sensitive skin",
    "top_n": 5,
    "documents": ["doc1", "doc2", "doc3"]
  }'
```

## 4) 支持模型（内置清单）

### Embedding models

- `jina-clip-v2`
- `jina-embeddings-v3`
- `jina-embeddings-v5`
- `jina-embeddings-v5-text-nano`
- `jina-embeddings-v5-text-small`
- `google-gemini-embedding-001`
- `openai-text-embedding-3-large`
- `openai-text-embedding-3-small`

### Rerank models

- `jina-reranker-v2-base-multilingual`
- `jina-reranker-v3`

说明：不在清单中的 `model` 也允许作为自定义 inference endpoint id 使用（不会自动补 `.`）。

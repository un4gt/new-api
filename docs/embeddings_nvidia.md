# Nvidia Embeddings Channel（new-api）

本仓库最小构建已支持 `Nvidia` 渠道（Channel Type `59`），用于统一接入 NVIDIA Build 上的指定 embedding 模型。

## 1) 渠道配置

- 渠道类型：`Nvidia`
- 默认 Base URL：`https://integrate.api.nvidia.com`
- 认证方式：`Bearer <NVIDIA_API_KEY>`
- 对外入口：`POST /v1/embeddings`

说明：
- 对于大多数 NVIDIA text embedding / multimodal embedding 模型，网关使用 OpenAI 兼容 embeddings 调用（`/v1/embeddings`）。
- 对于 `nv-dinov2`，网关自动改写为 NVIDIA 专用推理端点（`/v1/cv/nvidia/nv-dinov2`），并做请求格式适配。

## 2) 支持模型（仅以下白名单）

### text-to-embedding

- `nvidia/llama-nemotron-embed-1b-v2`
- `nvidia/llama-3.2-nemoretriever-300m-embed-v2`
- `nvidia/llama-3.2-nemoretriever-300m-embed-v1`
- `nvidia/nv-embed-v1`
- `baai/bge-m3`
- `nvidia/nv-embedqa-e5-v5`
- `nvidia/nv-embedcode-7b-v1`
- `nvidia/embed-qa-4`
- `nvidia/llama-3.2-nv-embedqa-1b-v2`
- `nvidia/llama-3.2-nv-embedqa-1b-v1`

### image-to-embedding / multimodal embedding

- `nvidia/llama-nemotron-embed-vl-1b-v2`
- `nvidia/llama-3.2-nemoretriever-1b-vlm-embed-v1`
- `nvidia/nv-dinov2`

未在上述列表中的模型会被网关拒绝（返回 unsupported model 错误）。

## 3) nv-dinov2 特殊规则

`nv-dinov2` 在网关内按 NVIDIA 官方文档规则分支处理：

- 图片 `< 200KB`：
  - 直接使用 base64 data URL
  - 请求体示例：`data:image/jpeg;base64,<...>`

- 图片 `>= 200KB`：
  - 先上传到 NVCF assets
  - 请求体使用：`data:image/{format};asset_id,{asset_id}`
  - 请求头自动补充：
    - `NVCF-INPUT-ASSET-REFERENCES: <asset_id>`（必需）
    - `NVCF-FUNCTION-ASSET-IDS: <asset_id>`（兼容性头，示例中常见）

### 支持输入格式

`nv-dinov2` 的 `input` 仅支持：

- 字符串（URL 或 base64/data URL）
- 单元素字符串数组（例如 `[
  "https://example.com/image.jpg"
]`）

不支持多图输入（数组长度必须为 1）。

## 4) 请求示例

### 4.1 text embedding（OpenAI 兼容）

```bash
curl -X POST 'http://localhost:3000/v1/embeddings' \
  -H 'Authorization: Bearer sk-your-token' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "nvidia/nv-embed-v1",
    "input": "hello world",
    "input_type": "query",
    "truncate": "NONE",
    "encoding_format": "float"
  }'
```

### 4.2 nv-dinov2（小图 / base64 直传）

```bash
curl -X POST 'http://localhost:3000/v1/embeddings' \
  -H 'Authorization: Bearer sk-your-token' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "nvidia/nv-dinov2",
    "input": "data:image/jpeg;base64,<BASE64_IMAGE>"
  }'
```

### 4.3 nv-dinov2（大图 / URL 输入，网关自动走 NVCF assets）

```bash
curl -X POST 'http://localhost:3000/v1/embeddings' \
  -H 'Authorization: Bearer sk-your-token' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "nvidia/nv-dinov2",
    "input": "https://example.com/large-image.jpg"
  }'
```

说明：
- 网关会先下载图片并检测大小。
- 若达到阈值（`>= 200KB`），自动创建并上传 NVCF asset，再调用推理端点。

## 5) 官方参考

- NVIDIA Build（入口）：`https://build.nvidia.com`
- nv-dinov2 页面：`https://build.nvidia.com/nvidia/nv-dinov2`
- API 参考：`https://docs.api.nvidia.com/nim/reference/nvidia-nv-dinov2`

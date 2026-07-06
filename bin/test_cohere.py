#!/usr/bin/env python3
"""
Cohere embeddings + rerank smoke test for new-api.

Calls the gateway using OpenAI-style endpoints:
  POST /v1/embeddings
  POST /v1/rerank

Image embeddings are tested with Cohere's native endpoint:
  POST /v2/embed

Usage:
  API_KEY=<YOUR_NEW_API_TOKEN> python3 test_cohere.py

Examples:
  API_KEY=sk-xxx python3 test_cohere.py \
    --base-url http://127.0.0.1:3000/v1 \
    --embedding-models embed-v4.0 \
    --rerank-models rerank-v3.5
"""

from __future__ import annotations

import argparse
import json
import os
import sys
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple

try:
    import requests  # type: ignore
except Exception:
    print("Missing dependency: requests (pip install requests)", file=sys.stderr)
    raise


DEFAULT_BASE_URL = os.getenv("BASE_URL", "http://127.0.0.1:3000/v1")
DEFAULT_EMBEDDING_MODEL = "embed-v4.0"
DEFAULT_IMAGE_EMBEDDING_MODEL = "embed-english-light-v3.0-image"
DEFAULT_RERANK_MODEL = "rerank-v3.5"
FALLBACK_EMBEDDING_MODELS = [
    "embed-english-light-v3.0",
    "embed-english-v3.0",
    "embed-multilingual-light-v3.0",
    "embed-multilingual-v3.0",
    "embed-v4.0",
]
FALLBACK_IMAGE_EMBEDDING_MODELS = [
    "embed-english-light-v3.0-image",
    "embed-english-v3.0-image",
    "embed-multilingual-light-v3.0-image",
    "embed-multilingual-v3.0-image",
]
FALLBACK_RERANK_MODELS = [
    "rerank-english-v3.0",
    "rerank-multilingual-v3.0",
    "rerank-v3.5",
    "rerank-v4.0-fast",
    "rerank-v4.0-pro",
]
TEST_IMAGE_DATA_URL = (
    "data:image/png;base64,"
    "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAFgwJ/"
    "l2uL0wAAAABJRU5ErkJggg=="
)


def _post(
    base_url: str,
    api_key: str,
    path: str,
    payload: Dict[str, Any],
    timeout: float,
) -> Tuple[int, Optional[Dict[str, Any]], str]:
    headers = {
        "Authorization": f"Bearer {api_key}",
        "Content-Type": "application/json",
        "Accept": "application/json",
    }
    resp = requests.post(
        f"{base_url}{path}",
        headers=headers,
        json=payload,
        timeout=timeout,
    )
    text = resp.text or ""
    try:
        data = resp.json()
    except Exception:
        data = None
    return resp.status_code, data, text


def _root_base_url(base_url: str) -> str:
    if base_url.endswith("/v1") or base_url.endswith("/v2"):
        return base_url[:-3]
    return base_url


def _must(cond: bool, msg: str) -> None:
    if not cond:
        raise AssertionError(msg)


def _is_error(status: int, data: Optional[Dict[str, Any]]) -> bool:
    return status != 200 or (isinstance(data, dict) and "error" in data)


def _preview(text: str, limit: int = 240) -> str:
    text = (text or "").strip().replace("\n", " ")
    return text if len(text) <= limit else text[:limit] + f"...(truncated {len(text)} chars)"


def _is_unsupported_image_embedding_error(text: str) -> bool:
    normalized = (text or "").lower()
    return (
        "does not support image embeddings" in normalized
        or "doesn't support image embeddings" in normalized
    )


def _check_embedding(
    data: Optional[Dict[str, Any]], expected_items: int, expected_model: Optional[str]
) -> int:
    _must(isinstance(data, dict), "embedding response must be an object")
    _must(data.get("object") in (None, "list"), f"unexpected object={data.get('object')!r}")
    if expected_model is not None and data.get("model") is not None:
        _must(data.get("model") == expected_model, f"unexpected model={data.get('model')!r}")

    items = data.get("data")
    _must(
        isinstance(items, list) and len(items) == expected_items,
        f"embedding.data must contain {expected_items} item(s)",
    )
    first = items[0]
    _must(isinstance(first, dict), "embedding.data[0] must be an object")
    embedding = first.get("embedding")
    _must(
        isinstance(embedding, list) and len(embedding) > 0,
        "embedding vector must be a non-empty list",
    )

    dim = len(embedding)
    for i, item in enumerate(items):
        _must(isinstance(item, dict), f"embedding.data[{i}] must be an object")
        vector = item.get("embedding")
        _must(
            isinstance(vector, list) and len(vector) == dim,
            f"embedding dim mismatch at index={i}",
        )
        _must(
            all(isinstance(value, (int, float)) for value in vector[:8]),
            f"embedding.data[{i}].embedding must contain numbers",
        )

    usage = data.get("usage")
    if usage is not None:
        _must(isinstance(usage, dict), "embedding.usage must be an object")
        total_tokens = usage.get("total_tokens")
        if total_tokens is not None:
            _must(isinstance(total_tokens, int), "embedding usage.total_tokens must be int")

    return dim


def _check_native_embedding(data: Optional[Dict[str, Any]], expected_items: int) -> int:
    _must(isinstance(data, dict), "native embedding response must be an object")
    embeddings = data.get("embeddings")
    _must(isinstance(embeddings, dict), "native embedding response missing embeddings")
    vectors = embeddings.get("float") or embeddings.get("float_")
    _must(
        isinstance(vectors, list) and len(vectors) == expected_items,
        f"native embeddings.float must contain {expected_items} item(s)",
    )
    first = vectors[0]
    _must(isinstance(first, list) and len(first) > 0, "native embedding vector is empty")
    dim = len(first)
    for i, vector in enumerate(vectors):
        _must(isinstance(vector, list) and len(vector) == dim, f"native embedding dim mismatch at index={i}")
        _must(
            all(isinstance(value, (int, float)) for value in vector[:8]),
            f"native embeddings.float[{i}] must contain numbers",
        )
    return dim


def _try_image_embedding(
    base_url: str,
    api_key: str,
    model: str,
    timeout: float,
) -> Tuple[int, Optional[Dict[str, Any]], str]:
    payload = {
        "model": model,
        "input_type": "image",
        "images": [TEST_IMAGE_DATA_URL],
        "embedding_types": ["float"],
    }
    return _post(_root_base_url(base_url), api_key, "/v2/embed", payload, timeout)


def _check_rerank(data: Optional[Dict[str, Any]], docs_len: int) -> int:
    _must(isinstance(data, dict), "rerank response must be an object")
    results = data.get("results")
    _must(
        isinstance(results, list) and len(results) > 0,
        "rerank.results must be a non-empty list",
    )
    for i, item in enumerate(results):
        _must(isinstance(item, dict), f"rerank.results[{i}] must be an object")
        idx = item.get("index")
        score = item.get("relevance_score")
        _must(isinstance(idx, int) and 0 <= idx < docs_len, f"invalid result index: {idx}")
        _must(isinstance(score, (int, float)), f"invalid relevance_score: {score}")
        if "document" in item and item.get("document") is not None:
            _must(
                isinstance(item.get("document"), (str, dict)),
                f"invalid document type: {type(item.get('document'))}",
            )

    usage = data.get("usage")
    if usage is not None:
        _must(isinstance(usage, dict), "rerank.usage must be an object")
        total_tokens = usage.get("total_tokens")
        if total_tokens is not None:
            _must(isinstance(total_tokens, int), "rerank usage.total_tokens must be int")

    return len(results)


def _parse_csv(value: str) -> List[str]:
    return [item.strip() for item in value.split(",") if item.strip()]


def _load_models_from_json(filename: str, endpoint: str) -> List[str]:
    script_dir = Path(__file__).resolve().parent
    paths = [
        script_dir / filename,
        script_dir.parent / filename,
    ]
    path = next((candidate for candidate in paths if candidate.exists()), paths[0])
    try:
        with path.open("r", encoding="utf-8") as f:
            raw = json.load(f)
    except FileNotFoundError:
        return []
    models = raw.get("models") if isinstance(raw, dict) else None
    if not isinstance(models, list):
        return []

    out = []
    for item in models:
        if not isinstance(item, dict):
            continue
        endpoints = item.get("endpoints")
        name = item.get("name")
        if isinstance(name, str) and isinstance(endpoints, list) and endpoint in endpoints:
            out.append(name)
    return out


def _select_models(
    models_arg: str,
    models_env_name: str,
    single_model: str,
    single_env_name: str,
    default_single_model: str,
    default_models: List[str],
) -> List[str]:
    models_from_arg = _parse_csv(models_arg)
    if models_from_arg:
        return models_from_arg

    models_from_env = _parse_csv(os.getenv(models_env_name, ""))
    if models_from_env:
        return models_from_env

    if os.getenv(single_env_name) or single_model != default_single_model:
        return [single_model]

    return default_models


def main() -> int:
    parser = argparse.ArgumentParser(description="new-api Cohere embeddings/rerank smoke test")
    parser.add_argument("--base-url", default=DEFAULT_BASE_URL, help="Gateway base URL with /v1")
    parser.add_argument(
        "--embedding-model",
        default=os.getenv("EMBEDDING_MODEL", DEFAULT_EMBEDDING_MODEL),
        help="Cohere embedding model",
    )
    parser.add_argument(
        "--embedding-models",
        default="",
        help="Optional comma-separated text embedding model list",
    )
    parser.add_argument(
        "--image-embedding-model",
        default=os.getenv("IMAGE_EMBEDDING_MODEL", DEFAULT_IMAGE_EMBEDDING_MODEL),
        help="Cohere image embedding model",
    )
    parser.add_argument(
        "--image-embedding-models",
        default="",
        help="Optional comma-separated image embedding model list",
    )
    parser.add_argument(
        "--rerank-model",
        default=os.getenv("RERANK_MODEL", DEFAULT_RERANK_MODEL),
        help="Cohere rerank model",
    )
    parser.add_argument(
        "--rerank-models",
        default="",
        help="Optional comma-separated rerank model list",
    )
    parser.add_argument("--timeout", type=float, default=120.0, help="Request timeout seconds")
    args = parser.parse_args()

    api_key = os.getenv("API_KEY")
    if not api_key:
        raise SystemExit("Missing env API_KEY. Example: API_KEY=xxx python3 test_cohere.py")

    base_url = args.base_url.rstrip("/")
    default_embedding_models = (
        _load_models_from_json("cohere-embed.json", "embed")
        or FALLBACK_EMBEDDING_MODELS
    )
    default_image_embedding_models = (
        _load_models_from_json("cohere-embed.json", "embed_image")
        or FALLBACK_IMAGE_EMBEDDING_MODELS
    )
    default_rerank_models = (
        _load_models_from_json("cohere-rerank.json", "rerank")
        or FALLBACK_RERANK_MODELS
    )
    embedding_models = _select_models(
        args.embedding_models,
        "EMBEDDING_MODELS",
        args.embedding_model,
        "EMBEDDING_MODEL",
        DEFAULT_EMBEDDING_MODEL,
        default_embedding_models,
    )
    image_embedding_models = _select_models(
        args.image_embedding_models,
        "IMAGE_EMBEDDING_MODELS",
        args.image_embedding_model,
        "IMAGE_EMBEDDING_MODEL",
        DEFAULT_IMAGE_EMBEDDING_MODEL,
        default_image_embedding_models,
    )
    rerank_models = _select_models(
        args.rerank_models,
        "RERANK_MODELS",
        args.rerank_model,
        "RERANK_MODEL",
        DEFAULT_RERANK_MODEL,
        default_rerank_models,
    )
    failures = []
    saw_embedding_success = False
    saw_rerank_success = False

    docs = [
        "RAG combines retrieval with generation by conditioning a language model on external documents.",
        "Retrieval-Augmented Generation improves factual accuracy by grounding answers.",
        "Cohere provides embedding and rerank models for search and retrieval applications.",
    ]
    for model in embedding_models:
        try:
            status, data, text = _post(
                base_url,
                api_key,
                "/embeddings",
                {
                    "model": model,
                    "input": "hello cohere embeddings",
                    "input_type": "search_document",
                    "encoding_format": "float",
                    "embedding_types": ["float"],
                },
                args.timeout,
            )
            _must(
                not _is_error(status, data),
                f"embedding failed: {status} {_preview(text)}",
            )
            dim = _check_embedding(data, expected_items=1, expected_model=None)
            print(f"[OK] embeddings(text) model={model} dim={dim}")

            status, data, text = _post(
                base_url,
                api_key,
                "/embeddings",
                {
                    "model": model,
                    "input": docs,
                    "input_type": "search_document",
                    "encoding_format": "float",
                    "embedding_types": ["float"],
                },
                args.timeout,
            )
            _must(
                not _is_error(status, data),
                f"batch embedding failed: {status} {_preview(text)}",
            )
            dim = _check_embedding(data, expected_items=len(docs), expected_model=None)
            print(f"[OK] embeddings(batch) model={model} dim={dim} items={len(docs)}")
            saw_embedding_success = True
        except Exception as e:
            failures.append(f"embeddings {model}: {e}")

    for model in image_embedding_models:
        try:
            status, data, text = _try_image_embedding(
                base_url,
                api_key,
                model,
                args.timeout,
            )
            if _is_unsupported_image_embedding_error(text):
                print(f"[SKIP] embeddings(image) model={model}: upstream reports image embeddings unsupported")
                continue
            _must(
                not _is_error(status, data),
                f"image embedding failed: {status} {_preview(text)}",
            )
            dim = _check_native_embedding(data, expected_items=1)
            print(f"[OK] embeddings(image) model={model} dim={dim} endpoint=/v2/embed")
            saw_embedding_success = True
        except Exception as e:
            failures.append(f"embeddings(image) {model}: {e}")

    query = "How does Cohere help with retrieval augmented generation?"
    for model in rerank_models:
        try:
            status, data, text = _post(
                base_url,
                api_key,
                "/rerank",
                {
                    "model": model,
                    "query": query,
                    "documents": docs,
                    "top_n": min(3, len(docs)),
                },
                args.timeout,
            )
            _must(not _is_error(status, data), f"rerank failed: {status} {_preview(text)}")
            count = _check_rerank(data, len(docs))
            print(f"[OK] rerank model={model} results={count}")
            saw_rerank_success = True
        except Exception as e:
            failures.append(f"rerank {model}: {e}")

    if saw_embedding_success:
        try:
            status, data, text = _post(
                base_url,
                api_key,
                "/embeddings",
                {"model": embedding_models[0]},
                args.timeout,
            )
            _must(
                _is_error(status, data),
                f"expected missing input to fail, got {status}: {_preview(text)}",
            )
            print("[OK] embeddings(edge) missing input rejected")
        except Exception as e:
            failures.append(f"embeddings(edge) missing input: {e}")

    if saw_rerank_success:
        try:
            status, data, text = _post(
                base_url,
                api_key,
                "/rerank",
                {"model": rerank_models[0], "query": query},
                args.timeout,
            )
            _must(
                _is_error(status, data),
                f"expected missing documents to fail, got {status}: {_preview(text)}",
            )
            print("[OK] rerank(edge) missing documents rejected")
        except Exception as e:
            failures.append(f"rerank(edge) missing documents: {e}")

    if failures:
        print("\nFAILED:")
        for failure in failures:
            print(f" - {failure}")
        return 1

    print("\nALL OK")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

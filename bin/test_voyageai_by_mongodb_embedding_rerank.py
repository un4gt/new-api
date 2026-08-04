#!/usr/bin/env python3
"""
Voyage AI by MongoDB embeddings + rerank smoke test for new-api.

Covered endpoints:
  POST /v1/embeddings
  POST /v1/rerank

Usage:
  BASE_URL=http://127.0.0.1:3000 API_KEY=<TOKEN> \
    python3 bin/test_voyageai_by_mongodb_embedding_rerank.py

BASE_URL may be either the gateway root or end in /v1. TIMEOUT optionally
overrides the per-request timeout in seconds (default: 120).

Provider documentation:
  https://www.mongodb.com/docs/voyageai/models/text-embeddings/
  https://www.mongodb.com/docs/voyageai/models/multimodal-embeddings/
  https://www.mongodb.com/docs/voyageai/models/rerankers/
"""

from __future__ import annotations

import os
import sys
from typing import Any, Callable, Dict, List, Tuple

try:
    import requests  # type: ignore
except Exception:
    print("Missing dependency: requests (pip install requests)", file=sys.stderr)
    raise


BASE_URL = (os.getenv("BASE_URL") or "").strip().rstrip("/")
API_KEY = (os.getenv("API_KEY") or "").strip()

if not BASE_URL:
    raise SystemExit(
        "Missing env BASE_URL. Example: BASE_URL=http://127.0.0.1:3000 "
        "API_KEY=xxx python3 bin/test_voyageai_by_mongodb_embedding_rerank.py"
    )
if not API_KEY:
    raise SystemExit(
        "Missing env API_KEY. Example: BASE_URL=http://127.0.0.1:3000 "
        "API_KEY=xxx python3 bin/test_voyageai_by_mongodb_embedding_rerank.py"
    )

try:
    TIMEOUT = float((os.getenv("TIMEOUT") or "120").strip())
except ValueError as error:
    raise SystemExit("TIMEOUT must be a number of seconds") from error
if TIMEOUT <= 0:
    raise SystemExit("TIMEOUT must be greater than zero")

API_BASE_URL = BASE_URL if BASE_URL.endswith("/v1") else f"{BASE_URL}/v1"
HEADERS = {
    "Authorization": f"Bearer {API_KEY}",
    "Content-Type": "application/json",
    "Accept": "application/json",
}

TEXT_EMBED_MODELS = [
    "voyage-4-large",
    "voyage-4",
    "voyage-4-lite",
    "voyage-code-3",
    "voyage-4-nano",
]
MULTIMODAL_EMBED_MODEL = "voyage-multimodal-3.5"
RERANK_MODELS = ["rerank-2.5", "rerank-2.5-lite"]

TEXT_BATCH_INPUTS = [
    "MongoDB is redefining what a database is in the AI era.",
    "Voyage AI embedding and reranking models support semantic retrieval.",
]
RERANK_DOCUMENTS = [
    "MongoDB stores application data as flexible documents.",
    "Voyage AI provides embedding and reranking models.",
    "A relational database organizes data into tables and rows.",
]
RERANK_QUERY = "Which document describes Voyage AI models?"

# Tiny transparent 1x1 PNG. The data URL keeps the smoke test self-contained.
_PNG_BASE64 = (
    "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/xcA"
    "An8B9pYy3QAAAABJRU5ErkJggg=="
)
IMAGE_DATA_URL = "data:image/png;base64," + _PNG_BASE64
MULTIMODAL_INPUTS = [
    {
        "content": [
            {"type": "text", "text": "A transparent test image."},
            {"type": "image_base64", "image_base64": IMAGE_DATA_URL},
        ]
    }
]


def _post(path: str, payload: Dict[str, Any]) -> Tuple[int, Any, str]:
    response = requests.post(
        url=f"{API_BASE_URL}{path}",
        headers=HEADERS,
        json=payload,
        timeout=TIMEOUT,
    )
    text = response.text or ""
    try:
        data = response.json()
    except Exception:
        data = None
    return response.status_code, data, text


def _must(condition: bool, message: str) -> None:
    if not condition:
        raise AssertionError(message)


def _is_error(status: int, data: Any) -> bool:
    return status != 200 or (isinstance(data, dict) and "error" in data)


def _preview(text: str, limit: int = 320) -> str:
    compact = (text or "").strip().replace("\n", " ")
    if len(compact) <= limit:
        return compact
    return compact[:limit] + f"...(truncated {len(compact)} chars)"


def _is_number(value: Any) -> bool:
    return isinstance(value, (int, float)) and not isinstance(value, bool)


def _check_usage(data: Dict[str, Any]) -> None:
    usage = data.get("usage")
    _must(isinstance(usage, dict), "response.usage must be an object")
    prompt_tokens = usage.get("prompt_tokens")
    total_tokens = usage.get("total_tokens")
    _must(isinstance(prompt_tokens, int), "usage.prompt_tokens must be an integer")
    _must(isinstance(total_tokens, int), "usage.total_tokens must be an integer")
    _must(total_tokens >= prompt_tokens >= 0, "usage token counts are inconsistent")


def _parse_embeddings(data: Any, expected_model: str, expected_items: int) -> int:
    _must(isinstance(data, dict), "embeddings response must be a JSON object")
    _must(data.get("object") == "list", f"unexpected object={data.get('object')!r}")
    _must(data.get("model") == expected_model, f"unexpected model={data.get('model')!r}")

    items = data.get("data")
    _must(
        isinstance(items, list) and len(items) == expected_items,
        f"expected {expected_items} embedding items, got "
        f"{len(items) if isinstance(items, list) else type(items).__name__}",
    )

    dimension = 0
    seen_indexes = set()
    for position, item in enumerate(items):
        _must(isinstance(item, dict), f"data[{position}] must be an object")
        _must(item.get("object") == "embedding", f"unexpected data[{position}].object")
        index = item.get("index")
        _must(
            isinstance(index, int) and 0 <= index < expected_items,
            f"invalid data[{position}].index={index!r}",
        )
        _must(index not in seen_indexes, f"duplicate embedding index={index}")
        seen_indexes.add(index)

        embedding = item.get("embedding")
        _must(
            isinstance(embedding, list) and embedding,
            f"data[{position}].embedding must be a non-empty list",
        )
        _must(
            all(_is_number(value) for value in embedding),
            f"data[{position}].embedding must contain only numbers",
        )
        if position == 0:
            dimension = len(embedding)
        _must(len(embedding) == dimension, f"embedding dimension mismatch at position={position}")

    _check_usage(data)
    return dimension


def _parse_rerank(
    data: Any,
    max_results: int,
    expect_documents: bool,
) -> int:
    _must(isinstance(data, dict), "rerank response must be a JSON object")
    results = data.get("results")
    _must(isinstance(results, list) and results, "rerank.results must be a non-empty list")
    _must(len(results) <= max_results, f"expected at most {max_results} rerank results")

    seen_indexes = set()
    for position, item in enumerate(results):
        _must(isinstance(item, dict), f"results[{position}] must be an object")
        index = item.get("index")
        _must(
            isinstance(index, int) and 0 <= index < len(RERANK_DOCUMENTS),
            f"invalid results[{position}].index={index!r}",
        )
        _must(index not in seen_indexes, f"duplicate rerank index={index}")
        seen_indexes.add(index)
        _must(
            _is_number(item.get("relevance_score")),
            f"results[{position}].relevance_score must be a number",
        )
        if expect_documents:
            _must("document" in item, f"results[{position}].document is missing")
            _must(
                item.get("document") == RERANK_DOCUMENTS[index],
                f"unexpected results[{position}].document for index={index}",
            )

    _check_usage(data)
    return len(results)


def _test_text_embedding(model: str, batch: bool) -> None:
    input_value: Any
    input_type: str
    if batch:
        input_value = TEXT_BATCH_INPUTS
        input_type = "document"
    else:
        input_value = "How do Voyage AI embeddings support semantic search?"
        input_type = "query"

    status, data, text = _post(
        "/embeddings",
        {
            "model": model,
            "input": input_value,
            "input_type": input_type,
            "truncation": False,
            "encoding_format": "float",
        },
    )
    _must(not _is_error(status, data), f"status={status}, body={_preview(text)}")
    expected_items = len(TEXT_BATCH_INPUTS) if batch else 1
    dimension = _parse_embeddings(data, model, expected_items)
    mode = "batch" if batch else "single"
    print(f"[OK] embeddings({mode}) model={model} dim={dimension} items={expected_items}")


def _test_multimodal_embedding(input_field: str) -> None:
    payload = {
        "model": MULTIMODAL_EMBED_MODEL,
        input_field: MULTIMODAL_INPUTS,
        "input_type": "document",
        "truncation": False,
    }
    status, data, text = _post("/embeddings", payload)
    _must(not _is_error(status, data), f"status={status}, body={_preview(text)}")
    dimension = _parse_embeddings(data, MULTIMODAL_EMBED_MODEL, len(MULTIMODAL_INPUTS))
    print(
        f"[OK] embeddings(multimodal-{input_field}) "
        f"model={MULTIMODAL_EMBED_MODEL} dim={dimension}"
    )


def _test_rerank(model: str, limit_field: str) -> None:
    max_results = 2
    status, data, text = _post(
        "/rerank",
        {
            "model": model,
            "query": RERANK_QUERY,
            "documents": RERANK_DOCUMENTS,
            limit_field: max_results,
            "return_documents": True,
            "truncation": False,
        },
    )
    _must(not _is_error(status, data), f"status={status}, body={_preview(text)}")
    result_count = _parse_rerank(
        data,
        max_results=max_results,
        expect_documents=True,
    )
    print(f"[OK] rerank({limit_field}) model={model} results={result_count}")


def _run_case(
    failures: List[str],
    label: str,
    test: Callable[..., None],
    *args: Any,
) -> None:
    try:
        test(*args)
    except requests.exceptions.RequestException as error:
        raise SystemExit(f"Cannot reach gateway at {API_BASE_URL}: {error}") from error
    except Exception as error:
        failures.append(f"{label}: {error}")


def main() -> int:
    print(f"VoyageAIByMongoDB embeddings endpoint: {API_BASE_URL}/embeddings")
    print(f"VoyageAIByMongoDB rerank endpoint: {API_BASE_URL}/rerank")
    failures: List[str] = []

    for model in TEXT_EMBED_MODELS:
        _run_case(failures, f"embeddings(single) {model}", _test_text_embedding, model, False)
        _run_case(failures, f"embeddings(batch) {model}", _test_text_embedding, model, True)

    # `inputs` is the provider-native field; `input` exercises gateway compatibility.
    _run_case(failures, "embeddings(multimodal-inputs)", _test_multimodal_embedding, "inputs")
    _run_case(failures, "embeddings(multimodal-input)", _test_multimodal_embedding, "input")

    for model in RERANK_MODELS:
        # `top_k` is provider-native; `top_n` exercises the OpenAI-style alias.
        _run_case(failures, f"rerank(top_k) {model}", _test_rerank, model, "top_k")
        _run_case(failures, f"rerank(top_n) {model}", _test_rerank, model, "top_n")

    if failures:
        print("\nFAILED:")
        for failure in failures:
            print(f" - {failure}")
        return 1

    print("\nALL OK")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

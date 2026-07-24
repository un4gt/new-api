#!/usr/bin/env python3
"""
Cloudflare Workers AI embeddings smoke test for new-api.

Calls the gateway using the OpenAI-compatible endpoint:
  POST /v1/embeddings

Usage:
  BASE_URL=http://127.0.0.1:3000 API_KEY=<TOKEN> \
    python3 bin/test_cloudflare_embeddings.py

BASE_URL may be either the gateway root or end in /v1. The public model IDs
use the cf/ namespace; the gateway maps them to Cloudflare's @cf/... IDs.

The batch checks below are synchronous OpenAI input[] requests. They do not
exercise Cloudflare's asynchronous queue-based Batch API.
"""

from __future__ import annotations

import os
import sys
from dataclasses import dataclass
from typing import Any, Dict, List, Optional, Tuple

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
        "API_KEY=xxx python3 bin/test_cloudflare_embeddings.py"
    )
if not API_KEY:
    raise SystemExit(
        "Missing env API_KEY. Example: BASE_URL=http://127.0.0.1:3000 "
        "API_KEY=xxx python3 bin/test_cloudflare_embeddings.py"
    )

API_BASE_URL = BASE_URL if BASE_URL.endswith("/v1") else f"{BASE_URL}/v1"
HEADERS = {
    "Authorization": f"Bearer {API_KEY}",
    "Content-Type": "application/json",
    "Accept": "application/json",
}


@dataclass(frozen=True)
class ModelCase:
    model: str
    test_batch: bool = False


MODEL_CASES = [
    ModelCase("cf/plamo-embedding-1b"),
    ModelCase("cf/embeddinggemma-300m"),
    ModelCase("cf/qwen-embedding-0.6b"),
    ModelCase("cf/bge-m3"),
    ModelCase("cf/bge-large-en-v1.5", test_batch=True),
    ModelCase("cf/bge-small-en-v1.5", test_batch=True),
    ModelCase("cf/bge-base-en-v1.5", test_batch=True),
]

BATCH_INPUTS = [
    "Cloudflare Workers AI provides serverless model inference.",
    "Embeddings map text into numerical vectors.",
    "Vector similarity is useful for semantic retrieval.",
]


def _post_embeddings(payload: Dict[str, Any]) -> Tuple[int, Optional[Dict[str, Any]], str]:
    response = requests.post(
        url=f"{API_BASE_URL}/embeddings",
        headers=HEADERS,
        json=payload,
        timeout=120,
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


def _is_error(status: int, data: Optional[Dict[str, Any]]) -> bool:
    return status != 200 or (isinstance(data, dict) and "error" in data)


def _preview(text: str, limit: int = 320) -> str:
    compact = (text or "").strip().replace("\n", " ")
    if len(compact) <= limit:
        return compact
    return compact[:limit] + f"...(truncated {len(compact)} chars)"


def _parse_embeddings(data: Any, expected_model: str, expected_items: int) -> int:
    _must(isinstance(data, dict), "response must be a JSON object")
    _must(data.get("object") == "list", f"unexpected object={data.get('object')!r}")
    _must(data.get("model") == expected_model, f"unexpected model={data.get('model')!r}")

    items = data.get("data")
    _must(
        isinstance(items, list) and len(items) == expected_items,
        f"expected {expected_items} embedding items, got "
        f"{len(items) if isinstance(items, list) else type(items).__name__}",
    )

    dimension = 0
    for index, item in enumerate(items):
        _must(isinstance(item, dict), f"data[{index}] must be an object")
        _must(item.get("object") == "embedding", f"unexpected data[{index}].object")
        _must(item.get("index") == index, f"unexpected data[{index}].index={item.get('index')!r}")
        embedding = item.get("embedding")
        _must(isinstance(embedding, list) and embedding, f"data[{index}].embedding must be non-empty")
        _must(
            all(isinstance(value, (int, float)) and not isinstance(value, bool) for value in embedding),
            f"data[{index}].embedding must contain only numbers",
        )
        if index == 0:
            dimension = len(embedding)
        _must(len(embedding) == dimension, f"embedding dimension mismatch at index={index}")

    usage = data.get("usage")
    _must(isinstance(usage, dict), "response.usage must be an object")
    prompt_tokens = usage.get("prompt_tokens")
    total_tokens = usage.get("total_tokens")
    _must(isinstance(prompt_tokens, int), "usage.prompt_tokens must be an integer")
    _must(isinstance(total_tokens, int), "usage.total_tokens must be an integer")
    _must(total_tokens >= prompt_tokens >= 0, "usage token counts are inconsistent")
    return dimension


def _test_single(model: str) -> None:
    status, data, text = _post_embeddings(
        {
            "model": model,
            "input": "A short sentence for the Cloudflare embedding smoke test.",
            "encoding_format": "float",
        }
    )
    _must(not _is_error(status, data), f"status={status}, body={_preview(text)}")
    dimension = _parse_embeddings(data, expected_model=model, expected_items=1)
    print(f"[OK] embeddings(single) model={model} dim={dimension}")


def _test_batch(model: str) -> None:
    status, data, text = _post_embeddings(
        {
            "model": model,
            "input": BATCH_INPUTS,
            "encoding_format": "float",
        }
    )
    _must(not _is_error(status, data), f"status={status}, body={_preview(text)}")
    dimension = _parse_embeddings(data, expected_model=model, expected_items=len(BATCH_INPUTS))
    print(f"[OK] embeddings(batch) model={model} dim={dimension} items={len(BATCH_INPUTS)}")


def main() -> None:
    print(f"Cloudflare embeddings endpoint: {API_BASE_URL}/embeddings")
    failures: List[str] = []

    for case in MODEL_CASES:
        try:
            _test_single(case.model)
        except requests.exceptions.RequestException as error:
            raise SystemExit(f"Cannot reach gateway at {API_BASE_URL}: {error}") from error
        except Exception as error:
            failures.append(f"single {case.model}: {error}")

        if case.test_batch:
            try:
                _test_batch(case.model)
            except requests.exceptions.RequestException as error:
                raise SystemExit(f"Cannot reach gateway at {API_BASE_URL}: {error}") from error
            except Exception as error:
                failures.append(f"batch {case.model}: {error}")

    if failures:
        print("\nFAILED:")
        for failure in failures:
            print(f" - {failure}")
        raise SystemExit(1)

    print("\nALL OK")


if __name__ == "__main__":
    main()

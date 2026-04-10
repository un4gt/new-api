#!/usr/bin/env python3
"""
ZeroEntropy embeddings smoke test for new-api.

Calls the gateway using OpenAI-style endpoint:
  POST /v1/embeddings

Usage:
  API_KEY=<YOUR_NEW_API_TOKEN> python3 bin/test_zeroentropy_embedding.py

Notes:
  - Base URL is fixed to http://127.0.0.1:3000/v1 (no extra env vars).
  - This script assumes model `zembed-1` is routed to the ZeroEntropy channel.
  - ZeroEntropy supports `dimensions` + `encoding_format` ("float" | "base64").
"""

from __future__ import annotations

import base64
import os
import struct
import sys
from typing import Any, Dict, Optional, Tuple

try:
    import requests  # type: ignore
except Exception:
    print("Missing dependency: requests (pip install requests)", file=sys.stderr)
    raise


BASE_URL = "http://127.0.0.1:3000/v1"
API_KEY = os.getenv("API_KEY")
if not API_KEY:
    raise SystemExit(
        "Missing env API_KEY. Example: API_KEY=xxx python3 bin/test_zeroentropy_embedding.py"
    )

HEADERS = {
    "Authorization": f"Bearer {API_KEY}",
    "Content-Type": "application/json",
    "Accept": "application/json",
}

MODEL = "zembed-1"


def _post(path: str, payload: Dict[str, Any]) -> Tuple[int, Optional[Dict[str, Any]], str]:
    resp = requests.post(url=f"{BASE_URL}{path}", headers=HEADERS, json=payload, timeout=120)
    text = resp.text or ""
    try:
        data = resp.json()
    except Exception:
        data = None
    return resp.status_code, data, text


def _is_error(status: int, data) -> bool:
    if status != 200:
        return True
    return isinstance(data, dict) and "error" in data


def _must(cond: bool, msg: str) -> None:
    if not cond:
        raise AssertionError(msg)


def _preview(text: str, limit: int = 240) -> str:
    s = (text or "").strip().replace("\n", " ")
    if len(s) <= limit:
        return s
    return s[:limit] + f"...(truncated {len(s)} chars)"


def _parse_float_embeddings(data, expected_model: str, expected_items: int, expected_dim: Optional[int]) -> int:
    _must(isinstance(data, dict), "embeddings response must be an object")
    _must(data.get("object") == "list", f"unexpected object={data.get('object')!r}")
    _must(
        isinstance(data.get("model"), str),
        f"unexpected model type={type(data.get('model'))}",
    )
    _must(data.get("model") == expected_model, f"unexpected model={data.get('model')!r}")

    items = data.get("data")
    _must(
        isinstance(items, list) and len(items) == expected_items,
        f"expected {expected_items} items, got {len(items) if isinstance(items, list) else type(items)}",
    )
    _must(isinstance(items[0], dict), "data[0] must be an object")
    emb0 = items[0].get("embedding")
    _must(isinstance(emb0, list) and len(emb0) > 0, "data[0].embedding must be a non-empty list")
    dim = len(emb0)
    if expected_dim is not None:
        _must(dim == expected_dim, f"expected dim={expected_dim}, got dim={dim}")
    for i, it in enumerate(items):
        _must(isinstance(it, dict), f"data[{i}] must be an object")
        emb = it.get("embedding")
        _must(isinstance(emb, list) and len(emb) == dim, f"embedding dim mismatch at index={i}")

    usage = data.get("usage")
    _must(isinstance(usage, dict), "embeddings.usage must be an object")
    _must(isinstance(usage.get("prompt_tokens"), int), "usage.prompt_tokens must be int")
    _must(isinstance(usage.get("total_tokens"), int), "usage.total_tokens must be int")
    return dim


def _parse_base64_embeddings(data, expected_model: str, expected_items: int, expected_dim: Optional[int]) -> int:
    _must(isinstance(data, dict), "embeddings response must be an object")
    _must(data.get("object") == "list", f"unexpected object={data.get('object')!r}")
    _must(data.get("model") == expected_model, f"unexpected model={data.get('model')!r}")

    items = data.get("data")
    _must(
        isinstance(items, list) and len(items) == expected_items,
        f"expected {expected_items} items, got {len(items) if isinstance(items, list) else type(items)}",
    )
    _must(isinstance(items[0], dict), "data[0] must be an object")
    emb0 = items[0].get("embedding")
    _must(isinstance(emb0, str) and emb0.strip(), "data[0].embedding must be a non-empty base64 string")

    raw0 = base64.b64decode(emb0)
    _must(len(raw0) % 4 == 0, "base64 embedding bytes must be divisible by 4 (fp32 array)")
    dim = len(raw0) // 4
    if expected_dim is not None:
        _must(dim == expected_dim, f"expected dim={expected_dim}, got dim={dim}")

    # Spot-check the first float decodes (to catch obvious non-fp32 payloads).
    _ = struct.unpack("<f", raw0[:4])[0]

    for i, it in enumerate(items):
        _must(isinstance(it, dict), f"data[{i}] must be an object")
        emb = it.get("embedding")
        _must(isinstance(emb, str) and emb.strip(), f"data[{i}].embedding must be a base64 string")
        raw = base64.b64decode(emb)
        _must(len(raw) == dim * 4, f"embedding dim mismatch at index={i}: bytes={len(raw)} dim={dim}")

    usage = data.get("usage")
    _must(isinstance(usage, dict), "embeddings.usage must be an object")
    _must(isinstance(usage.get("prompt_tokens"), int), "usage.prompt_tokens must be int")
    _must(isinstance(usage.get("total_tokens"), int), "usage.total_tokens must be int")
    return dim


def main() -> None:
    failures = []
    saw_success = False

    # 1) Query (single string) with explicit dimensions/float encoding.
    try:
        status, data, text = _post(
            "/embeddings",
            {
                "model": MODEL,
                "input": "What is RAG?",
                "dimensions": 320,
                "encoding_format": "float",
            },
        )
        _must(not _is_error(status, data), f"{MODEL} query(float) failed: {status} {_preview(text)}")
        dim = _parse_float_embeddings(data, expected_model=MODEL, expected_items=1, expected_dim=320)
        print(f"[OK] embeddings(query/float) model={MODEL} dim={dim}")
        saw_success = True
    except Exception as e:
        failures.append(f"embeddings {MODEL} query(float): {e}")

    # 2) Documents (batch list) default float encoding.
    docs = [
        "RAG combines retrieval with generation by conditioning the LLM on external documents.",
        "Retrieval-Augmented Generation improves factual accuracy by grounding answers.",
        "Transformers are a deep learning architecture.",
    ]
    try:
        status, data, text = _post(
            "/embeddings",
            {
                "model": MODEL,
                "input": docs,
                # dimensions omitted => default output dim (provider default)
            },
        )
        _must(not _is_error(status, data), f"{MODEL} documents(float-default) failed: {status} {_preview(text)}")
        dim = _parse_float_embeddings(data, expected_model=MODEL, expected_items=len(docs), expected_dim=None)
        print(f"[OK] embeddings(documents/float-default) model={MODEL} dim={dim} items={len(docs)}")
        saw_success = True
    except Exception as e:
        failures.append(f"embeddings {MODEL} documents(float-default): {e}")

    # 3) Query with base64 encoding + explicit dimensions.
    try:
        status, data, text = _post(
            "/embeddings",
            {
                "model": MODEL,
                "input": "base64 please",
                "dimensions": 80,
                "encoding_format": "base64",
            },
        )
        _must(not _is_error(status, data), f"{MODEL} query(base64) failed: {status} {_preview(text)}")
        dim = _parse_base64_embeddings(data, expected_model=MODEL, expected_items=1, expected_dim=80)
        print(f"[OK] embeddings(query/base64) model={MODEL} dim={dim}")
        saw_success = True
    except Exception as e:
        failures.append(f"embeddings {MODEL} query(base64): {e}")

    # Stable validation edge: missing input must be rejected by gateway.
    if saw_success:
        try:
            status, data, text = _post("/embeddings", {"model": MODEL})
            _must(_is_error(status, data), f"expected missing input to fail, got {status}: {_preview(text)}")
            print("[OK] embeddings(edge) missing input rejected")
        except Exception as e:
            failures.append(f"embeddings(edge) missing input: {e}")

    if failures:
        print("\nFAILED:")
        for f in failures:
            print(" - " + f)
        raise SystemExit(1)
    print("\nALL OK")


if __name__ == "__main__":
    main()


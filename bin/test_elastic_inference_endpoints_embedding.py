#!/usr/bin/env python3
"""
Elastic Inference Endpoints (EIS) embeddings smoke test for new-api.

Calls the gateway using OpenAI-style endpoint:
  POST /v1/embeddings

Usage:
  API_KEY=<YOUR_NEW_API_TOKEN> python3 bin/test_elastic_inference_endpoints_embedding.py

Notes:
  - Base URL is fixed to http://127.0.0.1:3000/v1 (no extra env vars).
  - This script assumes the listed models are routed to the Elastic Inference Endpoints channel.
"""

from __future__ import annotations

import json
import os
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
        "Missing env API_KEY. Example: API_KEY=xxx python3 bin/test_elastic_inference_endpoints_embedding.py"
    )

HEADERS = {
    "Authorization": f"Bearer {API_KEY}",
    "Content-Type": "application/json",
    "Accept": "application/json",
}


EMBEDDING_MODELS = [
    "jina-clip-v2",
    "jina-embeddings-v3",
    "jina-embeddings-v5",
    "jina-embeddings-v5-text-nano",
    "jina-embeddings-v5-text-small",
    "google-gemini-embedding-001",
    "openai-text-embedding-3-large",
    "openai-text-embedding-3-small",
]


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


def _get_openai_error(data: Optional[Dict[str, Any]]) -> Tuple[str, str, str]:
    if not isinstance(data, dict):
        return "", "", ""
    err = data.get("error")
    if not isinstance(err, dict):
        return "", "", ""
    msg = err.get("message")
    typ = err.get("type")
    code = err.get("code")
    return str(msg or ""), str(typ or ""), str(code or "")


def _is_elastic_base_url_empty(status: int, data: Optional[Dict[str, Any]]) -> bool:
    if status != 500:
        return False
    msg, _, _ = _get_openai_error(data)
    return "base url is empty" in (msg or "")


def _must(cond: bool, msg: str) -> None:
    if not cond:
        raise AssertionError(msg)


def _parse_embeddings(data, expected_model: str, expected_items: int) -> int:
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
    _must(isinstance(emb0, list) and len(emb0) > 0, "data[0].embedding must be a non-empty list")
    dim = len(emb0)
    for i, it in enumerate(items):
        _must(isinstance(it, dict), f"data[{i}] must be an object")
        emb = it.get("embedding")
        _must(isinstance(emb, list) and len(emb) == dim, f"embedding dim mismatch at index={i}")
    return dim


def _preview(text: str, limit: int = 240) -> str:
    s = (text or "").strip().replace("\n", " ")
    if len(s) <= limit:
        return s
    return s[:limit] + f"...(truncated {len(s)} chars)"


def _die_elastic_base_url_empty(model: str, status: int, data: Optional[Dict[str, Any]], text: str) -> None:
    msg, typ, code = _get_openai_error(data)
    print("\n[CONFIG ERROR] Elastic Inference Endpoints channel base_url is empty.")
    print(f"  - model: {model}")
    print(f"  - status: {status}")
    if msg:
        print(f"  - error.message: {msg}")
    if typ or code:
        print(f"  - error.type/code: {typ or '-'} / {code or '-'}")
    if text:
        print(f"  - raw: {_preview(text)}")
    print("\nFix:")
    print("  1) In new-api dashboard -> Channels -> Elastic Inference Endpoints, set Base URL")
    print("     Example: https://<cluster-id>.<region>.<provider>.cloud.es.io")
    print("  2) Make sure the channel is enabled and in the token's group routing.")
    print("  3) If you have multiple matching channels, disable the misconfigured ones.")
    print("     Admin-only tip: force a channel id via token suffix:")
    print("       API_KEY=<token>-<channel_id>  (new-api supports selecting specific_channel_id)")
    raise SystemExit(2)


def main() -> None:
    failures = []
    saw_success = False

    # Quick probe: if the EIS channel is selected but missing base_url, fail fast with guidance.
    try:
        probe_model = "google-gemini-embedding-001"
        status, data, text = _post("/embeddings", {"model": probe_model, "input": "probe"})
        if _is_elastic_base_url_empty(status, data):
            _die_elastic_base_url_empty(probe_model, status, data, text)
    except requests.exceptions.RequestException as e:
        raise SystemExit(f"Cannot reach gateway at {BASE_URL}: {e}") from e

    for model in EMBEDDING_MODELS:
        try:
            # Single input
            status, data, text = _post("/embeddings", {"model": model, "input": "hello elastic"})
            if _is_elastic_base_url_empty(status, data):
                _die_elastic_base_url_empty(model, status, data, text)
            _must(not _is_error(status, data), f"{model} embedding failed: {status} {_preview(text)}")
            dim = _parse_embeddings(data, model, expected_items=1)
            print(f"[OK] embeddings(text) model={model} dim={dim}")
            saw_success = True

            # Batch input
            status, data, text = _post("/embeddings", {"model": model, "input": ["text1", "text2", "text3"]})
            if _is_elastic_base_url_empty(status, data):
                _die_elastic_base_url_empty(model, status, data, text)
            _must(not _is_error(status, data), f"{model} batch embedding failed: {status} {_preview(text)}")
            dim = _parse_embeddings(data, model, expected_items=3)
            print(f"[OK] embeddings(batch) model={model} dim={dim} items=3")

            # Dimensions is not supported by EIS channel in new-api; it should be ignored (not forwarded upstream).
            if model == "google-gemini-embedding-001":
                status, data, text = _post("/embeddings", {"model": model, "input": "dims ignored", "dimensions": 8})
                if _is_elastic_base_url_empty(status, data):
                    _die_elastic_base_url_empty(model, status, data, text)
                _must(not _is_error(status, data), f"{model} dimensions test failed: {status} {_preview(text)}")
                dim = _parse_embeddings(data, model, expected_items=1)
                print(f"[OK] embeddings(dimensions-ignored) model={model} dim={dim}")
        except Exception as e:
            failures.append(f"embeddings {model}: {e}")

    # Stable validation edge: missing input must be rejected by gateway.
    if saw_success:
        try:
            status, data, text = _post("/embeddings", {"model": EMBEDDING_MODELS[0]})
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

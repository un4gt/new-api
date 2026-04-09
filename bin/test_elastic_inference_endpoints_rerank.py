#!/usr/bin/env python3
"""
Elastic Inference Endpoints (EIS) rerank smoke test for new-api.

Calls the gateway using OpenAI-style endpoint:
  POST /v1/rerank

Usage:
  API_KEY=<YOUR_NEW_API_TOKEN> python3 bin/test_elastic_inference_endpoints_rerank.py

Notes:
  - Base URL is fixed to http://127.0.0.1:3000/v1 (no extra env vars).
  - This script assumes the listed models are routed to the Elastic Inference Endpoints channel.
"""

from __future__ import annotations

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
    raise SystemExit("Missing env API_KEY. Example: API_KEY=xxx python3 bin/test_elastic_inference_endpoints_rerank.py")

HEADERS = {
    "Authorization": f"Bearer {API_KEY}",
    "Content-Type": "application/json",
    "Accept": "application/json",
}


RERANK_MODELS = [
    "jina-reranker-v2-base-multilingual",
    "jina-reranker-v3",
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


def _is_upstream_not_found(status: int, data: Optional[Dict[str, Any]]) -> bool:
    if status != 404:
        return False
    _, typ, code = _get_openai_error(data)
    return typ == "bad_response_status_code" or code == "bad_response_status_code"


def _must(cond: bool, msg: str) -> None:
    if not cond:
        raise AssertionError(msg)


def _preview(text: str, limit: int = 240) -> str:
    s = (text or "").strip().replace("\n", " ")
    if len(s) <= limit:
        return s
    return s[:limit] + f"...(truncated {len(s)} chars)"


def _parse_rerank_response(data, docs_len: int) -> None:
    _must(isinstance(data, dict), "rerank response must be an object")
    results = data.get("results")
    _must(isinstance(results, list) and len(results) > 0, "rerank.results must be a non-empty list")

    for i, item in enumerate(results):
        _must(isinstance(item, dict), f"results[{i}] must be an object")
        idx = item.get("index")
        score = item.get("relevance_score")
        _must(isinstance(idx, int), f"results[{i}].index must be int, got {type(idx)}")
        _must(isinstance(score, (int, float)), f"results[{i}].relevance_score must be number, got {type(score)}")
        _must(0 <= idx < docs_len, f"results[{i}].index out of range: {idx} (docs_len={docs_len})")

    usage = data.get("usage")
    _must(isinstance(usage, dict), "rerank.usage must be an object")
    _must(isinstance(usage.get("prompt_tokens"), int), "usage.prompt_tokens must be int")
    _must(isinstance(usage.get("total_tokens"), int), "usage.total_tokens must be int")


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

    docs = [
        "Organic skincare for sensitive skin with aloe vera and chamomile.",
        "New makeup trends focus on bold colors and innovative techniques.",
        "针对敏感肌专门设计的天然有机护肤产品，含芦荟与洋甘菊。",
        "新一季化妆趋势：大胆颜色与创新技巧。",
    ]
    query = "Organic skincare products for sensitive skin"

    # Quick probe: fail fast if EIS channel is selected but missing base_url.
    try:
        probe_model = "jina-reranker-v3"
        status, data, text = _post("/rerank", {"model": probe_model, "query": "probe", "documents": ["a", "b"]})
        if _is_elastic_base_url_empty(status, data):
            _die_elastic_base_url_empty(probe_model, status, data, text)
    except requests.exceptions.RequestException as e:
        raise SystemExit(f"Cannot reach gateway at {BASE_URL}: {e}") from e

    for model in RERANK_MODELS:
        try:
            status, data, text = _post(
                "/rerank",
                {
                    "model": model,
                    "query": query,
                    "top_n": 3,
                    "documents": docs,
                },
            )
            if _is_elastic_base_url_empty(status, data):
                _die_elastic_base_url_empty(model, status, data, text)
            _must(
                not _is_upstream_not_found(status, data),
                f"{model} rerank failed: upstream 404 (inference endpoint not found): {_preview(text)}",
            )
            _must(not _is_error(status, data), f"{model} rerank failed: {status} {_preview(text)}")
            _parse_rerank_response(data, docs_len=len(docs))
            print(f"[OK] rerank model={model} results={len(data.get('results') or [])}")
            saw_success = True

            # Print top 3
            for item in (data.get("results") or [])[:3]:
                if not isinstance(item, dict):
                    continue
                print(f"  - index={item.get('index')} score={item.get('relevance_score')}")
        except Exception as e:
            failures.append(f"rerank {model}: {e}")

    # Stable validation edge: missing documents should be rejected by gateway.
    if saw_success:
        try:
            status, data, text = _post("/rerank", {"model": RERANK_MODELS[0], "query": query})
            _must(_is_error(status, data), f"expected missing documents to fail, got {status}: {_preview(text)}")
            print("[OK] rerank(edge) missing documents rejected")
        except Exception as e:
            failures.append(f"rerank(edge) missing documents: {e}")

    if failures:
        print("\nFAILED:")
        for f in failures:
            print(" - " + f)
        raise SystemExit(1)
    print("\nALL OK")


if __name__ == "__main__":
    main()

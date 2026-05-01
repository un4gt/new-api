#!/usr/bin/env python3
"""
Xunfei Xingchen embeddings + rerank smoke test for new-api.

Usage:
  python3 bin/test_xunfei_xingchen_embedding_rerank.py NEW_API_KEY \
    --embedding-model sde0a5839 --rerank-model s125c8e0e
"""

from __future__ import annotations

import argparse
import sys
from typing import Any, Dict, Optional, Tuple

try:
    import requests  # type: ignore
except Exception:
    print("Missing dependency: requests (pip install requests)", file=sys.stderr)
    raise


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
    resp = requests.post(f"{base_url}{path}", headers=headers, json=payload, timeout=timeout)
    text = resp.text or ""
    try:
        data = resp.json()
    except Exception:
        data = None
    return resp.status_code, data, text


def _must(cond: bool, msg: str) -> None:
    if not cond:
        raise AssertionError(msg)


def _is_error(status: int, data: Optional[Dict[str, Any]]) -> bool:
    return status != 200 or (isinstance(data, dict) and "error" in data)


def _preview(text: str, limit: int = 240) -> str:
    text = (text or "").strip().replace("\n", " ")
    return text if len(text) <= limit else text[:limit] + f"...(truncated {len(text)} chars)"


def _check_embedding(data: Optional[Dict[str, Any]]) -> int:
    _must(isinstance(data, dict), "embedding response must be an object")
    items = data.get("data")
    _must(isinstance(items, list) and len(items) == 1, "embedding.data must contain one item")
    embedding = items[0].get("embedding") if isinstance(items[0], dict) else None
    _must(isinstance(embedding, list) and len(embedding) > 0, "embedding vector must be a non-empty list")
    usage = data.get("usage")
    _must(isinstance(usage, dict), "embedding.usage must be an object")
    _must(isinstance(usage.get("total_tokens"), int), "embedding usage.total_tokens must be int")
    return len(embedding)


def _check_rerank(data: Optional[Dict[str, Any]], docs_len: int) -> int:
    _must(isinstance(data, dict), "rerank response must be an object")
    results = data.get("results")
    _must(isinstance(results, list) and len(results) > 0, "rerank.results must be a non-empty list")
    for i, item in enumerate(results):
        _must(isinstance(item, dict), f"rerank.results[{i}] must be an object")
        idx = item.get("index")
        score = item.get("relevance_score")
        _must(isinstance(idx, int) and 0 <= idx < docs_len, f"invalid result index: {idx}")
        _must(isinstance(score, (int, float)), f"invalid relevance_score: {score}")
    usage = data.get("usage")
    _must(isinstance(usage, dict), "rerank.usage must be an object")
    _must(isinstance(usage.get("total_tokens"), int), "rerank usage.total_tokens must be int")
    return len(results)


def main() -> int:
    parser = argparse.ArgumentParser(description="new-api Xunfei Xingchen embeddings/rerank smoke test")
    parser.add_argument("NEW_API_KEY", help="new-api token")
    parser.add_argument("--base-url", default="http://127.0.0.1:3000/v1", help="Gateway base URL with /v1")
    parser.add_argument("--embedding-model", default="sde0a5839", help="Embedding model/service ID")
    parser.add_argument("--rerank-model", default="s125c8e0e", help="Rerank model/service ID")
    parser.add_argument("--timeout", type=float, default=120.0, help="Request timeout seconds")
    args = parser.parse_args()

    base_url = args.base_url.rstrip("/")
    failures = []

    try:
        status, data, text = _post(
            base_url,
            args.NEW_API_KEY,
            "/embeddings",
            {"model": args.embedding_model, "input": "hello xunfei xingchen", "encoding_format": "float"},
            args.timeout,
        )
        _must(not _is_error(status, data), f"embedding failed: {status} {_preview(text)}")
        dim = _check_embedding(data)
        print(f"[OK] embeddings model={args.embedding_model} dim={dim}")
    except Exception as e:
        failures.append(f"embeddings: {e}")

    docs = [
        "RAG combines retrieval with generation.",
        "The weather is sunny today.",
        "Retrieval can ground answers in external documents.",
    ]
    try:
        status, data, text = _post(
            base_url,
            args.NEW_API_KEY,
            "/rerank",
            {"model": args.rerank_model, "query": "What is RAG?", "documents": docs},
            args.timeout,
        )
        _must(not _is_error(status, data), f"rerank failed: {status} {_preview(text)}")
        count = _check_rerank(data, len(docs))
        print(f"[OK] rerank model={args.rerank_model} results={count}")
    except Exception as e:
        failures.append(f"rerank: {e}")

    if failures:
        print("\nFAILED:")
        for failure in failures:
            print(f" - {failure}")
        return 1

    print("\nALL OK")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

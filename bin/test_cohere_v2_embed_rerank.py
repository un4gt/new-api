#!/usr/bin/env python3
"""
Cohere v2 embed + rerank smoke test for new-api.

Usage:
  API_KEY=<YOUR_NEW_API_TOKEN> python3 bin/test_cohere_v2_embed_rerank.py
"""

from __future__ import annotations

import os
import sys

try:
    import requests  # type: ignore
except Exception:
    print("Missing dependency: requests (pip install requests)", file=sys.stderr)
    raise


BASE_URL = "http://127.0.0.1:3000"
API_KEY = os.getenv("API_KEY")
if not API_KEY:
    raise SystemExit(
        "Missing env API_KEY. Example: API_KEY=xxx python3 bin/test_cohere_v2_embed_rerank.py"
    )

HEADERS = {
    "Authorization": f"Bearer {API_KEY}",
    "Content-Type": "application/json",
    "Accept": "application/json",
}

EMBED_MODEL = "embed-v4.0"
RERANK_MODEL = "rerank-v3.5"
DOCS = [
    "Washington, D.C. is the capital of the United States.",
    "Paris is the capital of France.",
    "Ottawa is the capital of Canada.",
]
QUERY = "What is the capital of the United States?"


def preview(text, limit=240):
    text = (text or "").strip().replace("\n", " ")
    return text if len(text) <= limit else text[:limit] + f"...(truncated {len(text)} chars)"


def must(ok, message):
    if not ok:
        raise AssertionError(message)


def post(path, body, expect_ok=True):
    resp = requests.post(f"{BASE_URL}{path}", headers=HEADERS, json=body, timeout=120)
    try:
        data = resp.json()
    except Exception:
        data = None

    is_error = resp.status_code != 200 or (isinstance(data, dict) and "error" in data)
    if expect_ok:
        must(not is_error, f"{path} failed: {resp.status_code} {preview(resp.text)}")
        must(isinstance(data, dict), f"{path} response must be a JSON object")
    else:
        must(is_error, f"{path} should fail, got: {resp.status_code} {preview(resp.text)}")
    return data, resp.text


def native_embed_dim(data):
    embeddings = data.get("embeddings")
    must(isinstance(embeddings, dict), "native embed response missing embeddings object")
    vectors = embeddings.get("float") or embeddings.get("float_")
    must(isinstance(vectors, list) and len(vectors) == 1, "native embed response missing float vector")
    must(isinstance(vectors[0], list) and len(vectors[0]) > 0, "native embed vector is empty")
    return len(vectors[0])


def v1_embed_dim(data):
    items = data.get("data")
    must(isinstance(items, list) and len(items) == 1, "v1 embeddings response missing data[0]")
    vector = items[0].get("embedding") if isinstance(items[0], dict) else None
    must(isinstance(vector, list) and len(vector) > 0, "v1 embedding vector is empty")
    return len(vector)


def rerank_count(data):
    results = data.get("results")
    must(isinstance(results, list) and len(results) > 0, "rerank response missing results")
    for item in results:
        must(isinstance(item.get("index"), int), "rerank result.index must be int")
        must(isinstance(item.get("relevance_score"), (int, float)), "rerank result.relevance_score must be number")
    return len(results)


def main():
    data, _ = post(
        "/v2/embed",
        {
            "model": EMBED_MODEL,
            "input_type": "search_document",
            "texts": ["hello cohere"],
            "embedding_types": ["float"],
        },
    )
    print(f"[OK] /v2/embed dim={native_embed_dim(data)}")

    data, _ = post(
        "/v2/rerank",
        {
            "model": RERANK_MODEL,
            "query": QUERY,
            "documents": DOCS,
            "top_n": 2,
        },
    )
    print(f"[OK] /v2/rerank results={rerank_count(data)}")

    data, _ = post(
        "/v1/embeddings",
        {
            "model": EMBED_MODEL,
            "input": ["hello cohere"],
            "input_type": "search_document",
            "embedding_types": ["float"],
        },
    )
    print(f"[OK] /v1/embeddings dim={v1_embed_dim(data)}")

    data, _ = post(
        "/v1/rerank",
        {
            "model": RERANK_MODEL,
            "query": QUERY,
            "documents": DOCS,
            "top_n": 2,
        },
    )
    print(f"[OK] /v1/rerank results={rerank_count(data)}")

    _, text = post(
        "/v2/rerank",
        {
            "model": RERANK_MODEL,
            "query": QUERY,
            "documents": DOCS,
            "return_documents": True,
        },
        expect_ok=False,
    )
    must("无效的数据格式" in text, f"/v2/rerank missing invalid data message: {preview(text)}")
    print("[OK] /v2/rerank rejects v1-only field")

    print("\nALL OK")


if __name__ == "__main__":
    main()

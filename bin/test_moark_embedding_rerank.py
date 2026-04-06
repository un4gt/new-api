#!/usr/bin/env python3
"""
Moark (模力方舟 / ai.gitee.com) embeddings + rerank smoke tests for new-api.

Goal:
  - Keep it minimal (no CLI args, no extra env vars).
  - Cover the common success paths + key edge cases.
  - Distinguish text-only embeddings vs multimodal models.

Docs:
  https://ai.gitee.com/serverless-api#embedding-rerank

How to run:
  APIKEY=<YOUR_NEW_API_TOKEN> python3 bin/test_moark_embedding_rerank.py
"""

from __future__ import annotations

import json
import os
import sys

try:
    import requests  # type: ignore
except Exception:
    print("Missing dependency: requests (pip install requests)", file=sys.stderr)
    raise


BASE_URL = "http://localhost:3000/v1"
APIKEY = os.getenv("APIKEY")
if not APIKEY:
    raise SystemExit("Missing env APIKEY. Example: APIKEY=xxx python3 bin/test_moark_embedding_rerank.py")

HEADERS = {
    "Authorization": f"Bearer {APIKEY}",
    "Content-Type": "application/json",
}

# Tiny 1x1 PNG (transparent). Use a data URL to avoid any external network dependency.
_PNG_BASE64 = (
    "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/xcAAn8B9pYy3QAAAABJRU5ErkJggg=="
)
IMAGE_DATA_URL = "data:image/png;base64," + _PNG_BASE64
# Also keep the raw base64 variant (some upstreams accept this form).
IMAGE_BASE64 = _PNG_BASE64


TEXT_EMBED_MODELS = ["nomic-embed-code"]

MULTIMODAL_EMBED_MODELS = [
    "Qwen3-VL-Embedding-2B",
    "Qwen3-VL-Embedding-8B",
    "jina-clip-v1",
    "jina-clip-v2",
    "jina-embeddings-v4",
]

MULTIMODAL_RERANK_MODELS = [
    "Qwen3-VL-Reranker-2B",
    "Qwen3-VL-Reranker-8B",
    "jina-reranker-m0",
]


def _post(path: str, payload: dict):
    resp = requests.post(
        url=f"{BASE_URL}{path}",
        headers=HEADERS,
        data=json.dumps(payload),
        timeout=120,
    )
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


def _doc_to_text(doc) -> str:
    if doc is None:
        return ""
    if isinstance(doc, str):
        return doc
    if isinstance(doc, dict):
        if isinstance(doc.get("text"), str):
            return doc["text"]
        if isinstance(doc.get("image"), str):
            # Keep it compact (avoid printing huge base64).
            return "<image>"
    return str(doc)


def _parse_rerank_multimodal(data, want_documents: bool) -> None:
    _must(isinstance(data, list) and len(data) > 0, "rerank/multimodal response must be a non-empty list")
    for item in data:
        _must(isinstance(item, dict), "result item must be an object")
        _must(isinstance(item.get("index"), int), "result.index must be int")
        _must(isinstance(item.get("score"), (int, float)), "result.score must be number")
        if want_documents:
            _must("document" in item, "result.document must exist when return_documents=true")


def _print_rerank_multimodal_results(results) -> None:
    for item in results:
        if not isinstance(item, dict):
            print(f"Index: ?, Score: ?\n  Document: {str(item)[:120]}")
            continue
        idx = item.get("index")
        score = item.get("score")
        doc_text = _doc_to_text(item.get("document"))
        print(f"Index: {idx}, Score: {score}")
        if doc_text:
            print(f"  Document: {doc_text}")


def main() -> None:
    failures = []

    # ── Embeddings ────────────────────────────────────────────────────
    # 1) Text-only embeddings: single input + batch input
    for model in TEXT_EMBED_MODELS:
        try:
            status, data, text = _post(
                "/embeddings",
                {"model": model, "input": "hello moark"},
            )
            _must(not _is_error(status, data), f"{model} text embedding failed: {status} {_preview(text)}")
            dim = _parse_embeddings(data, model, expected_items=1)
            print(f"[OK] embeddings(text) model={model} dim={dim}")

            status, data, text = _post(
                "/embeddings",
                {"model": model, "input": ["a", "b"]},
            )
            _must(not _is_error(status, data), f"{model} batch embedding failed: {status} {_preview(text)}")
            dim = _parse_embeddings(data, model, expected_items=2)
            print(f"[OK] embeddings(text-batch) model={model} dim={dim} items=2")
        except Exception as e:
            failures.append(f"embeddings(text) {model}: {e}")

    # 2) Multimodal embeddings: text-only + mixed text+image inputs
    for model in MULTIMODAL_EMBED_MODELS:
        try:
            status, data, text = _post(
                "/embeddings",
                {"model": model, "input": "hello moark"},
            )
            _must(not _is_error(status, data), f"{model} embedding(text) failed: {status} {_preview(text)}")
            dim = _parse_embeddings(data, model, expected_items=1)
            print(f"[OK] embeddings(text) model={model} dim={dim}")

            status, data, text = _post(
                "/embeddings",
                {"model": model, "input": ["a", "b"]},
            )
            _must(not _is_error(status, data), f"{model} embedding(text-batch) failed: {status} {_preview(text)}")
            dim = _parse_embeddings(data, model, expected_items=2)
            print(f"[OK] embeddings(text-batch) model={model} dim={dim} items=2")

            status, data, text = _post(
                "/embeddings",
                {
                    "model": model,
                    "input": [
                        {"text": "a blue cat"},
                        {"image": IMAGE_BASE64},
                    ],
                },
            )
            if _is_error(status, data):
                # Some upstreams expect a data URL instead of raw base64.
                status, data, text = _post(
                    "/embeddings",
                    {
                        "model": model,
                        "input": [
                            {"text": "a blue cat"},
                            {"image": IMAGE_DATA_URL},
                        ],
                    },
                )
            _must(not _is_error(status, data), f"{model} embeddings(multimodal) failed: {status} {_preview(text)}")
            dim = _parse_embeddings(data, model, expected_items=2)
            print(f"[OK] embeddings(multimodal) model={model} dim={dim} items=2")
        except Exception as e:
            failures.append(f"embeddings(multimodal) {model}: {e}")

    # Edge case: missing input (should be rejected by gateway validation)
    try:
        status, data, text = _post("/embeddings", {"model": TEXT_EMBED_MODELS[0]})
        _must(_is_error(status, data), f"expected embeddings missing-input to fail, got {status}: {_preview(text)}")
        print("[OK] embeddings(edge) missing input rejected")
    except Exception as e:
        failures.append(f"embeddings(edge) missing input: {e}")

    # Edge case: explicit null input should be rejected by gateway validation too.
    try:
        status, data, text = _post("/embeddings", {"model": TEXT_EMBED_MODELS[0], "input": None})
        _must(_is_error(status, data), f"expected embeddings null-input to fail, got {status}: {_preview(text)}")
        print("[OK] embeddings(edge) null input rejected")
    except Exception as e:
        failures.append(f"embeddings(edge) null input: {e}")

    # ── Rerank (multimodal) ───────────────────────────────────────────
    docs_text = [
        {"text": "Paris is the capital of France."},
        {"text": "London is the capital of England."},
        {"text": "Berlin is the capital of Germany."},
    ]
    for model in MULTIMODAL_RERANK_MODELS:
        try:
            status, data, text = _post(
                "/rerank/multimodal",
                {
                    "model": model,
                    "query": {"text": "What is the capital of France?"},
                    "documents": docs_text,
                    "return_documents": True,
                },
            )
            _must(not _is_error(status, data), f"{model} rerank/multimodal failed: {status} {_preview(text)}")
            _parse_rerank_multimodal(data, want_documents=True)
            print(f"[OK] rerank/multimodal model={model} return_documents=true")
            _print_rerank_multimodal_results(data)
        except Exception as e:
            failures.append(f"rerank/multimodal {model}: {e}")

    # Multimodal coverage: include at least one image document.
    try:
        status, data, text = _post(
            "/rerank/multimodal",
            {
                "model": MULTIMODAL_RERANK_MODELS[0],
                "query": {"text": "red square"},
                "documents": [
                    {"text": "this is a red square"},
                    {"image": IMAGE_BASE64},
                ],
                "return_documents": True,
            },
        )
        if _is_error(status, data):
            # Fallback: some upstreams want a data URL.
            status, data, text = _post(
                "/rerank/multimodal",
                {
                    "model": MULTIMODAL_RERANK_MODELS[0],
                    "query": {"text": "red square"},
                    "documents": [
                        {"text": "this is a red square"},
                        {"image": IMAGE_DATA_URL},
                    ],
                    "return_documents": True,
                },
            )
        _must(not _is_error(status, data), f"rerank/multimodal image-doc failed: {status} {_preview(text)}")
        _parse_rerank_multimodal(data, want_documents=True)
        print("[OK] rerank/multimodal(edge) image document accepted")
    except Exception as e:
        failures.append(f"rerank/multimodal(edge) image document: {e}")

    # Boundary: documents == 25 is allowed.
    try:
        status, data, text = _post(
            "/rerank/multimodal",
            {
                "model": MULTIMODAL_RERANK_MODELS[0],
                "query": {"text": "x"},
                "documents": [{"text": f"d{i}"} for i in range(25)],
            },
        )
        _must(not _is_error(status, data), f"expected 25 documents to succeed, got {status}: {_preview(text)}")
        _parse_rerank_multimodal(data, want_documents=False)
        print("[OK] rerank/multimodal(edge) documents == 25 accepted")
    except Exception as e:
        failures.append(f"rerank/multimodal(edge) documents == 25: {e}")

    # Gateway validation edges (stable)
    try:
        status, data, text = _post("/rerank/multimodal", {"documents": docs_text, "query": {"text": "x"}})
        _must(_is_error(status, data), f"expected missing model to fail, got {status}: {_preview(text)}")
        print("[OK] rerank/multimodal(edge) missing model rejected")
    except Exception as e:
        failures.append(f"rerank/multimodal(edge) missing model: {e}")

    try:
        status, data, text = _post(
            "/rerank/multimodal",
            {"model": MULTIMODAL_RERANK_MODELS[0], "query": {"text": "x"}, "documents": []},
        )
        _must(_is_error(status, data), f"expected empty documents to fail, got {status}: {_preview(text)}")
        print("[OK] rerank/multimodal(edge) empty documents rejected")
    except Exception as e:
        failures.append(f"rerank/multimodal(edge) empty documents: {e}")

    try:
        status, data, text = _post(
            "/rerank/multimodal",
            {"model": MULTIMODAL_RERANK_MODELS[0], "query": {}, "documents": [{"text": "a"}]},
        )
        _must(_is_error(status, data), f"expected empty query item to fail, got {status}: {_preview(text)}")
        print("[OK] rerank/multimodal(edge) empty query item rejected")
    except Exception as e:
        failures.append(f"rerank/multimodal(edge) empty query item: {e}")

    try:
        status, data, text = _post(
            "/rerank/multimodal",
            {
                "model": MULTIMODAL_RERANK_MODELS[0],
                "query": {"text": "x", "image": IMAGE_DATA_URL},
                "documents": [{"text": "a"}],
            },
        )
        _must(_is_error(status, data), f"expected query one-of violation to fail, got {status}: {_preview(text)}")
        print("[OK] rerank/multimodal(edge) query text+image rejected")
    except Exception as e:
        failures.append(f"rerank/multimodal(edge) query text+image: {e}")

    try:
        status, data, text = _post(
            "/rerank/multimodal",
            {
                "model": MULTIMODAL_RERANK_MODELS[0],
                "query": {"text": "x"},
                "documents": [{"text": "a", "image": IMAGE_DATA_URL}],
            },
        )
        _must(_is_error(status, data), f"expected document one-of violation to fail, got {status}: {_preview(text)}")
        print("[OK] rerank/multimodal(edge) document text+image rejected")
    except Exception as e:
        failures.append(f"rerank/multimodal(edge) document text+image: {e}")

    try:
        status, data, text = _post(
            "/rerank/multimodal",
            {
                "model": MULTIMODAL_RERANK_MODELS[0],
                "query": {"text": "x"},
                "documents": [{}],
            },
        )
        _must(_is_error(status, data), f"expected empty document item to fail, got {status}: {_preview(text)}")
        print("[OK] rerank/multimodal(edge) empty document item rejected")
    except Exception as e:
        failures.append(f"rerank/multimodal(edge) empty document item: {e}")

    try:
        status, data, text = _post(
            "/rerank/multimodal",
            {
                "model": MULTIMODAL_RERANK_MODELS[0],
                "query": {"text": "x"},
                "documents": [{"text": f"d{i}"} for i in range(26)],
            },
        )
        _must(_is_error(status, data), f"expected >25 documents to fail, got {status}: {_preview(text)}")
        print("[OK] rerank/multimodal(edge) documents >25 rejected")
    except Exception as e:
        failures.append(f"rerank/multimodal(edge) documents >25: {e}")

    if failures:
        print("\nFAILED:")
        for f in failures:
            print(" - " + f)
        raise SystemExit(1)
    print("\nALL OK")


def _preview(text: str, limit: int = 240) -> str:
    s = (text or "").strip().replace("\n", " ")
    if len(s) <= limit:
        return s
    return s[:limit] + f"...(truncated {len(s)} chars)"


if __name__ == "__main__":
    main()

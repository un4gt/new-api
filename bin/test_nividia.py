#!/usr/bin/env python3
"""
Nvidia embeddings smoke test for new-api.

Covers:
1) text embedding model via /v1/embeddings (OpenAI-compatible)
2) nv-dinov2 small-image branch (<200KB inline base64)
3) nv-dinov2 large-image branch (>=200KB, gateway should upload to NVCF assets)

Usage:
  NEW_API_API_KEY=<TOKEN> python3 bin/test_nividia.py
"""

from __future__ import annotations

import argparse
import base64
import binascii
import json
import os
import struct
import sys
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple

try:
    import requests  # type: ignore
except Exception:
    print("Missing dependency: requests. Install with: pip install requests", file=sys.stderr)
    raise


@dataclass
class TestResult:
    name: str
    ok: bool
    seconds: float
    detail: str = ""
    skipped: bool = False


class SkipTest(Exception):
    pass


def _skip(msg: str) -> None:
    raise SkipTest(msg)


def _normalize_base_url(base_url: str) -> str:
    base_url = base_url.rstrip("/")
    # Accept both "https://.../v1" and "https://..." for convenience.
    if base_url.endswith("/v1"):
        base_url = base_url[:-3]
    return base_url.rstrip("/")


def _auth_headers(api_key: str) -> Dict[str, str]:
    return {
        "Authorization": f"Bearer {api_key}",
        "Content-Type": "application/json",
        "Accept": "application/json",
    }


def _must(cond: bool, msg: str) -> None:
    if not cond:
        raise AssertionError(msg)


def _json_preview(text: str, limit: int = 1200) -> str:
    s = text.strip()
    if len(s) <= limit:
        return s
    return s[:limit] + f"...(truncated, {len(s)} chars)"


def _request_json(
    session: requests.Session,
    method: str,
    url: str,
    headers: Dict[str, str],
    json_body: Optional[Dict[str, Any]],
    timeout: float,
    verify_tls: bool,
) -> Tuple[int, Optional[Dict[str, Any]], str, float]:
    t0 = time.time()
    resp = session.request(
        method=method,
        url=url,
        headers=headers,
        json=json_body,
        timeout=timeout,
        verify=verify_tls,
    )
    dt = time.time() - t0
    text = resp.text or ""
    try:
        data = resp.json()
    except Exception:
        data = None
    return resp.status_code, data, text, dt


def _normalize_model_for_assert(model: str) -> str:
    m = (model or "").strip().lower()
    if not m:
        return ""

    # Accept short aliases (new-api Nvidia channel supports them).
    if "/" not in m:
        if m == "bge-m3":
            m = "baai/" + m
        else:
            m = "nvidia/" + m

    # Accept legacy underscore variant (some docs/clients use "3_2" instead of "3.2").
    m = m.replace("llama-3_2-", "llama-3.2-")
    return m


def _parse_openai_embeddings_response(data: Dict[str, Any], expected_model: str) -> Tuple[int, int]:
    _must(isinstance(data, dict), "response is not JSON object")
    _must(data.get("object") == "list", f"unexpected object={data.get('object')!r}")
    resp_model = data.get("model")
    _must(isinstance(resp_model, str) and resp_model.strip(), "response.model must be non-empty string")
    if _normalize_model_for_assert(resp_model) != _normalize_model_for_assert(expected_model):
        raise AssertionError(f"unexpected model={resp_model!r} (expected {expected_model!r})")

    items = data.get("data")
    _must(isinstance(items, list) and len(items) > 0, "response.data must be non-empty list")

    first = items[0]
    _must(isinstance(first, dict), "response.data[0] must be object")
    emb = first.get("embedding")
    _must(isinstance(emb, list) and len(emb) > 0, "response.data[0].embedding must be non-empty list")
    dim = len(emb)

    for i, item in enumerate(items):
        _must(isinstance(item, dict), f"response.data[{i}] must be object")
        e = item.get("embedding")
        _must(isinstance(e, list) and len(e) == dim, f"embedding dims mismatch at index={i}")

    return dim, len(items)


def _png_chunk(chunk_type: bytes, data: bytes) -> bytes:
    crc = binascii.crc32(chunk_type + data) & 0xFFFFFFFF
    return struct.pack(">I", len(data)) + chunk_type + data + struct.pack(">I", crc)


def _make_png_bytes(width: int, height: int, randomize: bool) -> bytes:
    if width <= 0:
        width = 8
    if height <= 0:
        height = 8

    sig = b"\x89PNG\r\n\x1a\n"
    ihdr = struct.pack(">IIBBBBB", width, height, 8, 2, 0, 0, 0)  # 8-bit RGB

    rows: List[bytes] = []
    row_pixels = width * 3
    for y in range(height):
        if randomize:
            pixels = os.urandom(row_pixels)
        else:
            # Small deterministic gradient image.
            r = (y * 13) % 256
            g = (80 + y * 7) % 256
            b = (180 + y * 3) % 256
            pixels = bytes([r, g, b]) * width
        rows.append(bytes([0]) + pixels)  # filter=0

    raw = b"".join(rows)
    level = 0 if randomize else 9
    idat = __import__("zlib").compress(raw, level)
    return sig + _png_chunk(b"IHDR", ihdr) + _png_chunk(b"IDAT", idat) + _png_chunk(b"IEND", b"")


def _guess_mime_from_path(path: Path) -> str:
    ext = path.suffix.lower()
    if ext in {".jpg", ".jpeg"}:
        return "image/jpeg"
    if ext == ".png":
        return "image/png"
    if ext == ".webp":
        return "image/webp"
    if ext == ".bmp":
        return "image/bmp"
    return "application/octet-stream"


def _to_data_url(content: bytes, mime_type: str) -> str:
    b64 = base64.b64encode(content).decode("utf-8")
    return f"data:{mime_type};base64,{b64}"


def _run_test(name: str, fn) -> TestResult:
    t0 = time.time()
    try:
        fn()
        return TestResult(name=name, ok=True, seconds=time.time() - t0)
    except SkipTest as e:
        return TestResult(name=name, ok=True, seconds=time.time() - t0, detail=str(e), skipped=True)
    except Exception as e:
        return TestResult(name=name, ok=False, seconds=time.time() - t0, detail=str(e))


def main() -> int:
    parser = argparse.ArgumentParser(description="new-api Nvidia embeddings smoke test")
    parser.add_argument(
        "--base-url",
        default="http://localhost:3000",
        help="Gateway base URL (no /v1 suffix). Example: http://localhost:3000 or https://integrate.api.nvidia.com",
    )
    parser.add_argument(
        "--api-key",
        default=os.environ.get("NEW_API_API_KEY", ""),
        help="API key (env: NEW_API_API_KEY)",
    )
    parser.add_argument("--timeout", type=float, default=180.0, help="Request timeout seconds")
    parser.add_argument("--insecure", action="store_true", help="Disable TLS verification")
    parser.add_argument("--verbose", action="store_true", help="Print request/response previews")

    parser.add_argument(
        "--text-models",
        default=",".join(
            [
                "nvidia/llama-nemotron-embed-1b-v2",
                "nvidia/llama-3.2-nemoretriever-300m-embed-v2",
                "nvidia/llama-3.2-nemoretriever-300m-embed-v1",
                "nvidia/nv-embed-v1",
                "baai/bge-m3",
            ]
        ),
        help="Comma-separated Nvidia text embedding models to test (must match the Nvidia channel whitelist)",
    )
    parser.add_argument(
        "--text-input",
        default="hello nvidia embedding",
        help="Input text for text embedding test",
    )
    parser.add_argument(
        "--input-type",
        default="query",
        choices=["query", "passage"],
        help="NVIDIA embedding input_type (query or passage). Many NVIDIA models require this field.",
    )
    parser.add_argument(
        "--truncate",
        default="NONE",
        choices=["NONE", "START", "END"],
        help="NVIDIA embedding truncate strategy (see NVIDIA docs).",
    )
    parser.add_argument(
        "--encoding-format",
        default="float",
        choices=["float", "base64"],
        help="Embedding encoding_format (OpenAI-compatible).",
    )
    parser.add_argument(
        "--skip-openai-client",
        action="store_true",
        help="Skip OpenAI Python client test (requests-based tests still run).",
    )
    parser.add_argument(
        "--dinov2-model",
        default="nvidia/nv-dinov2",
        help="Model name for image embedding test",
    )
    parser.add_argument(
        "--large-image-file",
        default="",
        help="Optional local image file for large-image branch (>=200KB). If omitted, script generates one.",
    )
    parser.add_argument(
        "--skip-large",
        action="store_true",
        help="Skip large-image branch test",
    )
    parser.add_argument(
        "--skip-dinov2",
        action="store_true",
        help="Skip nv-dinov2 tests (useful when targeting upstream /v1/embeddings only).",
    )

    args = parser.parse_args()

    api_key = args.api_key.strip()
    if not api_key:
        print("Missing --api-key (or env NEW_API_API_KEY).", file=sys.stderr)
        return 2

    base_url = _normalize_base_url(args.base_url)
    verify_tls = not args.insecure
    headers = _auth_headers(api_key)
    session = requests.Session()

    openai_client_cache: List[Any] = []

    def _get_openai_client():
        if args.skip_openai_client:
            _skip("--skip-openai-client")
        if openai_client_cache:
            return openai_client_cache[0]

        try:
            from openai import DefaultHttpxClient, OpenAI  # type: ignore
        except Exception as e:
            raise AssertionError(
                f"Missing dependency: openai ({e}). Install with: pip install openai"
            ) from e

        openai_base_url = f"{base_url}/v1"
        http_client = DefaultHttpxClient(verify=verify_tls)
        client = OpenAI(
            base_url=openai_base_url,
            api_key=api_key,
            timeout=args.timeout,
            http_client=http_client,
        )
        openai_client_cache.append(client)
        return client

    def call_embeddings(model: str, input_value: Any) -> Tuple[int, Optional[Dict[str, Any]], str, float]:
        payload = {
            "model": model,
            "input": input_value,
        }
        # Match NVIDIA official examples: include input_type/truncate for text embeddings.
        if model != args.dinov2_model:
            payload["input_type"] = args.input_type
            payload["truncate"] = args.truncate
            payload["encoding_format"] = args.encoding_format
        url = f"{base_url}/v1/embeddings"
        status, data, text, dt = _request_json(
            session=session,
            method="POST",
            url=url,
            headers=headers,
            json_body=payload,
            timeout=args.timeout,
            verify_tls=verify_tls,
        )
        if args.verbose:
            print(f"\n[POST] /v1/embeddings ({model}) -> {status} ({dt:.2f}s)")
            preview = json.dumps(payload, ensure_ascii=False)
            print(f"request: {_json_preview(preview, 800)}")
            print(f"response: {_json_preview(text, 800)}")
        return status, data, text, dt

    results: List[TestResult] = []

    def test_text_embedding() -> None:
        text_models = [m.strip() for m in (args.text_models or "").split(",") if m.strip()]
        _must(len(text_models) > 0, "--text-models must not be empty")

        for model in text_models:
            status, data, text, _ = call_embeddings(model, args.text_input)
            _must(status == 200, f"text embedding failed: model={model}, status={status}, body={_json_preview(text)}")
            dim, items = _parse_openai_embeddings_response(data or {}, expected_model=model)
            _must(items == 1, f"text embedding expected 1 item, got {items} (model={model})")
            _must(dim > 0, f"text embedding dimension is 0 (model={model})")

    def test_text_embedding_openai_client() -> None:
        client = _get_openai_client()

        text_models = [m.strip() for m in (args.text_models or "").split(",") if m.strip()]
        _must(len(text_models) > 0, "--text-models must not be empty")

        for model in text_models:
            # NVIDIA-specific parameters are passed via extra_body to keep OpenAI client compatibility.
            resp = client.embeddings.create(
                model=model,
                input=args.text_input,
                encoding_format=args.encoding_format,
                extra_body={
                    "input_type": args.input_type,
                    "truncate": args.truncate,
                },
            )
            data = resp.model_dump() if hasattr(resp, "model_dump") else resp  # type: ignore[assignment]
            dim, items = _parse_openai_embeddings_response(data or {}, expected_model=model)
            _must(items == 1, f"openai client embedding expected 1 item, got {items} (model={model})")
            _must(dim > 0, f"openai client embedding dimension is 0 (model={model})")

    def test_dinov2_small_image() -> None:
        small_png = _make_png_bytes(width=16, height=16, randomize=False)
        _must(len(small_png) < 200 * 1024, f"small image is not <200KB, size={len(small_png)}")
        small_data_url = _to_data_url(small_png, "image/png")

        status, data, text, _ = call_embeddings(args.dinov2_model, small_data_url)
        _must(status == 200, f"nv-dinov2 small-image failed: status={status}, body={_json_preview(text)}")
        dim, items = _parse_openai_embeddings_response(data or {}, expected_model=args.dinov2_model)
        _must(items >= 1, "nv-dinov2 small-image returned empty embeddings")
        _must(dim > 0, "nv-dinov2 small-image embedding dimension is 0")

    def test_dinov2_small_image_openai_client() -> None:
        client = _get_openai_client()

        small_png = _make_png_bytes(width=16, height=16, randomize=False)
        _must(len(small_png) < 200 * 1024, f"small image is not <200KB, size={len(small_png)}")
        small_data_url = _to_data_url(small_png, "image/png")

        resp = client.embeddings.create(
            model=args.dinov2_model,
            input=small_data_url,
        )
        data = resp.model_dump() if hasattr(resp, "model_dump") else resp  # type: ignore[assignment]
        dim, items = _parse_openai_embeddings_response(data or {}, expected_model=args.dinov2_model)
        _must(items >= 1, "nv-dinov2 small-image returned empty embeddings (openai client)")
        _must(dim > 0, "nv-dinov2 small-image embedding dimension is 0 (openai client)")

    def test_dinov2_large_image() -> None:
        if args.skip_large:
            return

        if args.large_image_file:
            p = Path(args.large_image_file)
            _must(p.exists() and p.is_file(), f"large image file not found: {p}")
            raw = p.read_bytes()
            mime_type = _guess_mime_from_path(p)
        else:
            # Generate a valid PNG likely >200KB using random pixels and low compression.
            raw = _make_png_bytes(width=320, height=320, randomize=True)
            mime_type = "image/png"

        _must(
            len(raw) >= 200 * 1024,
            f"large image must be >=200KB for assets branch, got size={len(raw)}",
        )
        large_data_url = _to_data_url(raw, mime_type)

        status, data, text, _ = call_embeddings(args.dinov2_model, large_data_url)
        _must(status == 200, f"nv-dinov2 large-image failed: status={status}, body={_json_preview(text)}")
        dim, items = _parse_openai_embeddings_response(data or {}, expected_model=args.dinov2_model)
        _must(items >= 1, "nv-dinov2 large-image returned empty embeddings")
        _must(dim > 0, "nv-dinov2 large-image embedding dimension is 0")

    results.append(_run_test("text embedding (OpenAI-compatible)", test_text_embedding))
    results.append(_run_test("text embedding (OpenAI Python client)", test_text_embedding_openai_client))
    if args.skip_dinov2:
        print("[INFO] skip nv-dinov2 tests (--skip-dinov2)")
    else:
        results.append(_run_test("nv-dinov2 small-image (<200KB)", test_dinov2_small_image))
        results.append(_run_test("nv-dinov2 small-image (OpenAI Python client)", test_dinov2_small_image_openai_client))
        if args.skip_large:
            print("[INFO] skip large-image branch (--skip-large)")
        else:
            results.append(_run_test("nv-dinov2 large-image (>=200KB)", test_dinov2_large_image))

    failed = 0
    print(f"Base URL: {base_url}")
    print("Results:")
    for r in results:
        status = "SKIP" if r.skipped else ("OK" if r.ok else "FAIL")
        line = f"  - {status:4} {r.name} ({r.seconds:.2f}s)"
        if r.skipped and r.detail:
            line += f" :: {r.detail}"
        if not r.ok and r.detail:
            failed += 1
            line += f" :: {r.detail}"
        print(line)

    if failed:
        print(f"\nFAILED: {failed} test(s) failed.", file=sys.stderr)
        return 1

    print("\nALL PASSED")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

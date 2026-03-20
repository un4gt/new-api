#!/usr/bin/env python3
"""
Nvidia embeddings smoke test for new-api.

Covers:
1) text embedding model via /v1/embeddings (OpenAI-compatible)
2) nv-dinov2 small-image branch (<200KB inline base64)
3) nv-dinov2 large-image branch (>=200KB, gateway should upload to NVCF assets)

Usage:
  python3 bin/test_nividia.py --api-key <TOKEN>

Env shortcuts:
  NEW_API_BASE_URL (default: http://localhost:3000)
  NEW_API_API_KEY
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


def _normalize_base_url(base_url: str) -> str:
    return base_url.rstrip("/")


def _auth_headers(api_key: str) -> Dict[str, str]:
    return {
        "Authorization": f"Bearer {api_key}",
        "Content-Type": "application/json",
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


def _parse_openai_embeddings_response(data: Dict[str, Any], expected_model: str) -> Tuple[int, int]:
    _must(isinstance(data, dict), "response is not JSON object")
    _must(data.get("object") == "list", f"unexpected object={data.get('object')!r}")
    _must(data.get("model") == expected_model, f"unexpected model={data.get('model')!r}")

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
    except Exception as e:
        return TestResult(name=name, ok=False, seconds=time.time() - t0, detail=str(e))


def main() -> int:
    parser = argparse.ArgumentParser(description="new-api Nvidia embeddings smoke test")
    parser.add_argument(
        "--base-url",
        default=os.environ.get("NEW_API_BASE_URL", "http://localhost:3000"),
        help="Gateway base URL (env: NEW_API_BASE_URL)",
    )
    parser.add_argument(
        "--api-key",
        default=os.environ.get("NEW_API_API_KEY", ""),
        help="Gateway token (env: NEW_API_API_KEY)",
    )
    parser.add_argument("--timeout", type=float, default=180.0, help="Request timeout seconds")
    parser.add_argument("--insecure", action="store_true", help="Disable TLS verification")
    parser.add_argument("--verbose", action="store_true", help="Print request/response previews")

    parser.add_argument(
        "--text-model",
        default="nv-embed-v1",
        help="Nvidia text embedding model",
    )
    parser.add_argument(
        "--text-input",
        default="hello nvidia embedding",
        help="Input text for text embedding test",
    )
    parser.add_argument(
        "--dinov2-model",
        default="nv-dinov2",
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

    args = parser.parse_args()

    api_key = args.api_key.strip()
    if not api_key:
        print("Missing --api-key (or env NEW_API_API_KEY).", file=sys.stderr)
        return 2

    base_url = _normalize_base_url(args.base_url)
    verify_tls = not args.insecure
    headers = _auth_headers(api_key)
    session = requests.Session()

    def call_embeddings(model: str, input_value: Any) -> Tuple[int, Optional[Dict[str, Any]], str, float]:
        payload = {
            "model": model,
            "input": input_value,
        }
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
        status, data, text, _ = call_embeddings(args.text_model, args.text_input)
        _must(status == 200, f"text embedding failed: status={status}, body={_json_preview(text)}")
        dim, items = _parse_openai_embeddings_response(data or {}, expected_model=args.text_model)
        _must(items == 1, f"text embedding expected 1 item, got {items}")
        _must(dim > 0, "text embedding dimension is 0")

    def test_dinov2_small_image() -> None:
        small_png = _make_png_bytes(width=16, height=16, randomize=False)
        _must(len(small_png) < 200 * 1024, f"small image is not <200KB, size={len(small_png)}")
        small_data_url = _to_data_url(small_png, "image/png")

        status, data, text, _ = call_embeddings(args.dinov2_model, small_data_url)
        _must(status == 200, f"nv-dinov2 small-image failed: status={status}, body={_json_preview(text)}")
        dim, items = _parse_openai_embeddings_response(data or {}, expected_model=args.dinov2_model)
        _must(items >= 1, "nv-dinov2 small-image returned empty embeddings")
        _must(dim > 0, "nv-dinov2 small-image embedding dimension is 0")

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
    results.append(_run_test("nv-dinov2 small-image (<200KB)", test_dinov2_small_image))
    if args.skip_large:
        print("[INFO] skip large-image branch (--skip-large)")
    else:
        results.append(_run_test("nv-dinov2 large-image (>=200KB)", test_dinov2_large_image))

    failed = 0
    print(f"Base URL: {base_url}")
    print("Results:")
    for r in results:
        status = "OK" if r.ok else "FAIL"
        line = f"  - {status:4} {r.name} ({r.seconds:.2f}s)"
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

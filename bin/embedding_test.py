#!/usr/bin/env python3
"""
Embeddings E2E Test Script (new-api minimal build)

This script tests:
  1) OpenAI-style embeddings endpoint:
     POST /v1/embeddings  (model=gemini-embedding-001, text only)
     POST /v1/embeddings  (model=gemini-embedding-2-preview, text + optional extra_body.google.*)

  2) Vertex/Gemini embedContent endpoint (multimodal):
     POST /v1beta/models/gemini-embedding-2-preview:embedContent

Reference documentation:
  https://embedding-docs.tumuer.me/api/embeddings.html#endpoint
"""

from __future__ import annotations

import argparse
import base64
import binascii
import io
import json
import os
import struct
import sys
import time
import wave
import zlib
from dataclasses import dataclass
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
    return base_url.rstrip("/")


def _auth_headers(api_key: str) -> Dict[str, str]:
    return {
        "Authorization": f"Bearer {api_key}",
        "Content-Type": "application/json",
    }


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


def _must(cond: bool, msg: str) -> None:
    if not cond:
        raise AssertionError(msg)


def _parse_openai_embeddings_response(
    data: Dict[str, Any], expected_model: str, expected_items: Optional[int] = None
) -> Tuple[int, int]:
    _must(isinstance(data, dict), "response is not a JSON object")
    _must(data.get("object") == "list", f"unexpected object={data.get('object')!r}")
    _must(data.get("model") == expected_model, f"unexpected model={data.get('model')!r}")
    items = data.get("data")
    _must(isinstance(items, list) and len(items) > 0, "response.data must be a non-empty list")
    if expected_items is not None:
        _must(len(items) == expected_items, f"expected {expected_items} embeddings, got {len(items)}")

    first = items[0]
    _must(isinstance(first, dict), "response.data[0] must be an object")
    emb = first.get("embedding")
    _must(isinstance(emb, list) and len(emb) > 0, "response.data[0].embedding must be a non-empty list")
    dim = len(emb)
    for idx, it in enumerate(items):
        _must(isinstance(it, dict), f"response.data[{idx}] must be an object")
        e = it.get("embedding")
        _must(isinstance(e, list) and len(e) == dim, f"embedding dims mismatch at index {idx}")
    return dim, len(items)


def _extract_embedcontent_vector(data: Dict[str, Any]) -> List[float]:
    _must(isinstance(data, dict), "response is not a JSON object")

    # Vertex embedContent response usually:
    # {"embedding": {"values": [...]}, "usageMetadata": {...}}
    embedding = data.get("embedding")
    if isinstance(embedding, dict):
        values = embedding.get("values")
        if isinstance(values, list) and values:
            return values

    # Fallbacks (just in case upstream format changes)
    embeddings = data.get("embeddings")
    if isinstance(embeddings, list) and embeddings:
        first = embeddings[0]
        if isinstance(first, dict):
            values = first.get("values")
            if isinstance(values, list) and values:
                return values

    raise AssertionError("cannot find embedding values in embedContent response")


def _make_png_base64(
    width: int = 8, height: int = 8, rgb: Tuple[int, int, int] = (255, 0, 0)
) -> str:
    # Generate a small RGB PNG without third-party deps. This is more robust than
    # using an ultra-minimal PNG that some decoders/models may reject.
    if width <= 0:
        width = 8
    if height <= 0:
        height = 8
    r, g, b = rgb

    def _chunk(chunk_type: bytes, data: bytes) -> bytes:
        crc = binascii.crc32(chunk_type + data) & 0xFFFFFFFF
        return struct.pack(">I", len(data)) + chunk_type + data + struct.pack(">I", crc)

    sig = b"\x89PNG\r\n\x1a\n"
    ihdr = struct.pack(">IIBBBBB", width, height, 8, 2, 0, 0, 0)  # 8-bit RGB

    row = bytes([0]) + bytes([r, g, b]) * width  # filter=0, then RGB pixels
    raw = row * height
    idat = zlib.compress(raw, level=9)

    png = sig + _chunk(b"IHDR", ihdr) + _chunk(b"IDAT", idat) + _chunk(b"IEND", b"")
    return base64.b64encode(png).decode("utf-8")


def _make_wav_base64(duration_seconds: float = 1.0, sample_rate: int = 16000) -> str:
    if duration_seconds <= 0:
        duration_seconds = 1.0
    n_frames = int(sample_rate * duration_seconds)
    frames = b"\x00\x00" * n_frames  # 16-bit PCM silence

    bio = io.BytesIO()
    with wave.open(bio, "wb") as w:
        w.setnchannels(1)
        w.setsampwidth(2)
        w.setframerate(sample_rate)
        w.writeframes(frames)

    return base64.b64encode(bio.getvalue()).decode("utf-8")


def _make_minimal_pdf_bytes(text: str = "Hello PDF") -> bytes:
    # Build a minimal, valid PDF with proper xref offsets.
    # This avoids extra dependencies (reportlab, pypdf, etc.).

    def _obj(num: int, body: str) -> bytes:
        return f"{num} 0 obj\n{body}\nendobj\n".encode("latin-1")

    stream = f"BT /F1 24 Tf 72 720 Td ({text}) Tj ET\n".encode("latin-1")
    parts: List[bytes] = []

    header = b"%PDF-1.4\n%\xe2\xe3\xcf\xd3\n"
    parts.append(header)

    offsets: List[int] = [0]  # xref obj 0 is special
    current_len = len(header)

    # 1: Catalog
    obj1 = _obj(1, "<< /Type /Catalog /Pages 2 0 R >>")
    offsets.append(current_len)
    parts.append(obj1)
    current_len += len(obj1)

    # 2: Pages
    obj2 = _obj(2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
    offsets.append(current_len)
    parts.append(obj2)
    current_len += len(obj2)

    # 3: Page
    obj3 = _obj(
        3,
        "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] "
        "/Resources << /Font << /F1 5 0 R >> >> "
        "/Contents 4 0 R >>",
    )
    offsets.append(current_len)
    parts.append(obj3)
    current_len += len(obj3)

    # 4: Contents stream
    obj4_body = (
        f"<< /Length {len(stream)} >>\nstream\n".encode("latin-1")
        + stream
        + b"endstream"
    )
    obj4 = _obj(4, obj4_body.decode("latin-1"))
    offsets.append(current_len)
    parts.append(obj4)
    current_len += len(obj4)

    # 5: Font
    obj5 = _obj(5, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
    offsets.append(current_len)
    parts.append(obj5)
    current_len += len(obj5)

    xref_offset = current_len
    size = len(offsets)

    xref_lines = [f"xref\n0 {size}\n".encode("latin-1")]
    xref_lines.append(b"0000000000 65535 f \n")
    for off in offsets[1:]:
        xref_lines.append(f"{off:010d} 00000 n \n".encode("latin-1"))
    xref = b"".join(xref_lines)

    trailer = (
        f"trailer\n<< /Size {size} /Root 1 0 R >>\nstartxref\n{xref_offset}\n%%EOF\n".encode(
            "latin-1"
        )
    )

    return b"".join(parts) + xref + trailer


def _run_test(name: str, fn) -> TestResult:
    t0 = time.time()
    try:
        fn()
        return TestResult(name=name, ok=True, seconds=time.time() - t0)
    except SkipTest as e:
        return TestResult(name=name, ok=True, skipped=True, seconds=time.time() - t0, detail=str(e))
    except Exception as e:
        return TestResult(name=name, ok=False, seconds=time.time() - t0, detail=str(e))


def main() -> int:
    parser = argparse.ArgumentParser(description="new-api embeddings E2E test")
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
    parser.add_argument("--timeout", type=float, default=120.0, help="Request timeout seconds")
    parser.add_argument(
        "--insecure",
        action="store_true",
        help="Disable TLS verify (for self-signed certs)",
    )
    parser.add_argument(
        "--negative",
        action="store_true",
        help="Run negative tests (expect 400 errors on invalid requests)",
    )
    parser.add_argument(
        "--verbose",
        action="store_true",
        help="Print request/response previews for easier debugging",
    )
    parser.add_argument(
        "--video-file",
        default="",
        help="Optional local video file path to test video inlineData (mp4/mpeg)",
    )
    parser.add_argument(
        "--video-mime",
        default="video/mp4",
        help="MIME for --video-file (video/mp4 or video/mpeg)",
    )
    parser.add_argument(
        "--file-uri",
        default="",
        help="Optional http(s) URL to test embedContent fileData.fileUri",
    )
    parser.add_argument(
        "--file-uri-mime",
        default="application/pdf",
        help="MIME for --file-uri (image/png,image/jpeg,application/pdf,video/mp4,video/mpeg,audio/wav,audio/mp3)",
    )

    args = parser.parse_args()

    base_url = _normalize_base_url(args.base_url)
    api_key = args.api_key.strip()
    if not api_key:
        print("Missing --api-key (or env NEW_API_API_KEY).", file=sys.stderr)
        return 2

    verify_tls = not args.insecure
    headers = _auth_headers(api_key)

    sess = requests.Session()

    results: List[TestResult] = []

    def call(method: str, path: str, body: Optional[Dict[str, Any]]) -> Tuple[int, Optional[Dict[str, Any]], str, float]:
        url = f"{base_url}{path}"
        status, data, text, dt = _request_json(sess, method, url, headers, body, args.timeout, verify_tls)
        if args.verbose:
            body_preview = ""
            if body is not None:
                body_preview = json.dumps(body, ensure_ascii=False)[:4000]
            print(f"\n[{method}] {path} -> {status} ({dt:.2f}s)")
            if body_preview:
                print(f"request: {_json_preview(body_preview, limit=800)}")
            print(f"response: {_json_preview(text, limit=800)}")
        return status, data, text, dt

    def test_models_list() -> None:
        status, data, text, _ = call("GET", "/v1/models", None)
        _must(status == 200, f"GET /v1/models failed: status={status}, body={_json_preview(text)}")
        _must(isinstance(data, dict), "models response is not JSON object")
        _must("data" in data, "models response missing 'data'")

    def test_embedding_001_single() -> None:
        payload = {"model": "gemini-embedding-001", "input": "hello world"}
        status, data, text, _ = call("POST", "/v1/embeddings", payload)
        _must(status == 200, f"/v1/embeddings failed: status={status}, body={_json_preview(text)}")
        _parse_openai_embeddings_response(data or {}, expected_model="gemini-embedding-001", expected_items=1)

    def test_embedding_001_batch() -> None:
        payload = {"model": "gemini-embedding-001", "input": ["hello", "world"]}
        status, data, text, _ = call("POST", "/v1/embeddings", payload)
        _must(status == 200, f"/v1/embeddings batch failed: status={status}, body={_json_preview(text)}")
        _parse_openai_embeddings_response(data or {}, expected_model="gemini-embedding-001", expected_items=2)

    def test_embedding_001_dimensions_best_effort() -> None:
        # Best-effort: detect default dim, then request same dim via `dimensions`.
        status, data, text, _ = call(
            "POST", "/v1/embeddings", {"model": "gemini-embedding-001", "input": "dim probe"}
        )
        _must(status == 200, f"/v1/embeddings probe failed: status={status}, body={_json_preview(text)}")
        dim, _ = _parse_openai_embeddings_response(data or {}, expected_model="gemini-embedding-001")

        status2, data2, text2, _ = call(
            "POST", "/v1/embeddings", {"model": "gemini-embedding-001", "input": "dim override", "dimensions": dim}
        )
        if status2 != 200:
            raise AssertionError(
                f"dimensions override failed (best-effort): status={status2}, body={_json_preview(text2)}"
            )
        dim2, _ = _parse_openai_embeddings_response(data2 or {}, expected_model="gemini-embedding-001")
        _must(dim2 == dim, f"expected dim={dim}, got dim={dim2}")

    def test_embedding_2_preview_text() -> None:
        payload = {"model": "gemini-embedding-2-preview", "input": "hello embedding-2"}
        status, data, text, _ = call("POST", "/v1/embeddings", payload)
        _must(status == 200, f"/v1/embeddings (2-preview) failed: status={status}, body={_json_preview(text)}")
        dim, items = _parse_openai_embeddings_response(data or {}, expected_model="gemini-embedding-2-preview", expected_items=1)
        _must(dim > 0 and items == 1, "invalid embedding-2 /v1/embeddings response")

    def test_embedding_2_preview_extra_body_multimodal() -> None:
        # Non-standard extension: allow passing Gemini embedContent-style payloads
        # via extra_body.google.requests while keeping /v1/embeddings compatibility.
        png_b64 = _make_png_base64()
        payload = {
            "model": "gemini-embedding-2-preview",
            "extra_body": {
                "google": {
                    "requests": [
                        {"content": {"role": "user", "parts": [{"text": "hello extra_body"}]}},
                        {
                            "content": {
                                "role": "user",
                                "parts": [
                                    {"text": "embed this image (extra_body)"},
                                    {"inlineData": {"mimeType": "image/png", "data": png_b64}},
                                ],
                            }
                        },
                    ]
                }
            },
        }
        status, data, text, _ = call("POST", "/v1/embeddings", payload)
        _must(status == 200, f"/v1/embeddings (2-preview extra_body) failed: status={status}, body={_json_preview(text)}")
        dim, items = _parse_openai_embeddings_response(data or {}, expected_model="gemini-embedding-2-preview", expected_items=2)
        _must(dim > 0 and items == 2, "invalid embedding-2 /v1/embeddings extra_body response")

    def _embedcontent_post(payload: Dict[str, Any]) -> Dict[str, Any]:
        status, data, text, _ = call("POST", "/v1beta/models/gemini-embedding-2-preview:embedContent", payload)
        _must(status == 200, f"embedContent failed: status={status}, body={_json_preview(text)}")
        _must(isinstance(data, dict), "embedContent response is not JSON object")
        return data or {}

    def test_embedcontent_2_text() -> None:
        payload = {
            "content": {"role": "user", "parts": [{"text": "hello embedContent"}]},
        }
        data = _embedcontent_post(payload)
        vec = _extract_embedcontent_vector(data)
        _must(len(vec) > 0, "empty embedding vector")

    def test_embedcontent_2_dimensions_best_effort() -> None:
        # Best-effort: discover default dim, then request same dim explicitly.
        probe = {"content": {"role": "user", "parts": [{"text": "dim probe"}]}}
        data = _embedcontent_post(probe)
        vec = _extract_embedcontent_vector(data)
        dim = len(vec)
        _must(dim > 0, "empty embedding vector")

        override = {
            "content": {"role": "user", "parts": [{"text": "dim override"}]},
            "embedContentConfig": {"outputDimensionality": dim},
        }
        data2 = _embedcontent_post(override)
        vec2 = _extract_embedcontent_vector(data2)
        _must(len(vec2) == dim, f"expected dim={dim}, got dim={len(vec2)}")

    def test_embedcontent_2_image_png() -> None:
        png_b64 = _make_png_base64()
        payload = {
            "content": {
                "role": "user",
                "parts": [
                    {"text": "embed this image"},
                    {
                        "inlineData": {
                            "mimeType": "image/png",
                            "data": png_b64,
                        }
                    },
                ],
            },
        }
        data = _embedcontent_post(payload)
        vec = _extract_embedcontent_vector(data)
        _must(len(vec) > 0, "empty embedding vector")

    def test_embedcontent_2_pdf() -> None:
        pdf_bytes = _make_minimal_pdf_bytes("Hello PDF")
        pdf_b64 = base64.b64encode(pdf_bytes).decode("utf-8")
        payload = {
            "content": {
                "role": "user",
                "parts": [
                    {"text": "embed this pdf"},
                    {
                        "inlineData": {
                            "mimeType": "application/pdf",
                            "data": pdf_b64,
                        }
                    },
                ],
            },
        }
        data = _embedcontent_post(payload)
        vec = _extract_embedcontent_vector(data)
        _must(len(vec) > 0, "empty embedding vector")

    def test_embedcontent_2_pdf_document_ocr() -> None:
        pdf_bytes = _make_minimal_pdf_bytes("OCR PDF")
        pdf_b64 = base64.b64encode(pdf_bytes).decode("utf-8")
        payload = {
            "content": {
                "role": "user",
                "parts": [
                    {"text": "embed this pdf with ocr"},
                    {
                        "inlineData": {
                            "mimeType": "application/pdf",
                            "data": pdf_b64,
                        }
                    },
                ],
            },
            "embedContentConfig": {"documentOcr": True},
        }
        data = _embedcontent_post(payload)
        vec = _extract_embedcontent_vector(data)
        _must(len(vec) > 0, "empty embedding vector")

    def test_embedcontent_2_audio_wav() -> None:
        wav_b64 = _make_wav_base64(duration_seconds=1.0, sample_rate=16000)
        payload = {
            "content": {
                "role": "user",
                "parts": [
                    {"text": "embed this wav"},
                    {"inlineData": {"mimeType": "audio/wav", "data": wav_b64}},
                ],
            },
        }
        data = _embedcontent_post(payload)
        vec = _extract_embedcontent_vector(data)
        _must(len(vec) > 0, "empty embedding vector")

    def test_embedcontent_2_video_optional() -> None:
        if not args.video_file:
            _skip("no --video-file provided")
        with open(args.video_file, "rb") as f:
            raw = f.read()
        b64 = base64.b64encode(raw).decode("utf-8")
        payload = {
            "content": {
                "role": "user",
                "parts": [
                    {"text": "embed this video"},
                    {"inlineData": {"mimeType": args.video_mime, "data": b64}},
                ],
            },
        }
        data = _embedcontent_post(payload)
        vec = _extract_embedcontent_vector(data)
        _must(len(vec) > 0, "empty embedding vector")

    def test_embedcontent_2_fileuri_optional() -> None:
        if not args.file_uri:
            _skip("no --file-uri provided")
        payload = {
            "content": {
                "role": "user",
                "parts": [
                    {
                        "fileData": {
                            "mimeType": args.file_uri_mime,
                            "fileUri": args.file_uri,
                        }
                    }
                ],
            },
        }
        data = _embedcontent_post(payload)
        vec = _extract_embedcontent_vector(data)
        _must(len(vec) > 0, "empty embedding vector")

    def negative_tests() -> None:
        # 1) outputDimensionality > 3072 should be rejected by gateway
        payload2 = {
            "content": {"role": "user", "parts": [{"text": "dims too large"}]},
            "embedContentConfig": {"outputDimensionality": 3073},
        }
        status2, _, text2, _ = call("POST", "/v1beta/models/gemini-embedding-2-preview:embedContent", payload2)
        _must(status2 == 400, f"expected 400 for dims>3072, got {status2}: {text2}")

        # 2) too many images (7) should fail fast without upstream dependency
        too_many_images = {
            "content": {
                "role": "user",
                "parts": [
                    {"inlineData": {"mimeType": "image/jpeg", "data": "AA=="}},
                    {"inlineData": {"mimeType": "image/jpeg", "data": "AA=="}},
                    {"inlineData": {"mimeType": "image/jpeg", "data": "AA=="}},
                    {"inlineData": {"mimeType": "image/jpeg", "data": "AA=="}},
                    {"inlineData": {"mimeType": "image/jpeg", "data": "AA=="}},
                    {"inlineData": {"mimeType": "image/jpeg", "data": "AA=="}},
                    {"inlineData": {"mimeType": "image/jpeg", "data": "AA=="}},
                ],
            },
        }
        status3, _, text3, _ = call("POST", "/v1beta/models/gemini-embedding-2-preview:embedContent", too_many_images)
        _must(status3 == 400, f"expected 400 for too many images, got {status3}: {text3}")

        # 3) PDF page count > 6 should fail fast (best-effort PDF page counter)
        fake_pdf = b"%PDF-1.4\n<< /Type /Pages /Count 7 >>\n"
        fake_pdf_b64 = base64.b64encode(fake_pdf).decode("utf-8")
        too_many_pages = {
            "content": {
                "role": "user",
                "parts": [{"inlineData": {"mimeType": "application/pdf", "data": fake_pdf_b64}}],
            },
        }
        status4, _, text4, _ = call("POST", "/v1beta/models/gemini-embedding-2-preview:embedContent", too_many_pages)
        _must(status4 == 400, f"expected 400 for pdf pages > 6, got {status4}: {text4}")

        # 4) gs:// is not supported for file_uri
        gs_uri = {
            "content": {
                "role": "user",
                "parts": [{"fileData": {"mimeType": "application/pdf", "fileUri": "gs://bucket/a.pdf"}}],
            },
        }
        status5, _, text5, _ = call("POST", "/v1beta/models/gemini-embedding-2-preview:embedContent", gs_uri)
        _must(status5 == 400, f"expected 400 for gs:// file_uri, got {status5}: {text5}")

    # Run suite
    results.append(_run_test("GET /v1/models", test_models_list))
    results.append(_run_test("POST /v1/embeddings (001 single)", test_embedding_001_single))
    results.append(_run_test("POST /v1/embeddings (001 batch)", test_embedding_001_batch))
    results.append(_run_test("POST /v1/embeddings (001 dimensions best-effort)", test_embedding_001_dimensions_best_effort))
    results.append(_run_test("POST /v1/embeddings (2-preview text)", test_embedding_2_preview_text))
    results.append(_run_test("POST /v1/embeddings (2-preview extra_body multimodal)", test_embedding_2_preview_extra_body_multimodal))

    results.append(_run_test("POST :embedContent (2 text)", test_embedcontent_2_text))
    results.append(_run_test("POST :embedContent (2 dimensions best-effort)", test_embedcontent_2_dimensions_best_effort))
    results.append(_run_test("POST :embedContent (2 image/png)", test_embedcontent_2_image_png))
    results.append(_run_test("POST :embedContent (2 application/pdf)", test_embedcontent_2_pdf))
    results.append(_run_test("POST :embedContent (2 application/pdf, documentOcr)", test_embedcontent_2_pdf_document_ocr))
    results.append(_run_test("POST :embedContent (2 audio/wav)", test_embedcontent_2_audio_wav))
    results.append(_run_test("POST :embedContent (2 video optional)", test_embedcontent_2_video_optional))
    results.append(_run_test("POST :embedContent (2 fileUri optional)", test_embedcontent_2_fileuri_optional))

    if args.negative:
        results.append(_run_test("Negative tests", negative_tests))

    # Print summary
    print(f"Base URL: {base_url}")
    print("Results:")
    failed = 0
    for r in results:
        status = "FAIL"
        if r.ok:
            status = "OK"
        if r.skipped:
            status = "SKIP"
        line = f"  - {status:4} {r.name} ({r.seconds:.2f}s)"
        if not r.ok:
            failed += 1
            line += f" :: {r.detail}"
        elif r.skipped and r.detail:
            line += f" :: {r.detail}"
        print(line)

    if failed:
        print(f"\nFAILED: {failed} test(s) failed.", file=sys.stderr)
        return 1
    print("\nALL PASSED")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

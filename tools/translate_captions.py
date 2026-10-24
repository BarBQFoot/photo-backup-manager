"""Resume text-only Gemini translation. Default is a network-free plan.

Add --execute to send requests. This does not change the application database.
"""

import argparse
from contextlib import contextmanager
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import sys
import time
import urllib.error
import urllib.request
from uuid import uuid4

if __package__:
    from .validate_captions import validate
else:
    from validate_captions import validate


ROOT = Path(__file__).resolve().parents[1]
DEFAULT_MODEL = "gemini-3.5-flash-lite"  # Same default as the existing Go application.


def write_json(path, value):
    temporary = path.with_name(path.name + "." + uuid4().hex + ".tmp")
    try:
        temporary.write_text(json.dumps(value, ensure_ascii=False, indent=2), encoding="utf-8")
        temporary.replace(path)
    finally:
        temporary.unlink(missing_ok=True)


def fingerprint(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, ensure_ascii=False,
                                     separators=(",", ":")).encode("utf-8")).hexdigest()


def load_key():
    key = os.environ.get("GEMINI_API_KEY", "").strip()
    if key:
        return key
    path = ROOT / ".env"
    if path.is_file():
        for line in path.read_text(encoding="utf-8-sig").splitlines():
            line = line.strip().removeprefix("export ")
            if not line or line.startswith("#") or "=" not in line:
                continue
            name, value = line.split("=", 1)
            if name.strip() == "GEMINI_API_KEY":
                value = value.strip()
                if value.startswith(('"', "'")):
                    quote = value[0]
                    end = value.find(quote, 1)
                    if end < 0:
                        raise ValueError("Unterminated GEMINI_API_KEY value in .env")
                    return value[1:end].strip()
                return value.split(" #", 1)[0].strip()
    return ""


def read_catalog(run):
    payloads = json.loads((run / "translation_payloads.json").read_text(encoding="utf-8-sig"))
    # Reuse the validator's source checks without treating missing responses as success.
    validate(payloads, [])
    if not payloads:
        raise ValueError("No translation payloads")
    if any(set(item) != {"file_id", "caption_en"} for item in payloads):
        raise ValueError("Translation payloads require only file_id and caption_en")
    batches = []
    flattened = []
    for path in sorted((run / "translation_batches").glob("batch_[0-9][0-9][0-9].json")):
        batch = json.loads(path.read_text(encoding="utf-8-sig"))
        if not isinstance(batch, list) or not batch:
            raise ValueError(f"Empty or invalid batch: {path.name}")
        validate(batch, [])
        batches.append((path.stem, batch))
        flattened.extend(batch)
    if flattened != payloads:
        raise ValueError("Translation batches do not exactly match translation_payloads.json; prepare them again in a new run")
    return payloads, batches


def existing_response(run, name, batch):
    folder = run / "translation_responses"
    meta = folder / f"{name}_meta.json"
    if meta.is_file():
        previous = json.loads(meta.read_text(encoding="utf-8"))
        if previous.get("request_sha256") != fingerprint(batch):
            raise ValueError(f"{name}: source text changed since translation; preserve this run and create a new run")
    # Human-edited translations take priority, then API output, then manual output.
    for suffix in ("_translation_edited.json", "_api_response.json", "_response.json"):
        path = folder / (name + suffix)
        if path.is_file():
            try:
                response = json.loads(path.read_text(encoding="utf-8-sig"))
                if validate(batch, response)["structure_and_ids_valid"]:
                    return response, path
            except (ValueError, KeyError, TypeError):
                continue
    return None, None


def build_request(batch, prompt, max_tokens):
    return {
        "contents": [{"role": "user", "parts": [{"text": prompt + "\n" + json.dumps(batch, ensure_ascii=False)}]}],
        "generationConfig": {
            "temperature": 0.1, "maxOutputTokens": max_tokens,
            "responseMimeType": "application/json",
            "responseSchema": {
                # Keep the server schema small. Exact count/IDs are checked locally.
                "type": "ARRAY",
                "items": {"type": "OBJECT", "required": ["file_id", "caption"],
                          "properties": {"file_id": {"type": "STRING"},
                                         "caption": {"type": "STRING", "nullable": True}}},
            },
        },
    }


class APIError(Exception):
    def __init__(self, message, retryable=False, retry_after=0, http_status=None):
        super().__init__(message)
        self.retryable = retryable
        self.retry_after = retry_after
        self.http_status = http_status


def provider_error_message(content, key):
    """Expose useful provider diagnostics without credentials or raw HTTP bodies."""
    try:
        error = json.loads(content).get("error", {})
        message = error.get("message", "")
        status = error.get("status", "")
        reasons = [detail["reason"] for detail in error.get("details", [])
                   if isinstance(detail, dict) and isinstance(detail.get("reason"), str)]
        text = " ".join(str(value) for value in [status, message, *reasons] if value)
    except (ValueError, AttributeError, TypeError):
        return ""
    if key:
        text = text.replace(key, "[REDACTED]")
    text = re.sub(r"AIza[A-Za-z0-9_-]{20,}", "[REDACTED]", text)
    return " ".join(text.split())[:3000]


def request_api(model, key, body, timeout):
    url = f"https://generativelanguage.googleapis.com/v1beta/models/{model}:generateContent"
    request = urllib.request.Request(url, data=json.dumps(body, ensure_ascii=False).encode("utf-8"),
                                     headers={"Content-Type": "application/json", "x-goog-api-key": key}, method="POST")
    return request_json(request, key, timeout)


def check_access(model, key, timeout):
    request = urllib.request.Request(
        f"https://generativelanguage.googleapis.com/v1beta/models/{model}",
        headers={"x-goog-api-key": key}, method="GET")
    result = request_json(request, key, timeout)
    return {"access_check": "passed", "model": result.get("name"),
            "supported_generation_methods": result.get("supportedGenerationMethods", []),
            "generation_requests_sent": 0}


def request_json(request, key, timeout):
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            return json.load(response)
    except urllib.error.HTTPError as error:
        # Show only sanitized provider diagnostics, never headers or raw bodies.
        content = error.read().decode("utf-8", errors="replace")
        lower = content.lower()
        daily = any(term in lower for term in ("per_day", "perday", "per day", "daily"))
        try:
            retry_after = max(0, float(error.headers.get("Retry-After", "0")))
        except (ValueError, TypeError):
            retry_after = 0
        try:
            details = json.loads(content).get("error", {}).get("details", [])
            for detail in details:
                if detail.get("@type", "").endswith("RetryInfo"):
                    retry_after = max(retry_after, float(detail.get("retryDelay", "0s").removesuffix("s")))
        except (ValueError, TypeError, AttributeError):
            pass
        hint = {400: "Check model and request settings", 401: "Check API key",
                403: "Check API key and project permissions", 404: "Model unavailable; select an available model with --model",
                429: "Daily quota reached" if daily else "Rate limit or quota reached"}.get(error.code, "Provider error")
        if retry_after > 60:
            hint += f"; provider requested a {retry_after:.0f}s wait, stopping for later resume"
        diagnostic = provider_error_message(content, key)
        if diagnostic:
            hint += f". Provider: {diagnostic}"
        raise APIError(f"Gemini HTTP {error.code}: {hint}",
                       retryable=(error.code in {429, 500, 502, 503, 504} and not daily and retry_after <= 60),
                       retry_after=retry_after, http_status=error.code) from None
    except (urllib.error.URLError, TimeoutError, OSError):
        # A timed-out POST may already have been processed; avoid automatic duplicate calls.
        raise APIError("Network error or timeout; completion is unknown. Saved batches are preserved; inspect before rerunning.") from None


class ResponseValidationError(ValueError):
    """A complete provider response cannot be mapped safely to the input IDs."""


def parse_response(envelope, batch):
    candidates = envelope.get("candidates", [])
    if not candidates or candidates[0].get("finishReason") != "STOP":
        reason = candidates[0].get("finishReason", "NO_CANDIDATE") if candidates else "NO_CANDIDATE"
        raise ValueError(f"Incomplete or blocked response ({reason}); response not accepted")
    parts = candidates[0].get("content", {}).get("parts", [])
    text = "".join(part.get("text", "") for part in parts if not part.get("thought", False))
    try:
        response = json.loads(text)
    except json.JSONDecodeError:
        raise ResponseValidationError('Response caption JSON invalid') from None
    try:
        report = validate(batch, response)
    except ValueError as error:
        raise ResponseValidationError('Response caption format invalid: ' + str(error)) from None
    if not report["structure_and_ids_valid"]:
        raise ResponseValidationError("Response IDs or caption format invalid: " + "; ".join(report["errors"]))
    return response, report


@contextmanager
def run_lock(run):
    # OS-level lock is automatically released on process exit, including crashes.
    with (run / ".translation.lock").open("a+b") as stream:
        if stream.tell() == 0:
            stream.write(b"0")
            stream.flush()
        stream.seek(0)
        try:
            if os.name == "nt":
                import msvcrt
                msvcrt.locking(stream.fileno(), msvcrt.LK_NBLCK, 1)
            else:
                import fcntl
                fcntl.flock(stream, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except OSError:
            raise ValueError("Another translation process is already running for this folder") from None
        try:
            yield
        finally:
            stream.seek(0)
            if os.name == "nt":
                msvcrt.locking(stream.fileno(), msvcrt.LK_UNLCK, 1)
            else:
                fcntl.flock(stream, fcntl.LOCK_UN)


def merge_results(run, payloads, batches, requests_sent, error=None):
    translated = {}
    states = []
    for name, batch in batches:
        response, path = existing_response(run, name, batch)
        states.append({"batch": name, "status": "saved" if response is not None else "pending",
                       "response_file": str(path) if path else None})
        if response is not None:
            translated.update({item["file_id"]: item["caption"] for item in response})
    merged = []
    for item in payloads:
        file_id = item["file_id"]
        saved = file_id in translated
        caption = translated.get(file_id)
        merged.append({**item, "caption": caption,
                       "translation_status": "pending" if not saved else "needs_review" if caption is None else "translated"})
    summary = {"created_at": datetime.now(timezone.utc).isoformat(), "total": len(payloads),
               "translated": sum(item["translation_status"] == "translated" for item in merged),
               "needs_review": sum(item["translation_status"] == "needs_review" for item in merged),
               "pending": sum(item["translation_status"] == "pending" for item in merged),
               "api_requests_this_run": requests_sent, "batches": states, "error": error,
               "accuracy": "Structure and IDs checked; translation and image accuracy not certified."}
    write_json(run / "captions_translated.json", merged)
    write_json(run / "translation_summary.json", summary)
    return summary


def execute(run, payloads, batches, key, model, prompt, limit, interval, retries, max_tokens, timeout, on_saved=None):
    folder = run / "translation_responses"
    folder.mkdir(exist_ok=True)
    requests_sent = 0
    completed = 0
    error_text = None
    try:
        for name, batch in batches:
            response, path = existing_response(run, name, batch)
            if response is not None:
                # Adopt manually supplied responses without modifying their contents.
                if not (folder / f"{name}_meta.json").exists():
                    write_json(folder / f"{name}_meta.json", {"request_sha256": fingerprint(batch), "source": "existing_response"})
                print(f"SKIP {name}: {len(response)} saved items", flush=True)
                if on_saved:
                    on_saved(response)
                continue
            if limit is not None and completed >= limit:
                break
            write_json(folder / f"{name}_meta.json", {"request_sha256": fingerprint(batch),
                       "prompt_sha256": hashlib.sha256(prompt.encode("utf-8")).hexdigest(),
                       "model": model, "source": "pending_api_request"})
            body = build_request(batch, prompt, max_tokens)
            for attempt in range(retries + 1):
                if requests_sent and attempt == 0:
                    time.sleep(interval)
                requests_sent += 1
                print(f"SEND {name}: {len(batch)} items (attempt {attempt + 1})", flush=True)
                try:
                    envelope = request_api(model, key, body, timeout)
                    # Preserve each provider reply, even if validation fails.
                    stamp = datetime.now().strftime("%Y%m%d_%H%M%S_%f")
                    # Provider text is data; remove any literal copy of the credential before saving.
                    envelope = json.loads(json.dumps(envelope, ensure_ascii=False).replace(key, "[REDACTED]"))
                    write_json(folder / f"{name}_attempt_{stamp}.json", envelope)
                    response, report = parse_response(envelope, batch)
                    write_json(folder / f"{name}_api_response.json", response)
                    write_json(folder / f"{name}_api_validation.json", report)
                    write_json(folder / f"{name}_meta.json", {"request_sha256": fingerprint(batch),
                               "prompt_sha256": hashlib.sha256(prompt.encode("utf-8")).hexdigest(),
                               "model": model, "usage": envelope.get("usageMetadata", {}), "source": "api"})
                    completed += 1
                    print(f"SAVED {name}: {len(response)} validated IDs", flush=True)
                    merge_results(run, payloads, batches, requests_sent)
                    if on_saved:
                        on_saved(response)
                    break
                except ResponseValidationError as error:
                    if attempt == retries:
                        raise
                    print(f"{error}; retrying the same batch after {interval:.0f}s", flush=True)
                    time.sleep(interval)
                except APIError as error:
                    if not error.retryable or attempt == retries:
                        raise
                    base_delay = 60 if error.http_status == 503 else 5
                    delay = max(interval, min(60, base_delay * (2 ** attempt)), error.retry_after)
                    print(f"{error}; retrying after {delay:.0f}s", flush=True)
                    time.sleep(delay)
    except KeyboardInterrupt:
        error_text = "Interrupted; completed batches preserved"
    except (APIError, ValueError, KeyError, TypeError, OSError) as error:
        error_text = str(error).replace(key, "[REDACTED]")
    summary = merge_results(run, payloads, batches, requests_sent, error_text)
    print(json.dumps({key: value for key, value in summary.items() if key != "batches"}, ensure_ascii=False, indent=2))
    return 1 if error_text else 0


def main():
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8")
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--run", type=Path, required=True)
    parser.add_argument("--execute", action="store_true", help="Send real Gemini requests; otherwise only print a plan")
    parser.add_argument("--check-access", action="store_true", help="GET model metadata only; do not generate or translate content")
    parser.add_argument("--model", default=DEFAULT_MODEL)
    parser.add_argument("--max-batches", type=int)
    parser.add_argument("--interval", type=float, default=15)
    parser.add_argument("--retries", type=int, default=1)
    parser.add_argument("--max-output-tokens", type=int, default=8192)
    parser.add_argument("--timeout", type=float, default=120)
    args = parser.parse_args()
    if args.execute and args.check_access:
        parser.error("Choose --execute or --check-access, not both")
    if (args.max_batches is not None and args.max_batches < 1) or args.interval < 0 or args.retries < 0 or args.max_output_tokens < 1 or args.timeout <= 0:
        parser.error("Invalid numeric option")
    model = args.model.removeprefix("models/")
    if not re.fullmatch(r"[A-Za-z0-9._-]+", model):
        parser.error("Invalid model ID")
    key = ""
    try:
        run = args.run.resolve()
        payloads, batches = read_catalog(run)
        pending = [name for name, batch in batches if existing_response(run, name, batch)[0] is None]
        key = load_key()
        planned = pending[:args.max_batches] if args.max_batches is not None else pending
        print(json.dumps({"mode": "check_access" if args.check_access else "execute" if args.execute else "dry_run", "model": model,
                          "key_configured": bool(key), "total_items": len(payloads), "total_batches": len(batches),
                          "saved_batches": len(batches) - len(pending), "pending_batches": len(pending),
                          "planned_batches": planned, "planned_requests_before_retries": len(planned),
                          "interval_seconds": args.interval}, indent=2), flush=True)
        if args.check_access:
            if not key:
                raise ValueError("GEMINI_API_KEY is not configured")
            print(json.dumps(check_access(model, key, args.timeout), indent=2))
            return 0
        if not args.execute:
            return 0
        if planned and not key:
            raise ValueError("Set GEMINI_API_KEY in the process environment or project .env; do not paste it into chat")
        prompt = (ROOT / "docs/GEMINI-TRANSLATE-CAPTIONS.txt").read_text(encoding="utf-8")
        with run_lock(run):
            return execute(run, payloads, batches, key, model, prompt, args.max_batches,
                           args.interval, args.retries, args.max_output_tokens, args.timeout)
    except (APIError, ValueError, KeyError, TypeError, OSError) as error:
        # Do not dump stack traces or credential-bearing request objects.
        message = str(error).replace(key, "[REDACTED]") if key else str(error)
        print(f"Cannot continue: {message}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())

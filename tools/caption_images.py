r"""Caption photos locally with Florence-2, loading the model only once.

Windows: .\.venv\Scripts\python.exe -u tools/caption_images.py --input test-images/50 --pilot
"""

import argparse
import csv
from datetime import datetime
import hashlib
from importlib.metadata import version
import json
from pathlib import Path
import re
import time

import torch
from PIL import Image, ImageOps
from transformers import AutoProcessor, Florence2ForConditionalGeneration


ROOT = Path(__file__).resolve().parents[1]
MODEL_ID = "florence-community/Florence-2-base-ft"
TASK = "<CAPTION>"
EXTENSIONS = {".jpg", ".jpeg", ".png", ".webp", ".bmp", ".tif", ".tiff"}
# Fixed diverse pilot selection, not a statistically representative sample.
PILOT_FILES = [
    "000000025139.jpg", "000000026204.jpg", "000000045229.jpg",
    "000000080671.jpg", "000000166478.jpg", "000000180011.jpg",
    "000000348488.jpg", "000000349837.jpg", "000000357816.jpg",
    "000000553221.jpg",
]


def write_json(path, value):
    temporary = path.with_suffix(path.suffix + ".tmp")
    temporary.write_text(json.dumps(value, ensure_ascii=False, indent=2), encoding="utf-8")
    temporary.replace(path)


def synchronize(device):
    if device == "cuda":
        torch.cuda.synchronize()
    elif device == "mps":
        torch.mps.synchronize()


def select_photos(folder, pilot, limit):
    ids = {}
    manifest = folder / "manifest.csv"
    if manifest.is_file():
        with manifest.open(encoding="utf-8-sig", newline="") as stream:
            for row in csv.DictReader(stream):
                if row["filename"] in ids:
                    raise ValueError("Duplicate filename in manifest")
                ids[row["filename"]] = row["file_id"]
    if pilot:
        paths = [folder / name for name in PILOT_FILES]
        missing = [str(path) for path in paths if not path.is_file()]
        if missing:
            raise FileNotFoundError(f"Pilot photos not found: {missing}")
    else:
        paths = sorted(path for path in folder.rglob("*")
                       if path.is_file() and path.suffix.lower() in EXTENSIONS)
        if limit:
            paths = paths[:limit]
    result, seen = [], set()
    for path in paths:
        relative = path.relative_to(folder).as_posix()
        file_id = ids.get(relative) or "photo_" + hashlib.sha256(relative.encode("utf-8")).hexdigest()[:24]
        if not re.fullmatch(r"[A-Za-z0-9_-]{1,100}", file_id) or file_id in seen:
            raise ValueError(f"Unsafe or duplicate file_id: {file_id!r}")
        seen.add(file_id)
        result.append((file_id, path))
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--input", type=Path, required=True)
    parser.add_argument("--output", type=Path)
    parser.add_argument("--pilot", action="store_true", help="Use the fixed 10-photo pilot selection")
    parser.add_argument("--limit", type=int)
    parser.add_argument("--device", choices=["auto", "cuda", "mps", "cpu"], default="auto")
    parser.add_argument("--beams", type=int, default=1)
    parser.add_argument("--max-new-tokens", type=int, default=64)
    args = parser.parse_args()
    if args.beams < 1 or args.max_new_tokens < 1 or (args.limit is not None and args.limit < 1):
        parser.error("Numeric options must be positive")
    if args.pilot and args.limit is not None:
        parser.error("Choose --pilot or --limit, not both")
    folder = args.input.resolve()
    if not folder.is_dir():
        parser.error(f"Input folder not found: {folder}")
    items = select_photos(folder, args.pilot, args.limit)
    if not items:
        parser.error("No supported images found")
    device = args.device
    if device == "auto":
        device = "cuda" if torch.cuda.is_available() else "mps" if torch.backends.mps.is_available() else "cpu"
    if device == "cuda" and not torch.cuda.is_available():
        parser.error("CUDA unavailable; use --device cpu")
    if device == "mps" and not torch.backends.mps.is_available():
        parser.error("MPS unavailable; use --device cpu")
    dtype = torch.float16 if device == "cuda" else torch.float32
    output = (args.output or ROOT / "results" / (
        "florence_" + datetime.now().strftime("%Y%m%d_%H%M%S_%f"))).resolve()
    output.mkdir(parents=True, exist_ok=False)
    (output / "photos").mkdir()
    print(f"Device: {device} | Photos: {len(items)} | Task: {TASK}", flush=True)
    print(f"Output: {output}\nLoading Florence-2...", flush=True)
    started = time.perf_counter()
    processor = AutoProcessor.from_pretrained(MODEL_ID, local_files_only=True)
    model = Florence2ForConditionalGeneration.from_pretrained(
        MODEL_ID, dtype=dtype, local_files_only=True).to(device).eval()
    synchronize(device)
    load_seconds = time.perf_counter() - started
    processing_started = time.perf_counter()
    reports, errors = [], []
    interrupted = False
    for index, (file_id, path) in enumerate(items, 1):
        try:
            synchronize(device)
            image_started = time.perf_counter()
            with Image.open(path) as source:
                image = ImageOps.exif_transpose(source).convert("RGB")
            inputs = processor(text=TASK, images=image, return_tensors="pt").to(device, dtype)
            with torch.inference_mode():
                generated = model.generate(**inputs, max_new_tokens=args.max_new_tokens,
                                           num_beams=args.beams, do_sample=False)
            tokens = generated.cpu()
            raw = processor.batch_decode(tokens, skip_special_tokens=False)[0]
            parsed = processor.post_process_generation(raw, task=TASK, image_size=image.size)
            caption = parsed[TASK].strip()
            if not caption:
                raise ValueError("Model returned an empty caption")
            eos = model.generation_config.eos_token_id
            eos_ids = set(eos) if isinstance(eos, list) else {eos}
            hit_limit = tokens.shape[-1] >= args.max_new_tokens + 1 and int(tokens[0, -1]) not in eos_ids
            elapsed = time.perf_counter() - image_started
            report = {"file_id": file_id, "path": str(path), "caption_en": caption,
                      "elapsed_seconds": round(elapsed, 4), "possible_truncation": hit_limit,
                      "task": TASK, "model_id": MODEL_ID}
            write_json(output / "photos" / f"{file_id}.json", report)
            reports.append(report)
            print(f"[{index}/{len(items)}] {path.name} ({elapsed:.3f}s): {caption}", flush=True)
        except KeyboardInterrupt:
            interrupted = True
            print("Interrupted; saving completed results.", flush=True)
            break
        except Exception as error:
            errors.append({"file_id": file_id, "path": str(path), "error": f"{type(error).__name__}: {error}"})
            print(f"[{index}/{len(items)}] ERROR {path.name}: {error}", flush=True)
    processing_seconds = time.perf_counter() - processing_started
    write_json(output / "captions.json", reports)
    write_json(output / "errors.json", errors)
    write_json(output / "translation_payloads.json", [
        {"file_id": report["file_id"], "caption_en": report["caption_en"]} for report in reports
    ])
    with (output / "captions.csv").open("w", encoding="utf-8-sig", newline="") as stream:
        writer = csv.DictWriter(stream, fieldnames=["file_id", "path", "caption_en", "elapsed_seconds", "possible_truncation"])
        writer.writeheader()
        writer.writerows({key: report[key] for key in writer.fieldnames} for report in reports)
    summary = {
        "input": str(folder), "output": str(output), "device": device, "dtype": str(dtype),
        "gpu": torch.cuda.get_device_name(0) if device == "cuda" else None,
        "model_id": MODEL_ID, "model_revision": getattr(model.config, "_commit_hash", None),
        "task": TASK, "num_beams": args.beams, "max_new_tokens": args.max_new_tokens,
        "pilot": args.pilot, "requested": len(items), "completed": len(reports), "failed": len(errors),
        "interrupted": interrupted, "model_load_seconds": round(load_seconds, 3),
        "processing_seconds": round(processing_seconds, 3),
        "total_seconds": round(time.perf_counter() - started, 3),
        "average_image_seconds": round(sum(report["elapsed_seconds"] for report in reports) / len(reports), 3) if reports else None,
        "possible_truncation_ids": [report["file_id"] for report in reports if report["possible_truncation"]],
        "versions": {package: version(package) for package in ["torch", "transformers", "Pillow"]},
        "gemini_requests_sent": 0,
        "accuracy": "Not measured; captions require visual review."
    }
    write_json(output / "summary.json", summary)
    print("\n" + json.dumps(summary, ensure_ascii=False, indent=2), flush=True)
    return 130 if interrupted else 1 if errors else 0


if __name__ == "__main__":
    raise SystemExit(main())

"""Download a reproducible sample of 1,000 distinct COCO validation photos."""

import csv
import hashlib
import json
import random
import time
import urllib.request
import zipfile
from pathlib import Path

from PIL import Image


ROOT = Path(__file__).resolve().parents[1]
OUTPUT = ROOT / "test-images" / "coco-val2017-1000"
CACHE = ROOT / ".download-cache"
ARCHIVE = CACHE / "val2017.zip"
URL = "http://images.cocodataset.org/zips/val2017.zip"
SEED = 20261022
COUNT = 1000


def finalize_download(temporary):
    # Windows scanners can briefly hold a completed large archive open.
    for attempt in range(60):
        try:
            temporary.replace(ARCHIVE)
            return
        except OSError:
            if attempt == 59:
                raise
            time.sleep(1)


def download():
    CACHE.mkdir(parents=True, exist_ok=True)
    if ARCHIVE.is_file() and zipfile.is_zipfile(ARCHIVE):
        print("Using cached archive", flush=True)
        return
    temporary = ARCHIVE.with_suffix(".zip.part")
    if temporary.is_file() and zipfile.is_zipfile(temporary):
        print("Reusing completed temporary archive", flush=True)
        finalize_download(temporary)
        return
    for attempt in range(1, 5):
        try:
            request = urllib.request.Request(URL, headers={"User-Agent": "PhotoBackupManager-TestDataset/1.0"})
            with urllib.request.urlopen(request, timeout=120) as response, temporary.open("wb") as target:
                total = int(response.headers.get("Content-Length", 0))
                received = 0
                last_report = 0
                while chunk := response.read(1024 * 1024):
                    target.write(chunk)
                    received += len(chunk)
                    now = time.monotonic()
                    if now - last_report >= 5:
                        print(f"Download: {received / 1048576:.1f}/{total / 1048576:.1f} MiB", flush=True)
                        last_report = now
            if total and received != total:
                raise OSError(f"Incomplete download: {received}/{total}")
            if not zipfile.is_zipfile(temporary):
                raise OSError("Response is not a valid ZIP archive")
            finalize_download(temporary)
            return
        except Exception as error:
            print(f"Attempt {attempt}: {type(error).__name__}: {error}", flush=True)
            if attempt == 4:
                raise
            time.sleep(attempt * 2)


def main():
    started = time.monotonic()
    download()
    OUTPUT.mkdir(parents=True, exist_ok=True)
    records = []
    seen_hashes = set()
    with zipfile.ZipFile(ARCHIVE) as archive:
        names = sorted(name for name in archive.namelist()
                       if name.startswith("val2017/") and name.endswith(".jpg"))
        random.Random(SEED).shuffle(names)
        for name in names:
            content = archive.read(name)  # Includes ZIP CRC verification.
            digest = hashlib.sha256(content).hexdigest()
            if digest in seen_hashes:
                continue
            filename = Path(name).name
            if not filename.removesuffix(".jpg").isdigit():
                raise ValueError(f"Unexpected image name: {name}")
            destination = OUTPUT / filename
            if destination.exists() and hashlib.sha256(destination.read_bytes()).hexdigest() != digest:
                raise FileExistsError(f"Refusing to overwrite different file: {destination}")
            if not destination.exists():
                destination.write_bytes(content)
            with Image.open(destination) as image:
                image.load()
                width, height = image.size
                if image.format != "JPEG":
                    raise ValueError(f"Unexpected format: {destination}")
            seen_hashes.add(digest)
            records.append({
                "file_id": "coco_val2017_" + destination.stem,
                "filename": filename,
                "coco_image_id": int(destination.stem),
                "source_url": "http://images.cocodataset.org/" + name,
                "width": width,
                "height": height,
                "bytes": len(content),
                "sha256": digest,
            })
            if len(records) % 100 == 0:
                print(f"Verified photos: {len(records)}/{COUNT}", flush=True)
            if len(records) == COUNT:
                break
    assert len(records) == COUNT and len(seen_hashes) == COUNT
    actual = list(OUTPUT.glob("*.jpg"))
    assert len(actual) == COUNT, f"Expected exactly {COUNT} JPGs, found {len(actual)}"
    with (OUTPUT / "manifest.csv").open("w", encoding="utf-8-sig", newline="") as target:
        writer = csv.DictWriter(target, fieldnames=list(records[0]))
        writer.writeheader()
        writer.writerows(records)
    total_bytes = sum(record["bytes"] for record in records)
    summary = {
        "dataset": "COCO 2017 validation",
        "count": len(records),
        "unique_sha256": len(seen_hashes),
        "sampling": "Seeded shuffle, skipping byte-identical duplicates; not category-balanced",
        "seed": SEED,
        "bytes": total_bytes,
        "archive_url": URL,
        "source": "https://cocodataset.org/#download",
        "terms": "https://cocodataset.org/#termsofuse",
        "note": "Photos retain their individual source licenses; no blanket CC0 license is claimed. COCO is used by YOLO; this sample is not an independent generalization benchmark.",
    }
    (OUTPUT / "dataset_info.json").write_text(json.dumps(summary, indent=2), encoding="utf-8")
    (OUTPUT / "README.txt").write_text(
        "COCO 2017 validation: 1,000 distinct JPG photos for local application testing.\n"
        "Source: https://cocodataset.org/#download\n"
        "Terms: https://cocodataset.org/#termsofuse\n"
        "See manifest.csv for original image IDs, URLs, SHA-256 hashes and dimensions.\n"
        "Sample is randomly mixed, not balanced by category.\n"
        "Individual photos retain their respective licenses.\n"
        "This dataset is familiar to COCO-trained detectors; use other photos for independent evaluation.\n",
        encoding="utf-8",
    )
    print(json.dumps({"output": str(OUTPUT), "count": len(records), "unique": len(seen_hashes),
                      "size_mib": round(total_bytes / 1048576, 2),
                      "elapsed_seconds": round(time.monotonic() - started, 1)}, indent=2), flush=True)


if __name__ == "__main__":
    main()

"""Prepare text-only translation batches from a Florence run; no API calls."""

import argparse
import json
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]


def prepare(run, batch_size):
    if batch_size < 1:
        raise ValueError("Batch size must be positive")
    payloads = json.loads((run / "translation_payloads.json").read_text(encoding="utf-8-sig"))
    if not isinstance(payloads, list) or not payloads:
        raise ValueError("Expected a nonempty array of translation payloads")
    seen = set()
    for item in payloads:
        if not isinstance(item, dict) or set(item) != {"file_id", "caption_en"}:
            raise ValueError("Each payload must contain only file_id and caption_en")
        file_id = item["file_id"]
        if not isinstance(file_id, str) or not file_id.strip() or file_id in seen:
            raise ValueError("Invalid or duplicate file_id")
        seen.add(file_id)
        if item["caption_en"] is not None and not isinstance(item["caption_en"], str):
            raise ValueError(f"Invalid caption_en for {file_id}")
    prompt = (ROOT / "docs/GEMINI-TRANSLATE-CAPTIONS.txt").read_text(encoding="utf-8")
    output = run / "translation_batches"
    output.mkdir(exist_ok=False)
    count = 0
    for start in range(0, len(payloads), batch_size):
        count += 1
        data = json.dumps(payloads[start:start + batch_size], ensure_ascii=False, indent=2)
        (output / f"batch_{count:03d}.json").write_text(data, encoding="utf-8")
        (output / f"batch_{count:03d}_prompt.txt").write_text(prompt + "\n" + data, encoding="utf-8")
    summary = {"items": len(payloads), "batches": count, "batch_size": batch_size,
               "output": str(output), "gemini_requests_sent": 0,
               "note": "Translation preserves source claims; it does not verify image accuracy."}
    (output / "preparation_summary.json").write_text(json.dumps(summary, indent=2), encoding="utf-8")
    return summary


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--run", required=True, type=Path)
    parser.add_argument("--batch-size", default=50, type=int)
    args = parser.parse_args()
    print(json.dumps(prepare(args.run.resolve(), args.batch_size), ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()

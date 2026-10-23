"""Validate response structure and IDs; this does not verify image accuracy."""

import argparse
from collections import Counter
import json
from pathlib import Path


def validate(request, response):
    if not isinstance(request, list) or not isinstance(response, list):
        raise ValueError("Request and response must both be JSON arrays")
    expected = {}
    for item in request:
        if not isinstance(item, dict):
            raise ValueError("Request entries must be objects")
        file_id = item["file_id"]
        if not isinstance(file_id, str) or file_id in expected:
            raise ValueError("Request contains an invalid or duplicate file_id")
        if "accepted" in item and "caption_en" in item:
            raise ValueError("Request cannot mix accepted and caption_en in one entry")
        if "accepted" in item:
            accepted = item["accepted"]
            if not isinstance(accepted, dict):
                raise ValueError("accepted must be an object")
            empty = not accepted.get("objects") and not accepted.get("keywords")
        elif "caption_en" in item:
            caption_en = item["caption_en"]
            if caption_en is not None and not isinstance(caption_en, str):
                raise ValueError("caption_en must be text or null")
            empty = caption_en is None or not caption_en.strip()
        else:
            raise ValueError("Request requires accepted or caption_en")
        expected[file_id] = empty
    errors, ids, null_ids = [], [], []
    for index, item in enumerate(response):
        if not isinstance(item, dict) or set(item) != {"file_id", "caption"}:
            errors.append(f"Response item {index} must contain only file_id and caption")
            continue
        file_id = item["file_id"]
        if not isinstance(file_id, str):
            errors.append(f"Response item {index} has a non-string file_id")
            continue
        ids.append(file_id)
        caption = item["caption"]
        if caption is None:
            null_ids.append(file_id)
        elif not isinstance(caption, str) or not caption.strip():
            errors.append(f"{file_id}: caption must be a nonempty string or null")
        if file_id in expected:
            if expected[file_id] and caption is not None:
                errors.append(f"{file_id}: empty source requires caption null")
    duplicates = sorted(key for key, count in Counter(ids).items() if count > 1)
    missing = sorted(set(expected) - set(ids))
    unexpected = sorted(set(ids) - set(expected))
    if duplicates:
        errors.append(f"Duplicate IDs: {duplicates}")
    if missing:
        errors.append(f"Missing IDs: {missing}")
    if unexpected:
        errors.append(f"Unexpected IDs: {unexpected}")
    return {
        "structure_and_ids_valid": not errors,
        "requested": len(expected), "received": len(response),
        "caption_strings": sum(isinstance(item, dict) and isinstance(item.get("caption"), str)
                               and bool(item["caption"].strip()) for item in response),
        "null_ids": null_ids, "errors": errors,
        "semantic_and_image_accuracy": "Not verified; requires separate review",
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--request", type=Path, required=True)
    parser.add_argument("--response", type=Path, required=True)
    parser.add_argument("--report", type=Path)
    args = parser.parse_args()
    try:
        request = json.loads(args.request.read_text(encoding="utf-8-sig"))
        response = json.loads(args.response.read_text(encoding="utf-8-sig"))
        report = validate(request, response)
    except (ValueError, KeyError, TypeError) as error:
        report = {"structure_and_ids_valid": False, "errors": [str(error)]}
    if args.report:
        destination = args.report.resolve()
        if destination in {args.request.resolve(), args.response.resolve()}:
            parser.error("Report must not overwrite the request or response")
        destination.parent.mkdir(parents=True, exist_ok=True)
        destination.write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps(report, ensure_ascii=False, indent=2))
    return 0 if report["structure_and_ids_valid"] else 1


if __name__ == "__main__":
    raise SystemExit(main())

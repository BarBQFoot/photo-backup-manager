"""Compare real Florence image batches locally; no Gemini, downloads or app cache writes."""
import argparse
import gc
import json
from datetime import datetime
from pathlib import Path
import statistics
import sys
import time

from app_caption_worker import Captioner, SETTINGS


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--input', required=True, type=Path)
    parser.add_argument('--limit', type=int, default=100)
    parser.add_argument('--batches', nargs='+', type=int, default=[4, 6, 8, 10])
    parser.add_argument('--rounds', type=int, default=2)
    args = parser.parse_args()
    if args.limit < 1 or args.rounds < 1 or any(size < 1 for size in args.batches):
        parser.error('Limit, rounds and batch sizes must be positive')
    if not args.input.is_dir():
        parser.error('Input must be an existing image folder')
    paths = sorted(p.resolve() for p in args.input.iterdir()
                   if p.is_file() and p.suffix.lower() in {'.jpg', '.jpeg', '.png', '.webp', '.bmp'})[:args.limit]
    sizes = list(dict.fromkeys(args.batches))
    if len(paths) < max(sizes):
        parser.error('Input must contain at least as many images as the largest batch size')
    output = Path(__file__).resolve().parents[1] / 'results' / (
        'florence_batches_' + datetime.now().strftime('%Y%m%d_%H%M%S_%f'))
    output.mkdir(parents=True)
    started = time.perf_counter()
    load_started = time.perf_counter()
    captioner = Captioner()
    torch = captioner.torch

    def sync():
        if captioner.device == 'cuda':
            torch.cuda.synchronize()
        elif captioner.device == 'mps':
            torch.mps.synchronize()

    def clear():
        gc.collect()
        if captioner.device == 'cuda':
            torch.cuda.empty_cache()
        elif captioner.device == 'mps':
            torch.mps.empty_cache()

    def generate(size):
        rows = []
        for offset in range(0, len(paths), size):
            chunk = paths[offset:offset + size]
            # Direct generation deliberately avoids the app's automatic OOM batch reduction.
            captions = captioner._caption_many_once(chunk)
            if len(captions) != len(chunk):
                raise ValueError('Caption count does not match image count')
            rows.extend({'path': str(path), 'caption': caption}
                        for path, caption in zip(chunk, captions))
        return rows

    sync()
    report = {'input': str(args.input.resolve()), 'images': len(paths), 'device': captioner.device,
              'gpu': torch.cuda.get_device_name() if captioner.device == 'cuda' else None,
              'dtype': str(captioner.dtype), 'settings': SETTINGS,
              'model_load_seconds': round(time.perf_counter() - load_started, 3),
              'results': [], 'gemini_requests_sent': 0,
              'note': 'Timed work includes image loading, preprocessing, generation and decoding. '
                      'Model loading and warmup excluded. Caption accuracy not measured.'}
    print(f'Device: {report["device"]}; images: {len(paths)}; output: {output}', flush=True)
    failed = set()
    cleanup_failed = False
    for round_index in range(args.rounds):
        # Reverse the second round to reduce systematic first/last ordering effects.
        for size in sizes if round_index % 2 == 0 else reversed(sizes):
            if cleanup_failed:
                break
            if size in failed:
                continue
            print(f'Round {round_index + 1}/{args.rounds}: batch {size}', flush=True)
            record = {'round': round_index + 1, 'batch_size': size}
            try:
                clear()
                captioner._caption_many_once(paths[:size])  # Warm up this exact shape.
                sync()
                if captioner.device == 'cuda':
                    torch.cuda.reset_peak_memory_stats()
                tick = time.perf_counter()
                rows = generate(size)
                sync()
                seconds = time.perf_counter() - tick
                record.update(status='ok', completed=len(rows), seconds=round(seconds, 3),
                              images_per_second=round(len(rows) / seconds, 3))
                if captioner.device == 'cuda':
                    record['peak_allocated_mb'] = round(torch.cuda.max_memory_allocated() / 2**20, 1)
                    record['peak_reserved_mb'] = round(torch.cuda.max_memory_reserved() / 2**20, 1)
                (output / f'batch{size}_round{round_index + 1}_captions.json').write_text(
                    json.dumps(rows, ensure_ascii=False, indent=2), encoding='utf-8')
            except Exception as error:
                failed.add(size)
                record.update(status='failed', error=f'{type(error).__name__}: {error}')
            # Outside the exception handler so GPU tensors in tracebacks can be released.
            try:
                clear()
            except Exception as error:
                # CUDA can remain out of memory even during empty_cache(). Preserve
                # the failed result and stop instead of losing the report to a traceback.
                record['cleanup_error'] = f'{type(error).__name__}: {error}'
                record['status'] = 'failed'
                failed.add(size)
                cleanup_failed = True
            report['results'].append(record)
            print(json.dumps(record, ensure_ascii=False), flush=True)
            (output / 'summary.json').write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding='utf-8')
    report['median_seconds'] = {
        str(size): round(statistics.median(r['seconds'] for r in report['results']
                                         if r['batch_size'] == size and r['status'] == 'ok'), 3)
        for size in sizes if size not in failed and any(
            r['batch_size'] == size and r['status'] == 'ok' for r in report['results'])
    }
    report['fastest_batch'] = (int(min(report['median_seconds'], key=report['median_seconds'].get))
                               if report['median_seconds'] else None)
    report['total_seconds'] = round(time.perf_counter() - started, 3)
    report['stopped_after_cleanup_error'] = cleanup_failed
    (output / 'summary.json').write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding='utf-8')
    print(json.dumps({'fastest_batch': report['fastest_batch'], 'median_seconds': report['median_seconds'],
                      'summary': str(output / 'summary.json'), 'gemini_requests_sent': 0}, ensure_ascii=False), flush=True)
    return 0 if report['fastest_batch'] is not None else 1


if __name__ == '__main__':
    if hasattr(sys.stdout, 'reconfigure'):
        sys.stdout.reconfigure(encoding='utf-8')
    raise SystemExit(main())

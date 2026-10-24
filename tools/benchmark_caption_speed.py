"""Compare local Florence beam/batch settings. No Gemini calls or application DB/cache writes."""
import argparse
from datetime import datetime
import json
import statistics
from pathlib import Path
import time

if __package__:
    from . import app_caption_worker as worker
else:
    import app_caption_worker as worker


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--input', type=Path, required=True)
    parser.add_argument('--limit', type=int, default=12)
    parser.add_argument('--repeats', type=int, default=3)
    args = parser.parse_args()
    if args.limit < 1 or args.repeats < 1 or not args.input.is_dir():
        parser.error('Input must be an existing folder and limit must be positive')
    paths = sorted(path for path in args.input.rglob('*')
                   if path.is_file() and path.suffix.lower() in {'.jpg', '.jpeg', '.png', '.webp'})[:args.limit]
    if not paths:
        parser.error('No images found')
    started = time.perf_counter()
    captioner = worker.Captioner()
    model_seconds = time.perf_counter() - started
    torch = captioner.torch

    def sync():
        if captioner.device == 'cuda':
            torch.cuda.synchronize()
        elif captioner.device == 'mps':
            torch.mps.synchronize()

    reports = []
    for beams, batch_size in ((3, 1), (1, 1), (1, 2), (1, 4)):
        worker.SETTINGS['beams'] = beams
        captioner.batch_size = batch_size
        captioner.caption_many(paths[:batch_size])
        sync()
        if captioner.device == 'cuda':
            torch.cuda.reset_peak_memory_stats()
        trials = []
        for _ in range(args.repeats):
            tick = time.perf_counter()
            captions = []
            for start in range(0, len(paths), batch_size):
                captions.extend(captioner.caption_many(paths[start:start + batch_size]))
            sync()
            trials.append(time.perf_counter() - tick)
        seconds = statistics.median(trials)
        report = {'num_beams': beams, 'batch_size_requested': batch_size,
                  'batch_size_final': captioner.batch_size, 'images': len(paths),
                  'seconds': round(seconds, 3), 'seconds_per_image': round(seconds / len(paths), 4),
                  'trial_seconds': [round(trial, 3) for trial in trials],
                  'peak_gpu_mb': round(torch.cuda.max_memory_allocated() / 1048576, 1) if captioner.device == 'cuda' else None,
                  'captions': [{'file': path.name, 'caption': caption} for path, caption in zip(paths, captions)]}
        reports.append(report)
        print(json.dumps({key: value for key, value in report.items() if key != 'captions'}), flush=True)
    output = Path(__file__).resolve().parents[1] / 'results' / 'performance' / ('florence_' + datetime.now().strftime('%Y%m%d_%H%M%S') + '.json')
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps({'device': captioner.device, 'model_load_seconds': round(model_seconds, 3),
                                 'torch': torch.__version__, 'results': reports,
                                 'note': 'Warm model; local captioning only. Thai/API latency and caption accuracy are not measured.'},
                                ensure_ascii=False, indent=2), encoding='utf-8')
    print(f'Saved: {output}', flush=True)


if __name__ == '__main__':
    main()

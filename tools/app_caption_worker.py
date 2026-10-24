"""Desktop bridge: JSON request on stdin, JSON events on stdout; diagnostics on stderr.

Only English captions and stable request IDs are sent to Gemini. Files and keys
never appear in the protocol output. Successful translations are cached by image
content, caption settings, translation model and prompt, independently of DB IDs.
"""

from contextlib import redirect_stdout
from concurrent.futures import ThreadPoolExecutor
import hashlib
import json
import os
from pathlib import Path
import re
import statistics
import sys
import threading
import time

if __package__:
    from .translate_captions import (DEFAULT_MODEL, ROOT, execute, fingerprint,
                                    load_key, read_catalog, run_lock, write_json)
    from .prepare_translation import prepare
else:
    from translate_captions import (DEFAULT_MODEL, ROOT, execute, fingerprint,
                                    load_key, read_catalog, run_lock, write_json)
    from prepare_translation import prepare

SETTINGS = {'model': 'florence-community/Florence-2-base-ft', 'task': '<CAPTION>',
            'beams': 1, 'max_new_tokens': 64, 'cache_version': 2}
TRANSLATION_BATCH_SIZE = 250
TRANSLATION_INTERVAL_SECONDS = 10
TRANSLATION_MAX_OUTPUT_TOKENS = 16384
CAPTION_BATCH_SIZE = 4
_output_lock = threading.Lock()


class SetupError(ValueError):
    pass


class ResidentCaptioner:
    """One Florence instance for the lifetime of the desktop worker process."""
    def __init__(self, make_captioner=None):
        self.make_captioner = make_captioner or Captioner
        self.instance = None
        self.last_reused = False

    def __call__(self):
        self.last_reused = self.instance is not None
        if self.instance is None:
            self.instance = self.make_captioner()
        return self.instance


def emit(**value):
    with _output_lock:
        sys.__stdout__.write(json.dumps(value, ensure_ascii=False) + '\n')
        sys.__stdout__.flush()


def image_hash(path):
    with path.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def trim_generated_tokens(rows, eos, max_new_tokens):
    sequences = []
    for row in rows:
        end = next((index for index in range(1, len(row)) if row[index] in eos), None)
        if end is None and len(row) >= max_new_tokens + 1:
            raise ValueError('Caption may be truncated; image needs review')
        sequences.append(row[:end + 1] if end is not None else row)
    return sequences


class Captioner:
    def __init__(self):
        import torch
        from transformers import AutoProcessor, Florence2ForConditionalGeneration
        self.torch = torch
        self.device = 'cuda' if torch.cuda.is_available() else 'mps' if torch.backends.mps.is_available() else 'cpu'
        self.dtype = torch.float16 if self.device == 'cuda' else torch.float32
        self.batch_size = 1
        self.processor = AutoProcessor.from_pretrained(SETTINGS['model'], local_files_only=True)
        self.model = Florence2ForConditionalGeneration.from_pretrained(
            SETTINGS['model'], dtype=self.dtype, local_files_only=True).to(self.device).eval()

    def configure_batch(self, paths, cache, total):
        override = os.environ.get('PHOTO_AI_CAPTION_BATCH_SIZE', 'auto').strip().lower()
        if override not in ('auto', '1', '2', '4'):
            raise SetupError('PHOTO_AI_CAPTION_BATCH_SIZE must be auto, 1, 2 or 4')
        if self.device == 'cpu':
            self.batch_size = 1
            return
        if override != 'auto':
            self.batch_size = int(override)
            return
        # Small jobs avoid calibration overhead. Large jobs reuse a profile for this GPU/runtime.
        if total < 100 or len(paths) < 2:
            return
        gpu = self.torch.cuda.get_device_name() if self.device == 'cuda' else self.device
        identity = {'device': gpu, 'torch': self.torch.__version__, 'dtype': str(self.dtype), **SETTINGS}
        profile = cache / ('batch_profile_' + fingerprint(identity) + '.json')
        try:
            saved = json.loads(profile.read_text(encoding='utf-8'))
            if saved.get('identity') == identity and saved.get('batch_size') in (1, 2, 4):
                self.batch_size = saved['batch_size']
                return
        except (OSError, ValueError):
            pass
        emit(type='progress', phase='pipeline', detail='กำลังเทียบขนาดชุดอ่านภาพ 1, 2 และ 4 รูป', total=total)

        def sync():
            if self.device == 'cuda':
                self.torch.cuda.synchronize()
            elif self.device == 'mps':
                self.torch.mps.synchronize()

        measurements = {}
        try:
            self.caption_many(paths[:1])
            sync()
            for size in (1, 2, 4):
                self.batch_size = size
                self.caption_many(paths)
                sync()
                times = []
                for _ in range(2):
                    tick = time.perf_counter()
                    self.caption_many(paths)
                    sync()
                    times.append(time.perf_counter() - tick)
                if self.batch_size == size:
                    measurements[size] = statistics.median(times)
            self.batch_size = min(measurements, key=measurements.get)
            if measurements[self.batch_size] >= measurements[1] * 0.9:
                self.batch_size = 1
            write_json(profile, {'identity': identity, 'batch_size': self.batch_size,
                                 'seconds_per_sample': measurements})
        except Exception:
            # Calibration is optional; a bad sample must not stop the actual caption job.
            self.batch_size = 1

    def caption(self, path):
        return self.caption_many([path])[0]

    def caption_many(self, paths):
        if len(paths) > self.batch_size:
            size = self.batch_size
            return [caption for start in range(0, len(paths), size)
                    for caption in self.caption_many(paths[start:start + size])]
        try:
            return self._caption_many_once(paths)
        except RuntimeError as error:
            if len(paths) == 1 or 'out of memory' not in str(error).lower():
                raise
        # Retry outside the exception handler so its traceback no longer retains GPU tensors.
        self.batch_size = max(1, len(paths) // 2)
        if self.device == 'cuda':
            self.torch.cuda.empty_cache()
        elif self.device == 'mps':
            self.torch.mps.empty_cache()
        return self.caption_many(paths)

    def _caption_many_once(self, paths):
        from PIL import Image, ImageOps
        images = []
        for path in paths:
            with Image.open(path) as source:
                images.append(ImageOps.exif_transpose(source).convert('RGB'))
        inputs = self.processor(text=[SETTINGS['task']] * len(images), images=images,
                                padding=True, return_tensors='pt').to(self.device, self.dtype)
        with self.torch.inference_mode():
            generated = self.model.generate(**inputs, max_new_tokens=SETTINGS['max_new_tokens'],
                                            num_beams=SETTINGS['beams'], do_sample=False)
        tokens = generated.cpu()
        eos = self.model.generation_config.eos_token_id
        eos = set(eos) if isinstance(eos, list) else {eos}
        captions = []
        sequences = trim_generated_tokens(tokens.tolist(), eos, SETTINGS['max_new_tokens'])
        decoded = self.processor.batch_decode(sequences, skip_special_tokens=False)
        if len(decoded) != len(paths):
            raise ValueError('Caption batch size does not match input images')
        for raw, image in zip(decoded, images):
            caption = self.processor.post_process_generation(raw, task=SETTINGS['task'], image_size=image.size)[SETTINGS['task']].strip()
            if not caption:
                raise ValueError('Empty caption')
            captions.append(caption)
        return captions


def process(request, make_captioner=Captioner):
    items = request.get('items')
    if not isinstance(items, list) or not items or len(items) > 10000:
        raise ValueError('Expected 1 to 10000 photos')
    seen = set()
    for item in items:
        fid = item.get('file_id', '')
        if not re.fullmatch(r'file_[1-9][0-9]*', fid) or fid in seen:
            raise ValueError('Invalid or duplicated file ID')
        seen.add(fid)
        path = Path(item['path'])
        if not path.is_absolute() or not path.is_file():
            raise ValueError('Image file is missing')
    model = request.get('model', DEFAULT_MODEL)
    if not re.fullmatch(r'[A-Za-z0-9._-]+', model):
        raise ValueError('Invalid translation model')
    prompt = (ROOT / 'docs/GEMINI-TRANSLATE-CAPTIONS.txt').read_text(encoding='utf-8')
    cache = Path(request['cache_dir']).resolve()
    cache.mkdir(parents=True, exist_ok=True)
    # One local model/GPU job at a time, including across app instances.
    with run_lock(cache):
        return process_locked(items, model, prompt, cache, make_captioner)


def process_locked(items, model, prompt, cache, make_captioner):
    captioner = None
    pending = []
    total = len(items)
    counts = {'captioned': 0, 'translated': 0}
    count_lock = threading.Lock()
    translation_errors = []
    metrics = {'caption_batch_size_requested': CAPTION_BATCH_SIZE, 'num_beams': SETTINGS['beams'],
               'total': total, 'caption_seconds': 0.0, 'translation_seconds': 0.0,
               'translation_pause_seconds': 0.0, 'model_load_seconds': 0.0,
               'caption_cache_hits': 0, 'thai_cache_hits': 0}
    started = time.perf_counter()

    def progress(captioned=0, translated=0):
        with count_lock:
            counts['captioned'] += captioned
            counts['translated'] += translated
            emit(type='progress', phase='pipeline', completed=counts['translated'], total=total, **counts)

    def failed(fid):
        emit(type='failure', file_id=fid, message='Local caption failed. Check Python, cached Florence model, and image format.')
        progress(captioned=1)

    # Only this thread calls Gemini. CPU/GPU captioning continues on the main thread.
    # Stop sending after the first translation error; still preserve later English captions.
    last_translation_finished = None

    def translate(group):
        nonlocal last_translation_finished
        if translation_errors:
            return
        try:
            key = load_key()
            if not key:
                raise ValueError('GEMINI_API_KEY is not configured; English captions are preserved')
            payload = [{'file_id': fid, 'caption_en': caption} for fid, caption, _, _ in group]
            targets = {fid: (cache_key, path) for fid, _, cache_key, path in group}
            run = cache / ('run_' + fingerprint({'payloads': payload, 'model': model, 'prompt': prompt,
                                               'batch_size': TRANSLATION_BATCH_SIZE, 'pipeline_version': 1}))
            run.mkdir(exist_ok=True)
            write_json(run / 'translation_payloads.json', payload)
            if not (run / 'translation_batches').exists():
                prepare(run, TRANSLATION_BATCH_SIZE)
            payloads, batches = read_catalog(run)
            if last_translation_finished is not None:
                delay = max(0.0, TRANSLATION_INTERVAL_SECONDS - (time.monotonic() - last_translation_finished))
                if delay:
                    time.sleep(delay)
                    metrics['translation_pause_seconds'] += delay

            def on_saved(response):
                for row in response:
                    fid, caption = row['file_id'], row['caption']
                    if not isinstance(caption, str) or not caption.strip():
                        emit(type='failure', file_id=fid, message='Translation needs review')
                        continue
                    cache_key, path = targets[fid]
                    write_json(path, {'cache_key': cache_key, 'caption': caption})
                    emit(type='result', file_id=fid, caption=caption)
                    progress(translated=1)

            translation_started = time.perf_counter()
            try:
                code = execute(run, payloads, batches, key, model, prompt, None,
                               TRANSLATION_INTERVAL_SECONDS, 1, TRANSLATION_MAX_OUTPUT_TOKENS, 300, on_saved=on_saved)
            finally:
                metrics['translation_seconds'] += time.perf_counter() - translation_started
                last_translation_finished = time.monotonic()
            if code:
                summary = json.loads((run / 'translation_summary.json').read_text(encoding='utf-8'))
                raise ValueError(summary.get('error') or 'Translation stopped; completed results preserved')
        except Exception as error:
            translation_errors.append(error)

    with ThreadPoolExecutor(max_workers=1, thread_name_prefix='caption-translation') as executor:
        def accept(item, caption_key, caption):
            fid = item['file_id']
            translation_key = fingerprint({'caption_key': caption_key, 'caption_en': caption,
                                           'model': model, 'prompt': prompt})
            translated_path = cache / ('thai_' + translation_key + '.json')
            saved = json.loads(translated_path.read_text(encoding='utf-8')) if translated_path.is_file() else {}
            translated = saved.get('caption')
            progress(captioned=1)
            if saved.get('cache_key') == translation_key and isinstance(translated, str) and translated.strip():
                metrics['thai_cache_hits'] += 1
                emit(type='result', file_id=fid, caption=translated)
                progress(translated=1)
            else:
                pending.append((fid, caption, translation_key, translated_path))
                if len(pending) >= TRANSLATION_BATCH_SIZE:
                    executor.submit(translate, pending.copy())
                    pending.clear()

        progress()
        for start in range(0, total, CAPTION_BATCH_SIZE):
            uncached = []
            for item in items[start:start + CAPTION_BATCH_SIZE]:
                path = Path(item['path'])
                try:
                    digest = image_hash(path)
                    caption_key = fingerprint({'sha256': digest, **SETTINGS})
                    caption_path = cache / ('caption_' + caption_key + '.json')
                    saved = json.loads(caption_path.read_text(encoding='utf-8')) if caption_path.is_file() else {}
                    caption = saved.get('caption_en') if saved.get('cache_key') == caption_key else None
                    if isinstance(caption, str) and caption.strip():
                        metrics['caption_cache_hits'] += 1
                        accept(item, caption_key, caption)
                    else:
                        uncached.append((item, path, digest, caption_key, caption_path))
                except Exception:
                    failed(item['file_id'])
            if not uncached:
                continue
            if captioner is None:
                load_started = time.perf_counter()
                try:
                    captioner = make_captioner()
                except Exception as error:
                    detail = ' '.join(str(error).split())[:1200]
                    diagnostic = {'python': sys.executable, 'model': SETTINGS['model'],
                                  'error_type': type(error).__name__, 'message': detail}
                    try:
                        write_json(cache / 'model_load_error.json', diagnostic)
                    except OSError:
                        pass
                    raise SetupError(f'Florence could not load ({type(error).__name__}): {detail}. Python: {sys.executable}') from None
                metrics['model_load_seconds'] = time.perf_counter() - load_started
                metrics['model_reused'] = bool(getattr(make_captioner, 'last_reused', False))
                if hasattr(captioner, 'configure_batch'):
                    tuning_started = time.perf_counter()
                    captioner.configure_batch([entry[1] for entry in uncached], cache, total)
                    metrics['batch_tuning_seconds'] = time.perf_counter() - tuning_started

            def save_caption(entry, caption):
                item, path, digest, caption_key, caption_path = entry
                if not isinstance(caption, str) or not caption.strip():
                    raise ValueError('Empty caption')
                if image_hash(path) != digest:
                    raise ValueError('Image changed during captioning; try again')
                write_json(caption_path, {'cache_key': caption_key, 'caption_en': caption})
                accept(item, caption_key, caption)

            caption_started = time.perf_counter()
            try:
                paths = [entry[1] for entry in uncached]
                if hasattr(captioner, 'caption_many'):
                    captions = captioner.caption_many(paths)
                else:
                    captions = [captioner.caption(path) for path in paths]
                if len(captions) != len(uncached):
                    raise ValueError('Caption batch size does not match input images')
            except Exception:
                # A corrupt image or a batch-specific backend failure must not lose its neighbours.
                captions = []
                for entry in uncached:
                    try:
                        captions.append(captioner.caption(entry[1]))
                    except Exception:
                        captions.append(None)
            for entry, caption in zip(uncached, captions):
                try:
                    save_caption(entry, caption)
                except Exception:
                    failed(entry[0]['file_id'])
            metrics['caption_seconds'] += time.perf_counter() - caption_started
        if pending:
            executor.submit(translate, pending.copy())
    metrics.update(counts, total_seconds=time.perf_counter() - started,
                   device=getattr(captioner, 'device', 'cache'),
                   caption_batch_size_final=getattr(captioner, 'batch_size', 0),
                   translation_failed=bool(translation_errors))
    write_json(cache / 'pipeline_summary.json', metrics)
    if translation_errors:
        raise ValueError(str(translation_errors[0]))


def handle_request(request, make_captioner=Captioner):
    try:
        with redirect_stdout(sys.stderr):
            process(request, make_captioner=make_captioner)
        return True
    except Exception as error:
        key = os.environ.get('GEMINI_API_KEY', '')
        message = str(error)
        if key:
            message = message.replace(key, '[REDACTED]')
        emit(type='error', message=message)
        return False
    finally:
        emit(type='done')


def main():
    if hasattr(sys.stdout, 'reconfigure'):
        sys.stdout.reconfigure(encoding='utf-8')
        sys.stderr.reconfigure(encoding='utf-8')
        sys.stdin.reconfigure(encoding='utf-8')
    if '--serve' in sys.argv[1:]:
        factory = ResidentCaptioner()
        emit(type='ready')
        for line in sys.stdin:
            try:
                request = json.loads(line)
            except ValueError:
                emit(type='error', message='Invalid local AI request')
                emit(type='done')
                continue
            if request.get('command') == 'shutdown':
                break
            handle_request(request, make_captioner=factory)
        return 0
    return 0 if handle_request(json.load(sys.stdin)) else 1


if __name__ == '__main__':
    raise SystemExit(main())

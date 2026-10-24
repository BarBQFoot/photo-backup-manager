"""Desktop bridge: JSON request on stdin, JSON events on stdout; diagnostics on stderr.

Only English captions and stable request IDs are sent to Gemini. Files and keys
never appear in the protocol output. Successful translations are cached by image
content, caption settings, translation model and prompt, independently of DB IDs.
"""

from contextlib import redirect_stdout
import hashlib
import json
import os
from pathlib import Path
import re
import sys

if __package__:
    from .translate_captions import (DEFAULT_MODEL, ROOT, execute, fingerprint,
                                    load_key, read_catalog, run_lock, write_json)
    from .prepare_translation import prepare
else:
    from translate_captions import (DEFAULT_MODEL, ROOT, execute, fingerprint,
                                    load_key, read_catalog, run_lock, write_json)
    from prepare_translation import prepare

SETTINGS = {'model': 'florence-community/Florence-2-base-ft', 'task': '<CAPTION>',
            'beams': 3, 'max_new_tokens': 64, 'cache_version': 1}
TRANSLATION_BATCH_SIZE = 100
TRANSLATION_INTERVAL_SECONDS = 10


class SetupError(ValueError):
    pass


def emit(**value):
    sys.__stdout__.write(json.dumps(value, ensure_ascii=False) + '\n')
    sys.__stdout__.flush()


def image_hash(path):
    with path.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


class Captioner:
    def __init__(self):
        if __package__:
            from .caption_images import torch, AutoProcessor, Florence2ForConditionalGeneration
        else:
            from caption_images import torch, AutoProcessor, Florence2ForConditionalGeneration
        self.torch = torch
        self.device = 'cuda' if torch.cuda.is_available() else 'mps' if torch.backends.mps.is_available() else 'cpu'
        self.dtype = torch.float16 if self.device == 'cuda' else torch.float32
        self.processor = AutoProcessor.from_pretrained(SETTINGS['model'], local_files_only=True)
        self.model = Florence2ForConditionalGeneration.from_pretrained(
            SETTINGS['model'], dtype=self.dtype, local_files_only=True).to(self.device).eval()

    def caption(self, path):
        from PIL import Image, ImageOps
        with Image.open(path) as source:
            image = ImageOps.exif_transpose(source).convert('RGB')
        inputs = self.processor(text=SETTINGS['task'], images=image, return_tensors='pt').to(self.device, self.dtype)
        with self.torch.inference_mode():
            generated = self.model.generate(**inputs, max_new_tokens=SETTINGS['max_new_tokens'],
                                            num_beams=SETTINGS['beams'], do_sample=False)
        tokens = generated.cpu()
        eos = self.model.generation_config.eos_token_id
        eos = set(eos) if isinstance(eos, list) else {eos}
        if tokens.shape[-1] >= SETTINGS['max_new_tokens'] + 1 and int(tokens[0, -1]) not in eos:
            raise ValueError('Caption may be truncated; image needs review')
        raw = self.processor.batch_decode(tokens, skip_special_tokens=False)[0]
        caption = self.processor.post_process_generation(raw, task=SETTINGS['task'], image_size=image.size)[SETTINGS['task']].strip()
        if not caption:
            raise ValueError('Empty caption')
        return caption


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
    translations = {}
    total = len(items)
    finished = 0
    for index, item in enumerate(items):
        emit(type='progress', phase='captioning', completed=index, total=total)
        fid, path = item['file_id'], Path(item['path'])
        try:
            digest = image_hash(path)
            caption_key = fingerprint({'sha256': digest, **SETTINGS})
            caption_path = cache / ('caption_' + caption_key + '.json')
            caption = None
            if caption_path.is_file():
                saved = json.loads(caption_path.read_text(encoding='utf-8'))
                if saved.get('cache_key') == caption_key:
                    caption = saved.get('caption_en')
            if not isinstance(caption, str) or not caption.strip():
                if captioner is None:
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
                caption = captioner.caption(path)
                if image_hash(path) != digest:
                    raise ValueError('Image changed during captioning; try again')
                write_json(caption_path, {'cache_key': caption_key, 'caption_en': caption})
            translation_key = fingerprint({'caption_key': caption_key, 'caption_en': caption,
                                           'model': model, 'prompt': prompt})
            translated_path = cache / ('thai_' + translation_key + '.json')
            saved = json.loads(translated_path.read_text(encoding='utf-8')) if translated_path.is_file() else {}
            translated = saved.get('caption')
            if saved.get('cache_key') == translation_key and isinstance(translated, str) and translated.strip():
                emit(type='result', file_id=fid, caption=translated)
                finished += 1
            else:
                pending.append({'file_id': fid, 'caption_en': caption})
                translations[fid] = (translation_key, translated_path)
        except SetupError:
            raise
        except Exception:
            # Model/cache errors must be recoverable per photo; no paths or traceback in UI.
            emit(type='failure', file_id=fid, message='Local caption failed. Check Python, cached Florence model, and image format.')
    emit(type='progress', phase='translating', completed=finished, total=total)
    if not pending:
        return
    key = load_key()
    if not key:
        raise ValueError('GEMINI_API_KEY is not configured; English captions are preserved')
    run = cache / ('run_' + fingerprint({'payloads': pending, 'model': model, 'prompt': prompt,
                                       'batch_size': TRANSLATION_BATCH_SIZE}))
    run.mkdir(exist_ok=True)
    write_json(run / 'translation_payloads.json', pending)
    if not (run / 'translation_batches').exists():
        prepare(run, TRANSLATION_BATCH_SIZE)
    payloads, batches = read_catalog(run)

    def on_saved(response):
        nonlocal finished
        for row in response:
            fid, caption = row['file_id'], row['caption']
            if not isinstance(caption, str) or not caption.strip():
                emit(type='failure', file_id=fid, message='Translation needs review')
                continue
            cache_key, path = translations[fid]
            write_json(path, {'cache_key': cache_key, 'caption': caption})
            emit(type='result', file_id=fid, caption=caption)
            finished += 1
        emit(type='progress', phase='translating', completed=finished, total=total)

    code = execute(run, payloads, batches, key, model, prompt, None,
                   TRANSLATION_INTERVAL_SECONDS, 1, 8192, 300, on_saved=on_saved)
    if code:
        summary = json.loads((run / 'translation_summary.json').read_text(encoding='utf-8'))
        raise ValueError(summary.get('error') or 'Translation stopped; completed results preserved')


def main():
    if hasattr(sys.stdout, 'reconfigure'):
        sys.stdout.reconfigure(encoding='utf-8')
        sys.stderr.reconfigure(encoding='utf-8')
        sys.stdin.reconfigure(encoding='utf-8')
    try:
        request = json.load(sys.stdin)
        # Hugging Face and the CLI translator may print diagnostics to stdout.
        with redirect_stdout(sys.stderr):
            process(request)
        emit(type='done')
        return 0
    except Exception as error:
        key = os.environ.get('GEMINI_API_KEY', '')
        message = str(error)
        if key:
            message = message.replace(key, '[REDACTED]')
        emit(type='error', message=message)
        return 1


if __name__ == '__main__':
    raise SystemExit(main())

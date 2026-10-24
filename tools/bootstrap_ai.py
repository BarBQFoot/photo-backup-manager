"""First-launch AI setup protocol. No photo access, DB writes or Gemini requests."""
import argparse
import hashlib
from contextlib import redirect_stdout
from importlib.metadata import PackageNotFoundError, version
import json
import sys
import threading
import time
from pathlib import Path

MODEL = 'florence-community/Florence-2-base-ft'
MODEL_FILES = ('config.json', 'model.safetensors', 'tokenizer.json', 'preprocessor_config.json')
MODEL_PATTERNS = ['*.json', '*.safetensors', '*.txt', '*.model', '*.tiktoken']
_output_lock = threading.Lock()


def emit(**event):
    with _output_lock:
        sys.__stdout__.write(json.dumps(event, ensure_ascii=False) + '\n')
        sys.__stdout__.flush()


def inspect_model(folder):
    folder = Path(folder)
    missing = [name for name in MODEL_FILES
               if not (folder / name).is_file() or (folder / name).stat().st_size == 0]
    error = ('ไฟล์ที่ขาดหรือว่าง: ' + ', '.join(missing)) if missing else ''
    return folder, missing, error


def check():
    if not (3, 11) <= sys.version_info[:2] <= (3, 13):
        emit(type='check', python_ready=False, dependencies_ready=False, model_ready=False)
        return
    versions = {}
    missing = []
    for package in ('torch', 'torchvision', 'transformers', 'Pillow', 'safetensors', 'huggingface_hub'):
        try:
            versions[package] = version(package)
        except PackageNotFoundError:
            missing.append(package)
    torch_ready = all(name in versions for name in ('torch', 'torchvision'))
    if missing or int(versions['transformers'].split('.')[0]) != 5:
        emit(type='check', python_ready=True, dependencies_ready=False, model_ready=False, torch_ready=torch_ready)
        return
    from huggingface_hub import snapshot_download
    ready = False
    model_path = ''
    model_error = ''
    try:
        folder, missing, model_error = inspect_model(snapshot_download(
            MODEL, local_files_only=True, allow_patterns=MODEL_PATTERNS))
        model_path = str(folder)
        ready = not missing
    except (OSError, ValueError) as error:
        from huggingface_hub.constants import HF_HUB_CACHE
        model_path = str(Path(HF_HUB_CACHE) / ('models--' + MODEL.replace('/', '--')))
        model_error = f'{type(error).__name__}: {error}'[:1500]
    signature = ''
    if ready:
        files = [(name, (folder / name).stat().st_size, (folder / name).stat().st_mtime_ns)
                 for name in MODEL_FILES]
        signature = hashlib.sha256(json.dumps([sys.executable, sys.version, versions, str(folder), files],
                                             sort_keys=True).encode()).hexdigest()
    emit(type='check', python_ready=True, dependencies_ready=True, torch_ready=True, model_ready=ready,
         signature=signature, model_path=model_path, model_error=model_error)


def download():
    from huggingface_hub import snapshot_download
    from tqdm.auto import tqdm

    class Progress(tqdm):
        def __init__(self, *args, **kwargs):
            self._sent_at = 0.0
            kwargs['disable'] = False
            super().__init__(*args, **kwargs)

        def display(self, *args, **kwargs):
            now = time.monotonic()
            if now - self._sent_at < 0.2 and self.n != self.total:
                return
            self._sent_at = now
            emit(type='progress', phase='model', detail=str(self.desc or 'Florence'),
                 completed=int(self.n or 0), total=int(self.total or 0),
                 unit='bytes' if self.unit == 'B' else 'files')

    from huggingface_hub.constants import HF_HUB_CACHE, HF_HUB_OFFLINE
    cache_path = Path(HF_HUB_CACHE) / ('models--' + MODEL.replace('/', '--'))
    emit(type='progress', phase='model', detail=f'ดาวน์โหลดจาก Hugging Face ไปที่ {cache_path}')
    if HF_HUB_OFFLINE:
        raise RuntimeError(f'HF_HUB_OFFLINE เปิดอยู่ จึงดาวน์โหลดไม่ได้\nตำแหน่งแคช: {cache_path}')
    try:
        folder, missing, error = inspect_model(snapshot_download(
            MODEL, token=False, max_workers=2, tqdm_class=Progress,
            allow_patterns=MODEL_PATTERNS))
    except Exception as error:
        raise RuntimeError(f'{type(error).__name__}: {error}\nตำแหน่งแคช: {cache_path}') from error
    if missing:
        raise RuntimeError(f'ดาวน์โหลด Florence ยังไม่ครบ\n{error}\nตำแหน่งแคช: {folder}')
    emit(type='progress', phase='model', detail='ดาวน์โหลด Florence ครบแล้ว')


def verify():
    import torch
    from PIL import Image
    from transformers import AutoProcessor, Florence2ForConditionalGeneration

    device = 'cuda' if torch.cuda.is_available() else 'mps' if torch.backends.mps.is_available() else 'cpu'
    dtype = torch.float16 if device == 'cuda' else torch.float32
    emit(type='progress', phase='verify', detail='กำลังโหลดโมเดลและตรวจการทำงาน', device=device)
    processor = AutoProcessor.from_pretrained(MODEL, local_files_only=True)
    model = Florence2ForConditionalGeneration.from_pretrained(
        MODEL, dtype=dtype, local_files_only=True).to(device).eval()
    inputs = processor(text='<CAPTION>', images=Image.new('RGB', (224, 224), 'white'),
                       return_tensors='pt').to(device, dtype)
    with torch.inference_mode():
        model.generate(**inputs, max_new_tokens=16, num_beams=1, do_sample=False)
    if device == 'cuda':
        torch.cuda.synchronize()
    elif device == 'mps':
        torch.mps.synchronize()
    emit(type='verified', device=device)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=['check', 'download', 'verify'])
    action = parser.parse_args().action
    try:
        with redirect_stdout(sys.stderr):
            {'check': check, 'download': download, 'verify': verify}[action]()
        return 0
    except Exception as error:
        emit(type='error', detail=f'{type(error).__name__}: {error}'[:1500])
        return 1


if __name__ == '__main__':
    raise SystemExit(main())

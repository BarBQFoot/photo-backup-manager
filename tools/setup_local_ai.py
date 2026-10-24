"""Download and smoke-test the app's local Florence model. No Gemini or DB access."""
import os
import platform


def main():
    # Wails may supply loglevel=Info, which Python logging does not accept.
    for key in list(os.environ):
        if key.lower() == 'loglevel':
            os.environ.pop(key)

    import torch
    from huggingface_hub import snapshot_download
    from PIL import Image
    from transformers import AutoProcessor, Florence2ForConditionalGeneration

    model_id = 'florence-community/Florence-2-base-ft'
    print(f'Computer: {platform.system()} {platform.machine()}', flush=True)
    print(f'Downloading {model_id} (existing downloads are reused)...', flush=True)
    snapshot_download(model_id)
    processor = AutoProcessor.from_pretrained(model_id, local_files_only=True)
    model = Florence2ForConditionalGeneration.from_pretrained(
        model_id, dtype=torch.float32, local_files_only=True).eval()
    device = 'mps' if torch.backends.mps.is_available() else 'cpu'
    print(f'Testing Florence on {device}...', flush=True)
    model = model.to(device)
    inputs = processor(text='<CAPTION>', images=Image.new('RGB', (224, 224), 'white'),
                       return_tensors='pt').to(device, torch.float32)
    with torch.inference_mode():
        model.generate(**inputs, max_new_tokens=16, num_beams=1, do_sample=False)
    if device == 'mps':
        torch.mps.synchronize()
    print(f'Florence ready. Device: {device}. No Gemini requests sent.', flush=True)
    if device == 'cpu':
        print('MPS is unavailable; the app will use CPU and may be slower.', flush=True)


if __name__ == '__main__':
    main()

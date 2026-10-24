"""Offline first-launch checks; model downloads and GPU imports are not performed."""
from contextlib import redirect_stdout
import io
from pathlib import Path
import tempfile
import types
import unittest
from unittest.mock import patch

from tools import bootstrap_ai as setup


class BootstrapTests(unittest.TestCase):
    def setUp(self):
        constants = types.SimpleNamespace(HF_HUB_CACHE='mock-hub-cache', HF_HUB_OFFLINE=False)
        patcher = patch.dict('sys.modules', {'huggingface_hub.constants': constants})
        patcher.start()
        self.addCleanup(patcher.stop)

    def inspect(self, versions, folder=None):
        def installed(name):
            if name not in versions:
                raise setup.PackageNotFoundError(name)
            return versions[name]
        with patch.object(setup, 'version', side_effect=installed), \
             patch.object(setup, 'emit') as output, \
             patch.dict('sys.modules', {'huggingface_hub': types.SimpleNamespace(snapshot_download=lambda *a, **k: folder)}):
            setup.check()
        return output.call_args.kwargs

    def test_preserves_existing_torch_when_other_packages_are_missing(self):
        report = self.inspect({'torch': '2.13.0', 'torchvision': '0.28.0'})
        self.assertTrue(report['python_ready'])
        self.assertTrue(report['torch_ready'])
        self.assertFalse(report['dependencies_ready'])

    def test_complete_model_fingerprint_changes_with_weight_file(self):
        versions = {name: '5.20.2' if name == 'transformers' else '1.0'
                    for name in ('torch', 'torchvision', 'transformers', 'Pillow', 'safetensors', 'huggingface_hub')}
        test_root = Path(__file__).resolve().parents[1] / 'results'
        test_root.mkdir(exist_ok=True)
        with tempfile.TemporaryDirectory(prefix='bootstrap-test-', dir=test_root) as temporary:
            folder = Path(temporary)
            self.assertTrue(folder.resolve().is_relative_to(test_root.resolve()))
            for name in ('config.json', 'model.safetensors', 'tokenizer.json', 'preprocessor_config.json'):
                (folder / name).write_text('test', encoding='utf-8')
            first = self.inspect(versions, folder)
            self.assertTrue(first['model_ready'])
            (folder / 'model.safetensors').write_text('changed weights', encoding='utf-8')
            self.assertNotEqual(first['signature'], self.inspect(versions, folder)['signature'])
            (folder / 'tokenizer.json').unlink()
            self.assertFalse(self.inspect(versions, folder)['model_ready'])

    def test_model_download_reports_progress_with_mocked_download(self):
        def fake_download(*args, **kwargs):
            self.assertFalse(kwargs['token'])
            self.assertEqual(kwargs['allow_patterns'], setup.MODEL_PATTERNS)
            with kwargs['tqdm_class'](total=2, desc='Fetching model files', unit='files') as bar:
                bar.update(2)
            return 'mock-model'
        with patch.dict('sys.modules', {'huggingface_hub': types.SimpleNamespace(snapshot_download=fake_download)}), \
             patch.object(setup, 'inspect_model', return_value=(Path('mock-model'), [], '')), \
             patch.object(setup, 'emit') as output, redirect_stdout(io.StringIO()):
            setup.download()
        self.assertTrue(any(call.kwargs.get('total') == 2 and call.kwargs.get('completed') == 2
                            for call in output.call_args_list))

    def test_download_incomplete_files_reports_names_and_actual_path(self):
        with patch.dict('sys.modules', {'huggingface_hub': types.SimpleNamespace(snapshot_download=lambda *a, **k: 'actual-cache')}), \
             patch.object(setup, 'inspect_model', return_value=(Path('actual-cache'), ['tokenizer.json'], 'ไฟล์ที่ขาดหรือว่าง: tokenizer.json')), \
             patch.object(setup, 'emit'):
            with self.assertRaisesRegex(RuntimeError, 'tokenizer.json') as result:
                setup.download()
            self.assertIn('actual-cache', str(result.exception))

    def test_check_preserves_local_cache_lookup_failure(self):
        versions = {name: '5.20.2' if name == 'transformers' else '1.0'
                    for name in ('torch', 'torchvision', 'transformers', 'Pillow', 'safetensors', 'huggingface_hub')}
        with patch.dict('sys.modules', {'huggingface_hub': types.SimpleNamespace(
                snapshot_download=lambda *a, **k: (_ for _ in ()).throw(OSError('cache reference missing')))}), \
             patch.object(setup, 'version', side_effect=lambda name: versions[name]), \
             patch.object(setup, 'emit') as output:
            setup.check()
        report = output.call_args.kwargs
        self.assertFalse(report['model_ready'])
        self.assertIn('cache reference missing', report['model_error'])
        self.assertIn('models--florence-community--Florence-2-base-ft', report['model_path'])

    def test_offline_check_uses_same_filter_as_download_without_docs(self):
        versions = {name: '5.20.2' if name == 'transformers' else '1.0'
                    for name in ('torch', 'torchvision', 'transformers', 'Pillow', 'safetensors', 'huggingface_hub')}
        test_root = Path(__file__).resolve().parents[1] / 'results'
        with tempfile.TemporaryDirectory(prefix='filtered-snapshot-', dir=test_root) as temporary:
            folder = Path(temporary)
            for name in setup.MODEL_FILES:
                (folder / name).write_text('test', encoding='utf-8')
            def snapshot(*args, **kwargs):
                self.assertTrue(kwargs['local_files_only'])
                if kwargs.get('allow_patterns') != setup.MODEL_PATTERNS:
                    raise OSError('IncompleteSnapshotError: README.md and .gitattributes missing')
                return folder
            with patch.dict('sys.modules', {'huggingface_hub': types.SimpleNamespace(snapshot_download=snapshot)}), \
                 patch.object(setup, 'version', side_effect=lambda name: versions[name]), \
                 patch.object(setup, 'emit') as output:
                setup.check()
            self.assertTrue(output.call_args.kwargs['model_ready'])


if __name__ == '__main__':
    unittest.main()

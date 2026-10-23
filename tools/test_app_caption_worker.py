"""Offline desktop protocol/cache checks; no GPU or real API calls."""
from contextlib import redirect_stdout
import io
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

from tools import app_caption_worker as worker
from tools import translate_captions as translator


class FakeCaptioner:
    calls = 0

    def caption(self, path):
        FakeCaptioner.calls += 1
        return 'A test photo ' + path.read_text(encoding='utf-8')


class WorkerTests(unittest.TestCase):
    def setUp(self):
        root = Path(__file__).resolve().parents[1] / 'results'
        root.mkdir(exist_ok=True)
        self.temp = tempfile.TemporaryDirectory(prefix='worker-test-', dir=root)
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name).resolve()
        self.events = []
        self.api_sizes = []
        FakeCaptioner.calls = 0

    def request(self, size):
        items = []
        for n in range(size):
            path = self.root / f'{n}.jpg'
            path.write_text(str(n), encoding='utf-8')
            items.append({'file_id': f'file_{n+1}', 'path': str(path)})
        return {'items': items, 'cache_dir': str(self.root / 'cache'), 'model': 'test-model'}

    def api(self, model, key, body, timeout):
        self.assertEqual(key, 'fake-test-key')
        self.assertNotIn('inline_data', json.dumps(body))
        text = body['contents'][0]['parts'][0]['text']
        payload = json.loads(text[text.index('\n[')+1:])
        self.api_sizes.append(len(payload))
        response = [{'file_id': r['file_id'], 'caption': 'คำบรรยาย ' + r['caption_en']} for r in reversed(payload)]
        return {'candidates': [{'finishReason': 'STOP', 'content': {'parts': [{'text': json.dumps(response)}]}}]}

    def run_worker(self, request, api=None):
        with patch.object(worker, 'emit', side_effect=lambda **e: self.events.append(e)), \
             patch.object(worker, 'load_key', return_value='fake-test-key'), \
             patch.object(translator, 'request_api', side_effect=api or self.api), \
             patch.object(translator.time, 'sleep') as sleep, redirect_stdout(io.StringIO()):
            worker.process(request, make_captioner=FakeCaptioner)
        return sleep

    def test_batch_100_and_cache_reuses_captions_without_api(self):
        request = self.request(101)
        sleep = self.run_worker(request)
        sleep.assert_called_once_with(10)
        self.assertEqual(self.api_sizes, [100, 1])
        self.assertEqual(FakeCaptioner.calls, 101)
        results = [e for e in self.events if e['type'] == 'result']
        self.assertEqual({e['file_id'] for e in results}, {i['file_id'] for i in request['items']})
        self.events.clear()
        # DB identities can change without invalidating the content cache.
        for n, item in enumerate(request['items'], 101):
            item['file_id'] = f'file_{n}'
        self.run_worker(request, api=lambda *a: self.fail('cached results must not send requests'))
        self.assertEqual(FakeCaptioner.calls, 101)
        self.assertEqual(len([e for e in self.events if e['type'] == 'result']), 101)

    def test_timeout_preserves_completed_batch_and_resume_only_remaining(self):
        request = self.request(101)
        def fail_second(*args):
            if self.api_sizes:
                raise translator.APIError('timeout; completion unknown', False)
            return self.api(*args)
        with self.assertRaisesRegex(ValueError, 'timeout'):
            self.run_worker(request, api=fail_second)
        self.assertEqual(len([e for e in self.events if e['type'] == 'result']), 100)
        self.events.clear()
        self.run_worker(request)
        self.assertEqual(self.api_sizes, [100, 1])
        self.assertEqual(FakeCaptioner.calls, 101)
        self.assertEqual(len([e for e in self.events if e['type'] == 'result']), 101)

    def test_old_50_item_run_does_not_override_new_batch_size(self):
        request = self.request(101)
        def unavailable(*args):
            raise translator.APIError('timeout; completion unknown', False)
        with patch.object(worker, 'TRANSLATION_BATCH_SIZE', 50):
            with self.assertRaisesRegex(ValueError, 'timeout'):
                self.run_worker(request, api=unavailable)
        old_batch = next(Path(request['cache_dir']).glob('run_*/translation_batches/batch_001.json'))
        old_contents = old_batch.read_bytes()
        self.run_worker(request)
        self.assertEqual(self.api_sizes, [100, 1])
        self.assertEqual(FakeCaptioner.calls, 101)
        self.assertEqual(old_batch.read_bytes(), old_contents)

    def test_modified_image_invalidates_both_caches(self):
        request = self.request(1)
        self.run_worker(request)
        Path(request['items'][0]['path']).write_text('changed', encoding='utf-8')
        self.run_worker(request)
        self.assertEqual(FakeCaptioner.calls, 2)
        self.assertEqual(self.api_sizes, [1, 1])

    def test_invalid_id_stops_before_caption_or_api(self):
        request = self.request(1)
        request['items'][0]['file_id'] = '../../file_1'
        with self.assertRaises(ValueError):
            self.run_worker(request)
        self.assertEqual(FakeCaptioner.calls, 0)
        self.assertEqual(self.api_sizes, [])

    def test_model_load_failure_preserves_actual_cause_and_stops_once(self):
        request = self.request(2)
        def unavailable():
            raise OSError('test model cache unavailable')
        with patch.object(worker, 'emit'), patch.object(worker, 'load_key', return_value='fake-test-key'):
            with self.assertRaisesRegex(worker.SetupError, 'OSError.*test model cache unavailable'):
                worker.process(request, make_captioner=unavailable)
        diagnostic = json.loads((Path(request['cache_dir']) / 'model_load_error.json').read_text(encoding='utf-8'))
        self.assertEqual(diagnostic['error_type'], 'OSError')
        self.assertEqual(diagnostic['message'], 'test model cache unavailable')


if __name__ == '__main__':
    unittest.main()

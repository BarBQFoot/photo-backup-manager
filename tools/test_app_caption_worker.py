"""Offline desktop protocol/cache checks; no GPU or real API calls."""
from contextlib import redirect_stdout
import io
import json
from pathlib import Path
import tempfile
import threading
import types
import unittest
from unittest.mock import Mock, patch

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

    def run_worker(self, request, api=None, captioner=FakeCaptioner):
        with patch.object(worker, 'emit', side_effect=lambda **e: self.events.append(e)), \
             patch.object(worker, 'load_key', return_value='fake-test-key'), \
             patch.object(translator, 'request_api', side_effect=api or self.api), \
             patch.object(translator.time, 'sleep') as sleep, redirect_stdout(io.StringIO()):
            worker.process(request, make_captioner=captioner)
        return sleep

    def test_batch_100_and_cache_reuses_captions_without_api(self):
        request = self.request(101)
        sleep = self.run_worker(request)
        self.assertEqual(sleep.call_count, 1)
        self.assertGreater(sleep.call_args.args[0], 0)
        self.assertLessEqual(sleep.call_args.args[0], 10)
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

    def test_beam_change_recomputes_without_deleting_old_cache(self):
        request = self.request(1)
        with patch.dict(worker.SETTINGS, {'beams': 3}):
            self.run_worker(request)
        cache = Path(request['cache_dir'])
        old_files = {path: path.read_bytes() for pattern in ('caption_*.json', 'thai_*.json')
                     for path in cache.glob(pattern)}
        self.assertEqual(worker.SETTINGS['beams'], 1)
        self.run_worker(request)
        self.assertEqual(FakeCaptioner.calls, 2)
        # Identical English text can reuse a validated translation run, even after re-captioning.
        self.assertEqual(self.api_sizes, [1])
        for path, content in old_files.items():
            self.assertEqual(path.read_bytes(), content)
        self.run_worker(request, api=lambda *a: self.fail('beam-1 cache must be reused'))
        self.assertEqual(FakeCaptioner.calls, 2)

    def test_translation_overlaps_later_caption_batch_and_keeps_ids(self):
        request = self.request(12)
        api_started = threading.Event()
        later_reading = threading.Event()
        batch_sizes = []
        test = self

        class BatchCaptioner(FakeCaptioner):
            def caption_many(self, paths):
                batch_sizes.append(len(paths))
                if len(batch_sizes) == 2:
                    later_reading.set()
                    test.assertTrue(api_started.wait(5), 'Gemini did not start while later images were being read')
                return [self.caption(path) for path in paths]

        def api(*args):
            api_started.set()
            self.assertTrue(later_reading.wait(5), 'Captioning stopped while Gemini was in flight')
            return self.api(*args)

        with patch.object(worker, 'TRANSLATION_BATCH_SIZE', 4):
            self.run_worker(request, api=api, captioner=BatchCaptioner)
        self.assertEqual(batch_sizes, [4, 4, 4])
        results = {event['file_id']: event['caption'] for event in self.events if event['type'] == 'result'}
        self.assertEqual(results, {f'file_{n+1}': f'คำบรรยาย A test photo {n}' for n in range(12)})
        last = [event for event in self.events if event['type'] == 'progress'][-1]
        self.assertEqual((last['captioned'], last['translated']), (12, 12))

    def test_translation_failure_preserves_all_english_and_stops_future_sends(self):
        request = self.request(12)
        calls = []
        def unavailable(*args):
            calls.append(1)
            raise translator.APIError('timeout; completion unknown', False)
        with patch.object(worker, 'TRANSLATION_BATCH_SIZE', 4):
            with self.assertRaisesRegex(ValueError, 'timeout'):
                self.run_worker(request, api=unavailable)
        self.assertEqual(len(calls), 1)
        self.assertEqual(len(list(Path(request['cache_dir']).glob('caption_*.json'))), 12)
        self.assertEqual(len(list(Path(request['cache_dir']).glob('thai_*.json'))), 0)
        self.run_worker(request)
        self.assertEqual(FakeCaptioner.calls, 12)
        self.assertEqual(len([event for event in self.events if event['type'] == 'result']), 12)

    def test_corrupt_image_in_batch_does_not_lose_other_images(self):
        request = self.request(4)
        class BatchCaptioner(FakeCaptioner):
            def caption_many(self, paths):
                raise ValueError('one corrupt image')
            def caption(self, path):
                if path.stem == '2':
                    raise ValueError('corrupt image')
                return super().caption(path)
        self.run_worker(request, captioner=BatchCaptioner)
        self.assertEqual({e['file_id'] for e in self.events if e['type'] == 'result'}, {'file_1', 'file_2', 'file_4'})
        self.assertEqual([e['file_id'] for e in self.events if e['type'] == 'failure'], ['file_3'])

    def test_oom_reduces_batch_to_one_without_losing_order(self):
        captioner = worker.Captioner.__new__(worker.Captioner)
        captioner.device, captioner.batch_size = 'cuda', 4
        empty_cache = Mock()
        captioner.torch = types.SimpleNamespace(cuda=types.SimpleNamespace(empty_cache=empty_cache))
        def generate(paths):
            if len(paths) > 1:
                raise RuntimeError('CUDA out of memory')
            return [str(paths[0])]
        captioner._caption_many_once = generate
        self.assertEqual(captioner.caption_many(list(range(7))), [str(n) for n in range(7)])
        self.assertEqual(captioner.batch_size, 1)
        self.assertTrue(empty_cache.called)

    def test_batch_calibration_selects_faster_size_and_reuses_profile(self):
        captioner = worker.Captioner.__new__(worker.Captioner)
        captioner.device, captioner.batch_size, captioner.dtype = 'cuda', 1, 'float16'
        captioner.torch = types.SimpleNamespace(__version__='test-runtime', cuda=types.SimpleNamespace(
            get_device_name=lambda: 'test-GPU', synchronize=lambda: None))
        captioner.caption_many = Mock(return_value=['caption'] * 4)
        cache = self.root / 'profiles'
        cache.mkdir()
        # Two measurements for each candidate: size 1=10s, size 2=4s, size 4=6s.
        ticks = [0, 10, 20, 30, 40, 44, 50, 54, 60, 66, 70, 76]
        with patch.dict(worker.os.environ, {'PHOTO_AI_CAPTION_BATCH_SIZE': 'auto'}), \
             patch.object(worker, 'emit'), patch.object(worker.time, 'perf_counter', side_effect=ticks):
            captioner.configure_batch([1, 2, 3, 4], cache, 100)
        self.assertEqual(captioner.batch_size, 2)
        captioner.batch_size = 1
        captioner.caption_many = Mock(side_effect=AssertionError('saved profile must skip calibration'))
        with patch.dict(worker.os.environ, {'PHOTO_AI_CAPTION_BATCH_SIZE': 'auto'}):
            captioner.configure_batch([1, 2, 3, 4], cache, 100)
        self.assertEqual(captioner.batch_size, 2)

    def test_explicit_batch_override_skips_calibration(self):
        captioner = worker.Captioner.__new__(worker.Captioner)
        captioner.device, captioner.batch_size = 'cuda', 1
        with patch.dict(worker.os.environ, {'PHOTO_AI_CAPTION_BATCH_SIZE': '4'}):
            captioner.configure_batch([], self.root, 10)
        self.assertEqual(captioner.batch_size, 4)

    def test_batched_generation_removes_padding_after_eos_but_keeps_start_token(self):
        rows = [[2, 0, 91, 2, 1, 1], [2, 0, 92, 93, 94, 2]]
        self.assertEqual(worker.trim_generated_tokens(rows, {2}, 64),
                         [[2, 0, 91, 2], [2, 0, 92, 93, 94, 2]])
        with self.assertRaisesRegex(ValueError, 'truncated'):
            worker.trim_generated_tokens([[2, 0, 91, 92]], {2}, 3)

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

    def test_resident_factory_loads_once_for_two_uncached_jobs(self):
        request = self.request(1)
        make = Mock(side_effect=FakeCaptioner)
        factory = worker.ResidentCaptioner(make)
        with patch.object(worker, 'emit', side_effect=lambda **e: self.events.append(e)), \
             patch.object(worker, 'load_key', return_value='fake-test-key'), \
             patch.object(translator, 'request_api', side_effect=self.api), \
             patch.object(translator.time, 'sleep'), redirect_stdout(io.StringIO()):
            self.assertTrue(worker.handle_request(request, factory))
            Path(request['items'][0]['path']).write_text('second job', encoding='utf-8')
            self.assertTrue(worker.handle_request(request, factory))
        self.assertEqual(make.call_count, 1)
        self.assertEqual(FakeCaptioner.calls, 2)
        self.assertEqual(len([e for e in self.events if e['type'] == 'done']), 2)
        summary = json.loads((Path(request['cache_dir']) / 'pipeline_summary.json').read_text(encoding='utf-8'))
        self.assertTrue(summary['model_reused'])

    def test_resident_request_error_does_not_poison_next_job(self):
        request = self.request(1)
        factory = worker.ResidentCaptioner(FakeCaptioner)
        with patch.object(worker, 'emit', side_effect=lambda **e: self.events.append(e)), \
             patch.object(worker, 'load_key', return_value=''), redirect_stdout(io.StringIO()):
            self.assertFalse(worker.handle_request(request, factory))
        self.assertEqual(self.events[-1]['type'], 'done')
        with patch.object(worker, 'emit', side_effect=lambda **e: self.events.append(e)), \
             patch.object(worker, 'load_key', return_value='fake-test-key'), \
             patch.object(translator, 'request_api', side_effect=self.api), redirect_stdout(io.StringIO()):
            self.assertTrue(worker.handle_request(request, factory))
        self.assertEqual(FakeCaptioner.calls, 1)
        self.assertEqual(len([e for e in self.events if e['type'] == 'result']), 1)


if __name__ == '__main__':
    unittest.main()

"""Offline tests: provider calls are mocked, never sent to Gemini."""

from contextlib import redirect_stdout
import io
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import urllib.error

from tools import translate_captions as translator


KEY = "fake-secret-not-a-real-api-key"


def envelope(file_id, finish="STOP"):
    return {"candidates": [{"finishReason": finish, "content": {"parts": [
        {"text": json.dumps([{"file_id": file_id, "caption": "translated text"}])}
    ]}}], "usageMetadata": {"totalTokenCount": 10}}


class TranslationTests(unittest.TestCase):
    def setUp(self):
        self.base = (translator.ROOT / "results").resolve()
        self.base.mkdir(exist_ok=True)
        self.temporary = tempfile.TemporaryDirectory(prefix="translation-test-", dir=self.base)
        self.run = Path(self.temporary.name).resolve()
        self.assertTrue(self.run.is_relative_to(self.base))
        (self.run / "translation_batches").mkdir()
        (self.run / "translation_responses").mkdir()
        self.payloads = [{"file_id": value, "caption_en": "A test image."} for value in ("a", "b", "c")]
        translator.write_json(self.run / "translation_payloads.json", self.payloads)
        for index, item in enumerate(self.payloads, 1):
            translator.write_json(self.run / "translation_batches" / f"batch_{index:03d}.json", [item])
        self.manual = self.run / "translation_responses/batch_001_response.json"
        translator.write_json(self.manual, [{"file_id": "a", "caption": "manual translation"}])
        self.payloads, self.batches = translator.read_catalog(self.run)

    def tearDown(self):
        # Explicitly verify the recursive cleanup stays in this task's results directory.
        self.assertTrue(self.run.is_relative_to(self.base))
        self.assertTrue(self.run.name.startswith("translation-test-"))
        self.temporary.cleanup()

    def execute(self, limit=None, retries=0):
        with redirect_stdout(io.StringIO()):
            return translator.execute(self.run, self.payloads, self.batches, KEY,
                                      "gemini-2.5-flash", "translate", limit,
                                      interval=0, retries=retries, max_tokens=8192, timeout=120)

    def test_resume_skips_manual_and_api_results_and_merges_by_id(self):
        original = self.manual.read_bytes()
        with patch.object(translator, "request_api", return_value=envelope("b")) as api:
            self.assertEqual(self.execute(limit=1), 0)
            self.assertEqual(api.call_count, 1)
            self.assertIn('"file_id": "b"', api.call_args.args[2]["contents"][0]["parts"][0]["text"])
        summary = json.loads((self.run / "translation_summary.json").read_text())
        self.assertEqual((summary["translated"], summary["pending"]), (2, 1))
        with patch.object(translator, "request_api", return_value=envelope("c")) as api:
            self.assertEqual(self.execute(), 0)
            self.assertEqual(api.call_count, 1)
        merged = json.loads((self.run / "captions_translated.json").read_text())
        self.assertEqual([item["file_id"] for item in merged], ["a", "b", "c"])
        self.assertTrue(all(item["translation_status"] == "translated" for item in merged))
        self.assertEqual(self.manual.read_bytes(), original)

    def test_wrong_id_preserves_attempt_and_does_not_accept_batch(self):
        with patch.object(translator, "request_api", return_value=envelope("foreign")) as api:
            self.assertEqual(self.execute(), 1)
            self.assertEqual(api.call_count, 1)
        folder = self.run / "translation_responses"
        self.assertFalse((folder / "batch_002_api_response.json").exists())
        self.assertEqual(len(list(folder.glob("batch_002_attempt_*.json"))), 1)
        summary = json.loads((self.run / "translation_summary.json").read_text())
        self.assertEqual((summary["translated"], summary["pending"]), (1, 2))

    def test_extra_null_id_retries_same_batch_and_accepts_only_valid_response(self):
        bad = envelope('b')
        bad['candidates'][0]['content']['parts'][0]['text'] = json.dumps([
            {'file_id': 'b', 'caption': 'translated text'},
            {'file_id': 'file_7800', 'caption': None},
        ])
        with patch.object(translator, 'request_api', side_effect=[bad, envelope('b')]) as api, \
             patch.object(translator.time, 'sleep') as sleep:
            self.assertEqual(self.execute(limit=1, retries=1), 0)
            self.assertEqual(api.call_count, 2)
            self.assertEqual(api.call_args_list[0].args[2], api.call_args_list[1].args[2])
            sleep.assert_called_once_with(0)
        saved = json.loads((self.run / 'translation_responses/batch_002_api_response.json').read_text())
        self.assertEqual([item['file_id'] for item in saved], ['b'])
        self.assertEqual(len(list((self.run / 'translation_responses').glob('batch_002_attempt_*.json'))), 2)

    def test_malformed_caption_json_is_retryable_validation_error(self):
        for text in ('{', '{}'):
            bad = envelope('b')
            bad['candidates'][0]['content']['parts'][0]['text'] = text
            with self.subTest(text=text), self.assertRaises(translator.ResponseValidationError):
                translator.parse_response(bad, [self.payloads[1]])

    def test_invalid_ids_exhaust_retries_without_accepting_or_sending_next_batch(self):
        with patch.object(translator, 'request_api', return_value=envelope('foreign')) as api, \
             patch.object(translator.time, 'sleep'):
            self.assertEqual(self.execute(retries=1), 1)
            self.assertEqual(api.call_count, 2)
        summary = json.loads((self.run / 'translation_summary.json').read_text())
        self.assertEqual((summary['translated'], summary['pending']), (1, 2))
        self.assertFalse((self.run / 'translation_responses/batch_002_api_response.json').exists())

    def test_truncated_response_rejected_even_if_json_is_valid(self):
        with self.assertRaisesRegex(ValueError, "MAX_TOKENS"):
            translator.parse_response(envelope("b", "MAX_TOKENS"), [self.payloads[1]])

    def test_rate_limit_retries_bounded_then_succeeds(self):
        error = translator.APIError("Gemini HTTP 429", retryable=True)
        with patch.object(translator, "request_api", side_effect=[error, envelope("b")]) as api, patch.object(translator.time, "sleep"):
            self.assertEqual(self.execute(limit=1, retries=1), 0)
            self.assertEqual(api.call_count, 2)

    def test_overloaded_model_waits_one_minute_without_extra_interval_sleep(self):
        error = translator.APIError("Gemini HTTP 503: high demand", retryable=True, http_status=503)
        with patch.object(translator, "request_api", side_effect=[error, envelope("b")]) as api, patch.object(translator.time, "sleep") as sleep:
            self.assertEqual(self.execute(limit=1, retries=1), 0)
            self.assertEqual(api.call_count, 2)
            sleep.assert_called_once_with(60)

    def test_rate_limit_exhaustion_stops_without_skipping_pending_batch(self):
        error = translator.APIError("Gemini HTTP 429", retryable=True)
        with patch.object(translator, "request_api", side_effect=error) as api, patch.object(translator.time, "sleep"):
            self.assertEqual(self.execute(retries=1), 1)
            self.assertEqual(api.call_count, 2)
        summary = json.loads((self.run / "translation_summary.json").read_text())
        self.assertEqual(summary["pending"], 2)

    def test_daily_quota_is_not_retried_and_key_not_logged(self):
        error = urllib.error.HTTPError("https://example.invalid", 429, "quota", {},
                                      io.BytesIO(("quota per_day " + KEY).encode()))
        with patch.object(translator.urllib.request, "urlopen", side_effect=error):
            with self.assertRaises(translator.APIError) as caught:
                translator.request_api("gemini-2.5-flash", KEY, {}, 120)
        self.assertFalse(caught.exception.retryable)
        self.assertNotIn(KEY, str(caught.exception))

    def test_timeout_is_not_automatically_retried(self):
        with patch.object(translator.urllib.request, "urlopen", side_effect=TimeoutError):
            with self.assertRaises(translator.APIError) as caught:
                translator.request_api("gemini-2.5-flash", KEY, {}, 120)
        self.assertFalse(caught.exception.retryable)

    def test_provider_retry_info_is_honored_without_retrying_too_early(self):
        body = json.dumps({"error": {"details": [{"@type": "type.googleapis.com/google.rpc.RetryInfo", "retryDelay": "120s"}]}}).encode()
        error = urllib.error.HTTPError("https://example.invalid", 429, "quota", {}, io.BytesIO(body))
        with patch.object(translator.urllib.request, "urlopen", side_effect=error):
            with self.assertRaises(translator.APIError) as caught:
                translator.request_api("gemini-2.5-flash", KEY, {}, 120)
        self.assertEqual(caught.exception.retry_after, 120)
        self.assertFalse(caught.exception.retryable)

    def test_keyword_payloads_cannot_be_sent_to_translation_endpoint(self):
        item = {"file_id": "a", "accepted": {"objects": [], "keywords": []}}
        translator.write_json(self.run / "translation_payloads.json", [item])
        with self.assertRaisesRegex(ValueError, "caption_en"):
            translator.read_catalog(self.run)

    def test_source_change_cannot_reuse_response(self):
        translator.write_json(self.run / "translation_responses/batch_001_meta.json",
                              {"request_sha256": translator.fingerprint([{"file_id": "a", "caption_en": "Old caption"}])})
        with self.assertRaisesRegex(ValueError, "source text changed"):
            translator.existing_response(self.run, "batch_001", self.batches[0][1])

    def test_dry_run_does_not_send_or_write_and_does_not_print_key(self):
        before = {str(path.relative_to(self.run)): path.read_bytes() for path in self.run.rglob("*") if path.is_file()}
        output = io.StringIO()
        with patch.object(translator.sys, "argv", ["translate", "--run", str(self.run)]), patch.object(translator, "load_key", return_value=KEY), patch.object(translator, "request_api") as api, redirect_stdout(output):
            self.assertEqual(translator.main(), 0)
        api.assert_not_called()
        self.assertNotIn(KEY, output.getvalue())
        self.assertEqual(json.loads(output.getvalue())["pending_batches"], 2)
        after = {str(path.relative_to(self.run)): path.read_bytes() for path in self.run.rglob("*") if path.is_file()}
        self.assertEqual(before, after)

    def test_key_is_header_only_and_request_has_no_images(self):
        response = io.BytesIO(json.dumps(envelope("b")).encode())
        body = translator.build_request([self.payloads[1]], "translate", 8192)
        with patch.object(translator.urllib.request, "urlopen", return_value=response) as urlopen:
            translator.request_api("gemini-2.5-flash", KEY, body, 120)
        request = urlopen.call_args.args[0]
        self.assertNotIn(KEY, request.full_url)
        self.assertNotIn(KEY, request.data.decode())
        headers = {key.lower(): value for key, value in request.header_items()}
        self.assertEqual(headers["x-goog-api-key"], KEY)
        self.assertEqual(set(body["contents"][0]["parts"][0]), {"text"})

    def test_bad_request_reports_provider_reason_but_redacts_credentials(self):
        body = json.dumps({"error": {"status": "INVALID_ARGUMENT", "message": "Bad responseSchema " + KEY,
                                    "details": [{"reason": "SCHEMA_INVALID"}]}}).encode()
        error = urllib.error.HTTPError("https://example.invalid", 400, "bad request", {}, io.BytesIO(body))
        with patch.object(translator.urllib.request, "urlopen", side_effect=error):
            with self.assertRaises(translator.APIError) as caught:
                translator.request_api("gemini-2.5-flash", KEY, {}, 120)
        message = str(caught.exception)
        self.assertIn("SCHEMA_INVALID", message)
        self.assertIn("Bad responseSchema", message)
        self.assertNotIn(KEY, message)
        self.assertFalse(caught.exception.retryable)

    def test_metadata_check_is_get_only_and_never_sends_caption_data(self):
        response = io.BytesIO(json.dumps({"name": "models/test", "supportedGenerationMethods": ["generateContent"]}).encode())
        with patch.object(translator.urllib.request, "urlopen", return_value=response) as urlopen:
            result = translator.check_access("gemini-2.5-flash", KEY, 30)
        request = urlopen.call_args.args[0]
        self.assertEqual(request.get_method(), "GET")
        self.assertIsNone(request.data)
        self.assertNotIn("generateContent", request.full_url)
        self.assertNotIn(KEY, request.full_url)
        self.assertEqual(result["generation_requests_sent"], 0)

    def test_large_batch_keeps_server_schema_small_and_preserves_all_input_ids(self):
        batch = [{"file_id": f"photo_{index:03d}", "caption_en": "A photo."} for index in range(50)]
        body = translator.build_request(batch, "translate", 8192)
        schema = body["generationConfig"]["responseSchema"]
        self.assertLess(len(json.dumps(schema)), 400)
        self.assertNotIn("enum", json.dumps(schema))
        self.assertNotIn("minItems", schema)
        self.assertNotIn("maxItems", schema)
        text = body["contents"][0]["parts"][0]["text"]
        self.assertEqual(json.loads(text.split('\n', 1)[1]), batch)

    def test_run_lock_releases_after_failure(self):
        with translator.run_lock(self.run):
            with self.assertRaisesRegex(ValueError, "already running"):
                with translator.run_lock(self.run):
                    self.fail("Second lock must not succeed")
        with translator.run_lock(self.run):
            pass


if __name__ == "__main__":
    unittest.main()

"""Unit fault injection for bulk HTTP-200 responses, not real OpenSearch E2E."""
import io
import json
import pathlib
import runpy
import unittest
from unittest.mock import patch


class SeedTests(unittest.TestCase):
    def run_seed(self, bulk):
        def response(request, timeout):
            payload = bulk if '/_bulk' in request.full_url else {'acknowledged': True}
            return io.BytesIO(json.dumps(payload).encode())
        with patch('urllib.request.urlopen', side_effect=response):
            runpy.run_path(str(pathlib.Path(__file__).with_name('seed.py')), run_name='__main__')

    def test_bulk_success(self):
        self.run_seed({'errors': False, 'items': [{}, {}, {}]})

    def test_item_error_rejected(self):
        with self.assertRaisesRegex(RuntimeError, 'Bulk fixture indexing failed'):
            self.run_seed({'errors': True, 'items': [{}, {}, {}]})

    def test_missing_items_rejected(self):
        with self.assertRaisesRegex(RuntimeError, 'Bulk fixture indexing failed'):
            self.run_seed({'errors': False, 'items': [{}]})


if __name__ == '__main__':
    unittest.main()

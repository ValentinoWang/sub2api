from pathlib import Path
import tempfile
import unittest
from apply_native_wiring import blob_sha, transform, run


class NativeWiringTests(unittest.TestCase):
    def test_applies_and_is_idempotent(self):
        raw=b'prefix\nanchor\nsuffix\n';sha=blob_sha(raw)
        status,new=transform(raw,sha,'anchor\n','anchor\naddition\n')
        self.assertEqual(status,'READY')
        self.assertEqual(transform(new,sha,'anchor\n','anchor\naddition\n'),('ALREADY_APPLIED',new))
    def test_drift_refused(self):
        with self.assertRaises(ValueError):
            transform(b'changed\nanchor\n',blob_sha(b'anchor\n'),'anchor\n','anchor\naddition\n')
    def test_preflight_does_not_partially_write(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp);(root/'a').write_bytes(b'one');(root/'b').write_bytes(b'drift')
            rules=(('a',blob_sha(b'one'),'one','one+'),('b',blob_sha(b'two'),'two','two+'))
            with self.assertRaises(ValueError): run(root,True,rules)
            self.assertEqual((root/'a').read_bytes(),b'one')
    def test_check_is_read_only(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp);(root/'a').write_bytes(b'one')
            result=run(root,False,(('a',blob_sha(b'one'),'one','one+'),))
            self.assertEqual(result[0]['status'],'READY')
            self.assertEqual((root/'a').read_bytes(),b'one')

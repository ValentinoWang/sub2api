import unittest

from xui_sales.xianyu import XianyuCallbackError, calculate_signature, verify_signature


class XianyuSignatureTests(unittest.TestCase):
    def test_valid_signature(self):
        body = b'{"order_no":"12345678"}'
        signature = calculate_signature(body, "app", 1000, "secret", "merchant")
        verify_signature(body, "app", "1000", signature, "app", "secret", 30, "merchant", now=1000)

    def test_stale_signature_is_rejected(self):
        body = b"{}"
        signature = calculate_signature(body, "app", 1000, "secret")
        with self.assertRaises(XianyuCallbackError):
            verify_signature(body, "app", "1000", signature, "app", "secret", 30, now=1031)


if __name__ == "__main__":
    unittest.main()

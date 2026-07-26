from __future__ import annotations

import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
DEPLOY = ROOT / "deploy"


class DeployContractTests(unittest.TestCase):
    def test_service_credentials_are_file_backed(self) -> None:
        web = (DEPLOY / "xui-sales.service").read_text(encoding="utf-8")
        recover = (DEPLOY / "xui-sales-recover.service").read_text(encoding="utf-8")

        for unit in (web, recover):
            self.assertIn("LoadCredential=xui_api_token:", unit)
            self.assertIn("LoadCredential=redemption_pepper:", unit)
            self.assertIn("LoadCredential=sub2api_admin_api_key:", unit)
            self.assertNotIn("Environment=SUB2API_ADMIN_API_KEY=", unit)
            self.assertNotIn("Environment=XUI_API_TOKEN=", unit)
            self.assertIn("NoNewPrivileges=true", unit)
            self.assertIn("ProtectSystem=strict", unit)

    def test_sub2api_plain_http_is_confined_to_loopback_tunnel(self) -> None:
        tunnel = (DEPLOY / "xui-sales-sub2api-tunnel.service").read_text(encoding="utf-8")
        environment = (DEPLOY / "xui-sales.env.example").read_text(encoding="utf-8")

        self.assertIn("StrictHostKeyChecking=yes", tunnel)
        self.assertIn("ExitOnForwardFailure=yes", tunnel)
        self.assertIn("-L 127.0.0.1:19080:127.0.0.1:8080", tunnel)
        self.assertIn("SUB2API_BASE_URL=http://127.0.0.1:19080", environment)
        self.assertNotIn("SUB2API_ADMIN_API_KEY=", environment)

    def test_public_entrypoint_terminates_tls_and_proxies_to_loopback(self) -> None:
        nginx = (DEPLOY / "nginx-xui-sales.conf").read_text(encoding="utf-8")

        self.assertIn("listen 8443 ssl;", nginx)
        self.assertIn("ssl_protocols TLSv1.2 TLSv1.3;", nginx)
        self.assertIn("proxy_pass http://127.0.0.1:18080;", nginx)
        self.assertIn('Strict-Transport-Security "max-age=31536000" always', nginx)


if __name__ == "__main__":
    unittest.main()

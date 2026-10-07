"""Credential initialization must preserve installations and fail without leaking values."""

from contextlib import redirect_stdout
import io
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import bcrypt
import runtime


class InitializationTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        root = Path(self.temp.name)
        self.state = root / "state"
        self.template = root / "template.yaml"
        self.template.write_text('key: "__API_KEY__"\nmanagement: "__MANAGEMENT_PASSWORD_BCRYPT__"\n')
        self.patch = patch.multiple(runtime, STATE=self.state, TEMPLATE=self.template)
        self.patch.start()
        self.addCleanup(self.patch.stop)

    def initialize(self, mode="init", stdin=""):
        out = io.StringIO()
        with redirect_stdout(out), patch("sys.stdin", io.StringIO(stdin)):
            runtime.initialize(mode)
        return out.getvalue()

    def test_fresh_install_is_private_idempotent_and_hashed(self):
        output = self.initialize()
        values = [runtime.read_key(field) for field in runtime.FIELDS]
        self.assertNotEqual(*values)
        for value in values:
            self.assertNotIn(value, output)
        config = (self.state / "proxy/config.yaml").read_text()
        self.assertIn(values[0], config)
        self.assertNotIn(values[1], config)
        hashed = config.split('management: "')[1].split('"')[0]
        self.assertTrue(bcrypt.checkpw(values[1].encode(), hashed.encode()))
        auth = self.state / "proxy/auths"
        auth.mkdir()
        (auth / "account.json").write_text("private provider state")
        self.initialize()
        self.assertEqual(values, [runtime.read_key(field) for field in runtime.FIELDS])
        self.assertEqual((auth / "account.json").read_text(), "private provider state")
        for path in [self.state, self.state / "secrets", self.state / "proxy"]:
            self.assertEqual(path.stat().st_mode & 0o777, 0o700)
        for path in [self.state / "proxy/config.yaml", *list((self.state / "secrets").iterdir())]:
            self.assertEqual(path.stat().st_mode & 0o777, 0o600)

    def test_existing_legacy_install_requires_explicit_import(self):
        (self.state / "proxy").mkdir(parents=True)
        config = self.state / "proxy/config.yaml"
        config.write_text("old configuration")
        with self.assertRaisesRegex(ValueError, "existing installation"):
            self.initialize()
        self.assertEqual(config.read_text(), "old configuration")
        self.assertFalse((self.state / "secrets").exists())

    def test_partial_credentials_never_regenerate(self):
        self.initialize()
        (self.state / "secrets/management-password").unlink()
        before = runtime.read_key("claude-api-key")
        with self.assertRaisesRegex(ValueError, "incomplete"):
            self.initialize()
        self.assertEqual(runtime.read_key("claude-api-key"), before)

    def test_import_quotes_values_and_rotation_is_explicit(self):
        api = ' quoted"key\\with-special-characters '
        mgmt = "separate-long-management-password"
        output = self.initialize("import", api + "\n" + mgmt + "\n")
        self.assertEqual(runtime.read_key("claude-api-key"), api)
        self.assertNotIn(api, output)
        self.assertNotIn(mgmt, output)
        self.assertIn('quoted\\"key\\\\with', (self.state / "proxy/config.yaml").read_text())
        self.initialize("rotate")
        self.assertNotEqual(runtime.read_key("claude-api-key"), api)
        self.assertNotEqual(runtime.read_key("management-password"), mgmt)

    def test_bad_import_or_template_leaves_existing_secrets_untouched(self):
        self.initialize()
        before = runtime.read_key("claude-api-key")
        for stdin in ["short\nshort\n", "a" * 73 + "\n" + "b" * 32 + "\n", "a" * 32 + "\n" + "b" * 32 + "\nextra"]:
            with self.assertRaises(ValueError):
                self.initialize("import", stdin)
            self.assertEqual(runtime.read_key("claude-api-key"), before)
        self.template.write_text("missing placeholders")
        with self.assertRaises(ValueError):
            self.initialize("rotate")
        self.assertEqual(runtime.read_key("claude-api-key"), before)

    def test_tool_credentials_are_loaded_only_for_the_required_tool(self):
        self.initialize()
        with patch.dict(os.environ, {}, clear=True), patch("os.execv") as execute:
            runtime.main(["quota"])
            self.assertIn("MGMT_KEY", os.environ)
            self.assertNotIn("API_KEY", os.environ)
            execute.assert_called_once_with("/usr/local/bin/quota", ["quota"])
        with patch.dict(os.environ, {"URL": "http://proxy:8317"}, clear=True), patch("os.execv") as execute:
            runtime.main(["ops", "models"])
            execute.assert_called_once_with("/usr/local/bin/ops", ["ops", "models", "-url", "http://proxy:8317"])

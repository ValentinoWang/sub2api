import hashlib
import json
import shutil
import subprocess
import sys
import tempfile
import unittest
import zipfile
from pathlib import Path


ROOT = Path(__file__).resolve().parent
BUILD = ROOT / "build_release.py"


def sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


class ReleaseArtifactTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.root = Path(self.temporary.name)

    def tearDown(self):
        self.temporary.cleanup()

    def build(self, version: str, output: Path, check: bool = True, repository: str = "example/sub2api"):
        return subprocess.run(
            [
                sys.executable, str(BUILD), "--version", version,
                "--repository", repository, "--output", str(output),
            ],
            check=check,
            capture_output=True,
            text=True,
        )

    def test_release_is_reproducible_and_manifest_matches_archives(self):
        first = self.root / "first"
        second = self.root / "second"
        self.build("1.2.3", first)
        self.build("1.2.3", second)

        first_files = {path.name: sha256(path) for path in first.iterdir()}
        second_files = {path.name: sha256(path) for path in second.iterdir()}
        self.assertEqual(first_files, second_files)
        manifest = json.loads((first / "codex-memory-release-manifest.json").read_text())
        self.assertEqual(manifest["tag"], "codex-memory-v1.2.3")
        self.assertEqual({item["platform"] for item in manifest["assets"]}, {"macos", "windows", "linux"})
        for asset in manifest["assets"]:
            archive = first / asset["filename"]
            self.assertEqual(asset["sha256"], sha256(archive))
            self.assertEqual(asset["size"], archive.stat().st_size)
            with zipfile.ZipFile(archive) as bundle:
                names = set(bundle.namelist())
                self.assertIn("codex_memory_unifier.py", names)
                self.assertIn("README.md", names)
                self.assertFalse(any("auth" in name.lower() or "token" in name.lower() for name in names))
                launcher = "codex-memory.ps1" if asset["platform"] == "windows" else "codex-memory"
                self.assertIn(launcher, names)
                if asset["platform"] != "windows":
                    self.assertEqual((bundle.getinfo(launcher).external_attr >> 16) & 0o777, 0o755)

    def test_committed_website_manifest_is_generated_release_authority(self):
        output = self.root / "website"
        self.build("0.1.0", output, repository="ValentinoWang/sub2api")
        committed = ROOT.parent.parent / "frontend" / "public" / "codex-memory-release-manifest.json"
        self.assertEqual(
            (output / "codex-memory-release-manifest.json").read_bytes(),
            committed.read_bytes(),
        )

    def test_clean_install_upgrade_and_uninstall(self):
        first = self.root / "v1"
        second = self.root / "v2"
        install = self.root / "install"
        self.build("1.0.0", first)
        self.build("1.0.1", second)
        with zipfile.ZipFile(first / "codex-memory_1.0.0_linux.zip") as bundle:
            bundle.extractall(install)
        with zipfile.ZipFile(second / "codex-memory_1.0.1_linux.zip") as bundle:
            bundle.extractall(install)
        completed = subprocess.run(
            [sys.executable, str(install / "codex_memory_unifier.py"), "--help"],
            check=True,
            capture_output=True,
            text=True,
        )
        self.assertIn("Safely merge provider-independent Codex state", completed.stdout)
        shutil.rmtree(install)
        self.assertFalse(install.exists())

    def test_unsafe_version_and_nonempty_output_are_rejected(self):
        unsafe_output = self.root / "unsafe"
        result = self.build("../../escape", unsafe_output, check=False)
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(unsafe_output.exists())

        occupied = self.root / "occupied"
        occupied.mkdir()
        sentinel = occupied / "keep.txt"
        sentinel.write_text("keep", encoding="utf-8")
        result = self.build("1.2.3", occupied, check=False)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(sentinel.read_text(encoding="utf-8"), "keep")


if __name__ == "__main__":
    unittest.main()

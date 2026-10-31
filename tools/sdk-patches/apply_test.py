import importlib.util
import json
from pathlib import Path
import tempfile
import unittest


ROOT = Path(__file__).resolve().parent
SPEC = importlib.util.spec_from_file_location("sdk_patches", ROOT / "apply.py")
patches = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(patches)


class PatchTests(unittest.TestCase):
    def setUp(self):
        self.manifest = json.loads((ROOT / "manifest.json").read_text())

    def test_preserves_upstream_overlays_and_is_idempotent(self):
        workflow = (
            "speakeasyVersion: latest\nsources:\n    konnect:\n"
            "        overlays:\n            - location: ./upstream.yaml\n"
            "            - location: ./.speakeasy/overlays/update-portal-patch-defaults.yaml\n"
            "        registry:\n            location: example\n"
        )
        result = patches.configure_workflow(workflow, self.manifest)
        self.assertIn("location: ./upstream.yaml", result)
        self.assertNotIn("./.speakeasy/overlays/", result)
        for name in self.manifest["overlays"]:
            self.assertEqual(result.count("./kongctl-patches/overlays/" + name), 1)
        self.assertEqual(patches.configure_workflow(result, self.manifest), result)

    def test_preserves_multiline_speakeasy_overlay_entries(self):
        workflow = (
            "speakeasyVersion: latest\nsources:\n    konnect:\n"
            "        overlays:\n            -\n                location: ./upstream.yaml\n"
            "            -\n                location: ./kongctl-patches/overlays/update-portal-patch-defaults.yaml\n"
            "        registry:\n                location: registry\n"
        )
        result = patches.configure_workflow(workflow, self.manifest)
        self.assertIn("            -\n                location: ./upstream.yaml\n", result)
        for name in self.manifest["overlays"]:
            self.assertEqual(result.count("./kongctl-patches/overlays/" + name), 1)
        self.assertIn("        registry:\n                location: registry\n", result)
        self.assertEqual(patches.configure_workflow(result, self.manifest), result)

    def test_installs_overlays_in_upstream_workflow(self):
        result = patches.configure_workflow(
            "speakeasyVersion: latest\nsources:\n    konnect:\n        registry:\n"
            "targets:\n    go:\n        codeSamples:\n            registry:\n                location: samples\n",
            self.manifest,
        )
        self.assertIn("        overlays:\n", result)
        self.assertIn("            registry:\n                location: samples\n", result)

    def test_rejects_tag_drift_and_duplicate_fields(self):
        patch = self.manifest["post_generation"][0]
        with self.assertRaises(ValueError):
            patches.replace_tag("IdentityRealms has changed", patch)
        with self.assertRaises(ValueError):
            patches.replace_tag(patch["before"] * 2, patch)
        self.assertEqual(patches.replace_tag(patch["before"], patch), patch["after"])
        self.assertEqual(patches.replace_tag(patch["after"], patch), patch["after"])

    def test_optional_internal_field_preserves_absence_but_rejects_drift(self):
        patch = self.manifest["post_generation"][1]
        public = 'type CreatePortal struct {\nName string `json:"name"`\n}\n'
        self.assertEqual(patches.replace_tag(public, patch), public)
        self.assertEqual(patches.replace_tag(patch["before"], patch), patch["after"])
        self.assertEqual(patches.replace_tag(patch["after"], patch), patch["after"])
        with self.assertRaises(ValueError):
            patches.replace_tag('CreateDefaultContent bool `json:"create_default_content"`', patch)
        with self.assertRaises(ValueError):
            patches.replace_tag(patch["before"] * 2, patch)

    def test_legacy_config_file_is_optional_but_present_files_are_checked(self):
        patch = self.manifest["post_generation"][0]
        with tempfile.TemporaryDirectory() as directory:
            sdk = Path(directory)
            patches.post_generate(sdk, {"post_generation": [patch]})
            required = dict(patch, optional_file=False)
            with self.assertRaises(FileNotFoundError):
                patches.post_generate(sdk, {"post_generation": [required]})
            target = sdk / patch["file"]
            target.parent.mkdir(parents=True)
            target.write_text("changed schema")
            with self.assertRaises(ValueError):
                patches.post_generate(sdk, {"post_generation": [patch]})
            target.write_text(patch["before"])
            patches.post_generate(sdk, {"post_generation": [patch]})
            self.assertEqual(target.read_text(), patch["after"])

    def test_multiple_replacements_in_one_file_accumulate_and_fail_atomically(self):
        with tempfile.TemporaryDirectory() as directory:
            sdk = Path(directory)
            target = sdk / "request.go"
            target.write_text("first second")
            manifest = {"post_generation": [
                {"file": "request.go", "before": "first", "after": "FIRST"},
                {"file": "request.go", "before": "second", "after": "SECOND"},
            ]}
            patches.post_generate(sdk, manifest)
            self.assertEqual(target.read_text(), "FIRST SECOND")
            patches.post_generate(sdk, manifest)
            self.assertEqual(target.read_text(), "FIRST SECOND")
            target.write_text("first drift")
            with self.assertRaises(ValueError):
                patches.post_generate(sdk, manifest)
            self.assertEqual(target.read_text(), "first drift")


if __name__ == "__main__":
    unittest.main()

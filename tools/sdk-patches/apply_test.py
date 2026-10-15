import importlib.util
import json
from pathlib import Path
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

    def test_installs_overlays_in_upstream_workflow(self):
        result = patches.configure_workflow(
            "speakeasyVersion: latest\nsources:\n    konnect:\n        registry:\n",
            self.manifest,
        )
        self.assertIn("        overlays:\n", result)

    def test_rejects_tag_drift_and_duplicate_fields(self):
        patch = self.manifest["post_generation"][0]
        with self.assertRaises(ValueError):
            patches.replace_tag("IdentityRealms has changed", patch)
        with self.assertRaises(ValueError):
            patches.replace_tag(patch["before"] * 2, patch)
        self.assertEqual(patches.replace_tag(patch["before"], patch), patch["after"])
        self.assertEqual(patches.replace_tag(patch["after"], patch), patch["after"])


if __name__ == "__main__":
    unittest.main()

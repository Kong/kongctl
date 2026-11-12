import importlib.util
import json
import hashlib
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

    def test_additive_patch_is_repeatable_but_rejects_duplicates(self):
        patch = {"file": "model.go", "before": "field", "after": "field\nadditional"}
        self.assertEqual(patches.replace_tag("field", patch), "field\nadditional")
        self.assertEqual(patches.replace_tag("field\nadditional", patch), "field\nadditional")
        with self.assertRaises(ValueError):
            patches.replace_tag("field\nadditional\nfield", patch)

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

    def typed_config_fixture(self):
        source = '''package components
import "github.com/Kong/sdk-konnect-go/internal/utils"
type Sample struct {
    Issuer string `json:"issuer"`
    Flag *bool `default:"true" json:"flag"`
    Scopes []string `default:"[openid]" json:"scopes,omitempty"`
    Header *string `json:"header,omitempty"`
}
func (s *Sample) UnmarshalJSON(data []byte) error {
    if err := utils.UnmarshalJSON(data, &s, "", false, []string{"issuer"}); err != nil { return err }
    return nil
}
func (s *Sample) GetFlag() *bool {
    if s == nil { return nil }
    return s.Flag
}
func (s *Sample) GetScopes() []string {
    if s == nil { return nil }
    return s.Scopes
}
func (s *Sample) GetHeader() *string {
    if s == nil { return nil }
    return s.Header
}
'''
        # Match the generated declarations, which always use tab indentation.
        source = source.replace("    ", "\t")
        fields = [["Issuer", "string", "issuer"], ["Flag", "*bool", "flag"],
                  ["Scopes", "[]string", "scopes"], ["Header", "*string", "header"]]
        signature = hashlib.sha256(json.dumps(fields, separators=(",", ":")).encode()).hexdigest()
        policy = {"file": "model.go", "models": {"Sample": {"fields_sha256": signature, "required": ["issuer"]}}}
        return source, policy

    def test_typed_presence_retains_struct_getters_and_required_fields(self):
        source, policy = self.typed_config_fixture()
        result, models = patches.typed_presence(source, policy)
        self.assertRegex(result, r'Issuer\s+string\s+`json:"issuer"`')
        self.assertIn("optionalnullable.OptionalNullable[bool]", result)
        self.assertIn('"github.com/Kong/sdk-konnect-go/optionalnullable"', result)
        self.assertIn("optionalnullable.OptionalNullable[[]string]", result)
        self.assertIn("optionalnullable.OptionalNullable[string]", result)
        self.assertNotIn('default:', result)
        self.assertIn('json:"flag,omitempty"', result)
        self.assertIn('additionalProperties:"true"', result)
        self.assertIn("GetFlag() *bool", result)
        self.assertIn("GetScopes() []string", result)
        self.assertIn("s.Flag.Get()", result)
        self.assertIn("return *value", result)
        again, _ = patches.typed_presence(result, policy)
        self.assertEqual(result, again)
        self.assertEqual(set(models["Sample"]), {"Flag", "Scopes", "Header"})

    def test_typed_presence_rejects_field_and_required_schema_drift(self):
        source, policy = self.typed_config_fixture()
        for changed in [source.replace("Flag *bool", "Flag *int"),
                        source.replace('[]string{"issuer"}', '[]string{"issuer", "flag"}'),
                        source + "\ntype Added struct {}\n"]:
            with self.assertRaises(ValueError):
                patches.typed_presence(changed, policy)

    def test_typed_presence_docs_are_repeatable(self):
        source, policy = self.typed_config_fixture()
        _, models = patches.typed_presence(source, policy)
        doc = "# Sample\n\n## Fields\n\n| Field | Type | Required | Description |\n| --- | --- | --- | --- |\n"
        for name in ["Issuer", "Flag", "Scopes", "Header"]:
            doc += f"| `{name}` | old | optional | Description |\n"
        updated = patches.typed_presence_docs(doc, models["Sample"])
        self.assertIn("`optionalnullable.OptionalNullable[bool]`", updated)
        self.assertIn("`AdditionalProperties`", updated)
        self.assertIn("| `Issuer` | old |", updated)
        self.assertEqual(patches.typed_presence_docs(updated, models["Sample"]), updated)

    def test_typed_presence_reports_missing_json_tag_and_new_digest(self):
        source, policy = self.typed_config_fixture()
        with self.assertRaisesRegex(ValueError, r"Sample.Flag: missing.*json tag"):
            patches.typed_presence(source.replace('default:"true" json:"flag"', 'default:"true"'), policy)
        with self.assertRaisesRegex(ValueError, r"computed fields_sha256=[0-9a-f]{64}; review normalized fields"):
            patches.typed_presence(source.replace("Flag *bool", "Flag *int"), policy)

    def test_typed_presence_supports_single_and_block_imports(self):
        source, policy = self.typed_config_fixture()
        for form in ['import (\n"github.com/Kong/sdk-konnect-go/internal/utils"\n)',
                     'import(\n"github.com/Kong/sdk-konnect-go/internal/utils"\n)',
                     'import "github.com/Kong/sdk-konnect-go/internal/utils"']:
            candidate = source.replace('import "github.com/Kong/sdk-konnect-go/internal/utils"', form)
            result, _ = patches.typed_presence(candidate, policy)
            self.assertEqual(result.count('"github.com/Kong/sdk-konnect-go/optionalnullable"'), 1)
            self.assertEqual(patches.typed_presence(result, policy)[0], result)
        with self.assertRaisesRegex(ValueError, "unsupported import declaration"):
            patches.typed_presence(source.replace('import "github.com/Kong/sdk-konnect-go/internal/utils"', ''), policy)

    def test_typed_presence_docs_preserve_links_and_scope_fields_table(self):
        doc = ("# Example\n\n## Fields\n\n| Field | Type | Required | Description |\n"
               "| --- | --- | --- | --- |\n"
               "| `Key` | [*components.ClientJwk](../../models/components/clientjwk.md) | optional | A key |\n"
               "\n## Other table\n\n| Field | Type |\n| --- | --- |\n| `Key` | unchanged |\n")
        optional = {"Key": ("*ClientJwk", "optionalnullable.OptionalNullable[ClientJwk]", "key")}
        result = patches.typed_presence_docs(doc, optional)
        fields, other = result.split("## Other table")
        self.assertIn("[`optionalnullable.OptionalNullable[ClientJwk]`](../../models/components/clientjwk.md)", fields)
        self.assertIn("`AdditionalProperties`", fields)
        self.assertNotIn("`AdditionalProperties`", other)
        self.assertIn("| `Key` | unchanged |", other)
        self.assertEqual(patches.typed_presence_docs(result, optional), result)
        # Restore links in docs produced by an older version of this patch.
        plain = doc.replace("[*components.ClientJwk](../../models/components/clientjwk.md)",
                            "`optionalnullable.OptionalNullable[ClientJwk]`")
        self.assertIn("../../models/components/clientjwk.md", patches.typed_presence_docs(plain, optional))

    def test_generation_runs_typed_tests_only_after_post_generation_patch(self):
        original = "generate.sdk.speakeasy: speakeasy\ngenerate.sdk:\n\t$(MAKE) _generate.omitempty\n\tgo mod tidy\n"
        result = patches.configure_makefile(original, self.manifest)
        self.assertLess(result.index(patches.POST_GENERATE), result.index("\t$(MAKE) test.kongctl-patches"))
        self.assertLess(result.index("\t$(MAKE) test.kongctl-patches"), result.index("\tgo mod tidy"))
        self.assertIn("go test -count=1 -tags=kongctl_sdk_patched .", result)
        self.assertEqual(patches.configure_makefile(result, self.manifest), result)

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

    def test_documentation_replacements_are_scoped_and_idempotent(self):
        for declaration in ["import (", "import("]:
            example = declaration + '\n\t"context"\n)\nExpiresAt: timestamp,\n'
            source = "## First\n" + example + "## Second\n" + example
            patch = {
                "file": "README.md", "section": "Second",
                "before": "ExpiresAt: timestamp,", "after": "ExpiresAt: nullable(timestamp),",
                "imports": ["example/nullable"],
            }
            result = patches.replace_doc_section(source, patch)
            self.assertEqual(result.split("## Second")[0], source.split("## Second")[0])
            self.assertEqual(result.count('"example/nullable"'), 1)
            self.assertIn("ExpiresAt: nullable(timestamp),", result)
            self.assertEqual(patches.replace_doc_section(result, patch), result)

    def test_documentation_drift_does_not_partially_write_models(self):
        with tempfile.TemporaryDirectory() as directory:
            sdk = Path(directory)
            model, document = sdk / "request.go", sdk / "README.md"
            model.write_text("before")
            document.write_text("## Changed\nexample\n")
            manifest = {
                "post_generation": [{"file": "request.go", "before": "before", "after": "after"}],
                "post_generation_doc_sections": [{
                    "file": "README.md", "section": "Expected", "before": "example",
                    "after": "updated", "imports": [], "optional_file": True,
                }],
            }
            with self.assertRaises(ValueError):
                patches.post_generate(sdk, manifest)
            self.assertEqual(model.read_text(), "before")
            self.assertEqual(document.read_text(), "## Changed\nexample\n")
            document.unlink()
            patches.post_generate(sdk, manifest)
            self.assertEqual(model.read_text(), "after")


if __name__ == "__main__":
    unittest.main()

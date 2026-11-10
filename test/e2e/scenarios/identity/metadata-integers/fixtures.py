"""Compare metadata using exact JSON integers rather than float assertions."""

import json
from pathlib import Path
import re
import sys


def write(work, name, value):
    data = json.dumps(value, indent=2)
    data = re.sub(r'"REF:([^"]+)"', lambda m: "!ref " + m.group(1), data)
    (work / (name + ".yaml")).write_text(data + "\n")


def main():
    action, location = sys.argv[1:3]
    work = Path(location)
    if action == "prepare":
        metadata = {
            "safe": 9007199254740991,
            "boundary": 9007199254740992,
            "exact": 9007199254740994,
            "stable": 1152921504606846976,
            "odd": 9007199254740993,
            "negative": -9007199254740993,
            "maximum": 9223372036854775807,
            "minimum": -9223372036854775808,
            "as_string": "9007199254740993",
            "array": [9007199254740993, -9007199254740993],
        }
        principal = {
            "ref": "numeric-agent", "type": "agent", "display_name": "Numeric agent",
            "labels": {"e2e-fixture": "REF:people#id"}, "metadata": metadata,
        }
        directory = {
            "ref": "people", "_external": {"selector": {"matchFields": {"name": sys.argv[3]}}},
            "principals": [principal],
        }
        value = {
            "_defaults": {"kongctl": {"namespace": "identity-integer-e2e"}},
            "identity": {"directories": [directory]},
        }
        write(work, "initial", value)
        (work / "expected.json").write_text(json.dumps(metadata))
        for name, bad in [
            ("invalid-scalar", 9223372036854775808),
            ("invalid-array", [1, -9223372036854775809]),
            ("invalid-large", 1.25),
        ]:
            changed = json.loads(json.dumps(value))
            changed["identity"]["directories"][0]["principals"][0]["metadata"] = {"identifier": bad}
            write(work, name, changed)
        principal.pop("metadata")
        principal["display_name"] = "Preserved numeric metadata"
        write(work, "omitted", value)
        return {"prepared": True}
    if action == "verify":
        observed = json.loads(Path(sys.argv[3]).read_text())
        expected = json.loads((work / "expected.json").read_text())
        return {
            "exact_metadata": observed["metadata"] == expected,
            "integer_types": all(
                isinstance(observed["metadata"][key], int)
                for key in ["safe", "boundary", "exact", "stable", "odd", "negative", "maximum", "minimum"]
            ),
            "string_preserved": isinstance(observed["metadata"]["as_string"], str),
        }
    raise ValueError("unknown action")


print(json.dumps(main()))

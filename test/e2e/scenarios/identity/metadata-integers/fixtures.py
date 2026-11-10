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
        # Keep identifier precision coverage with strings. Extreme numeric
        # values are tested locally; production currently rounds int64 limits.
        metadata = {
            "employee_id": "001234",
            "external_id": "9223372036854775807",
            "requests_per_minute": 60,
            "retry_budget": 3,
            "adjustment": -1,
            "levels": [1, 2, 3],
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
                for key in ["requests_per_minute", "retry_budget", "adjustment"]
            ),
            "string_identifiers_preserved": all(
                isinstance(observed["metadata"][key], str) for key in ["employee_id", "external_id"]
            ),
        }
    raise ValueError("unknown action")


print(json.dumps(main()))

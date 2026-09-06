"""Generate consumers across two gateways to exercise pagination and identity."""

import json
import pathlib
import sys


gateways = []
for suffix, count in (("primary", 104), ("secondary", 1)):
    gateways.append(
        {
            "ref": f"local-{suffix}",
            "name": f"consumer-pagination-{suffix}",
            "display_name": "Consumer Pagination",
            "consumers": [
                {
                    "ref": f"local-{suffix}-{i:03d}",
                    "name": f"consumer-{i:03d}",
                    "display_name": "Shared Consumer Display",
                    "type": "api-key",
                    "labels": {"parent": suffix},
                }
                for i in range(count)
            ],
        }
    )

config = {
    "_defaults": {"kongctl": {"namespace": "consumer-pagination-e2e"}},
    "ai_gateways": gateways,
}
pathlib.Path(sys.argv[1]).write_text(json.dumps(config, indent=2) + "\n")
print(json.dumps({"gateways": 2, "consumers": 105}))

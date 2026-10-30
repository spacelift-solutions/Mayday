#!/usr/bin/env python3
"""Consume the Tofusible output supplied by a Spacelift stack dependency."""

import json
import os
import sys


def inventory(hosts):
    result = {"_meta": {"hostvars": {}}}
    for host in hosts:
        name = host["host"]
        if not isinstance(name, str) or not name:
            raise ValueError("Every inventory entry needs a host")
        values = {
            "ansible_" + k: v
            for k, v in host.items()
            if k not in ("groups", "extra_vars")
        }
        values.update(host.get("extra_vars", {}))
        result["_meta"]["hostvars"][name] = values
        for group in host.get("groups", ["mayday"]):
            result.setdefault(group, {"hosts": []})["hosts"].append(name)
    return result


if __name__ == "__main__":
    try:
        data = inventory(json.loads(os.environ["TOFUSIBLE_INVENTORY"]))
        print(json.dumps({} if "--host" in sys.argv else data))
    except (KeyError, ValueError, TypeError) as error:
        print(f"Invalid TOFUSIBLE_INVENTORY: {error}", file=sys.stderr)
        sys.exit(1)

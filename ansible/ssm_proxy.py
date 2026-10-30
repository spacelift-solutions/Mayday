"""Connect OpenSSH through SSM using tools in Spacelift's standard AWS runner."""

import json
import re
import subprocess
import sys

import boto3


def connect(instance_id, port, region):
    if not re.fullmatch(r"i-[0-9a-f]{8,17}", instance_id) or port != "22":
        raise ValueError("Mayday SSH requires an EC2 instance ID and port 22")
    client = boto3.client("ssm", region_name=region)
    parameters = {
        "Target": instance_id,
        "DocumentName": "AWS-StartSSHSession",
        "Parameters": {"portNumber": [port]},
    }
    session = client.start_session(**parameters)
    try:
        return subprocess.call(
            [
                "session-manager-plugin",
                json.dumps(session),
                region,
                "StartSession",
                "",
                json.dumps(parameters),
                client.meta.endpoint_url,
            ]
        )
    finally:
        client.terminate_session(SessionId=session["SessionId"])


if __name__ == "__main__":
    sys.exit(connect(*sys.argv[1:]))

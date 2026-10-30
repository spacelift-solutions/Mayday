#!/usr/bin/env python3
"""Run the real target and monitor locally with a disposable license file."""

import os
import shutil
import signal
import subprocess
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def main():
    local = ROOT / ".local"
    local.mkdir(exist_ok=True)
    license_file = local / "demo-license.json"
    if not license_file.exists():
        shutil.copyfile(ROOT / "demo-license.json", license_file)
    env = dict(os.environ, LICENSE_FILE=str(license_file))
    subprocess.run(
        ["go", "build", "-o", str(local / "mayday"), "./cmd/mayday"],
        cwd=ROOT,
        check=True,
    )
    children = []

    def stop(*_):
        for child in children:
            child.terminate()

    signal.signal(signal.SIGINT, stop)
    signal.signal(signal.SIGTERM, stop)
    try:
        for mode, port in (("target", 8081), ("monitor", 8080)):
            cfg = dict(
                env,
                LISTEN_ADDR=f"127.0.0.1:{port}",
                TARGET_URL="http://127.0.0.1:8081",
                STATE_PATH=str(local / "monitor.json"),
                DEMO_PASSWORD=os.environ.get("DEMO_PASSWORD", "mayday-local-demo"),
            )
            # Local runs never forward telemetry to a configured cloud Flow.
            cfg.pop("FLOWS_WEBHOOK_URL", None)
            children.append(
                subprocess.Popen([str(local / "mayday"), mode], cwd=ROOT, env=cfg)
            )
        print("Dashboard: http://127.0.0.1:8080", flush=True)
        print(
            "Local password: use DEMO_PASSWORD, or mayday-local-demo",
            flush=True,
        )
        print(
            "Flows and SSM are not simulated. Restore the local license with: cp demo-license.json .local/demo-license.json",
            flush=True,
        )
        for child in children:
            child.wait()
    finally:
        stop()
        for child in children:
            try:
                child.wait(timeout=10)
            except subprocess.TimeoutExpired:
                child.kill()
                child.wait()


if __name__ == "__main__":
    main()

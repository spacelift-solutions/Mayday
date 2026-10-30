#!/usr/bin/env python3
"""Exercise real HTTP failures and recovery without AWS or Spacelift credentials."""

import http.cookiejar
import json
import os
import secrets
import shutil
import socket
import subprocess
import tempfile
import time
import urllib.error
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
PASSWORD = secrets.token_urlsafe(24)
CLIENT = urllib.request.build_opener(
    urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar())
)


def free_port():
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


def request(url, body=None):
    req = urllib.request.Request(
        url,
        data=None if body is None else json.dumps(body).encode(),
        headers={
            "Content-Type": "application/json",
        },
    )
    with CLIENT.open(req, timeout=5) as response:
        return json.load(response)


def wait_for(fn, description, timeout=45):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        try:
            if fn():
                print("PASS", description)
                return
        except (OSError, ValueError, KeyError, TypeError):
            pass
        time.sleep(0.5)
    raise RuntimeError("Timed out: " + description)


def main():
    with tempfile.TemporaryDirectory(prefix="mayday-smoke-") as tmp:
        path = Path(tmp)
        binary = path / "mayday"
        subprocess.run(
            ["go", "build", "-o", str(binary), "./cmd/mayday"], cwd=ROOT, check=True
        )
        license_file = path / "license.json"
        shutil.copyfile(ROOT / "demo-license.json", license_file)
        target_port, monitor_port = free_port(), free_port()
        target_url, monitor_url = (
            f"http://127.0.0.1:{target_port}",
            f"http://127.0.0.1:{monitor_port}",
        )
        env = dict(
            os.environ,
            LICENSE_FILE=str(license_file),
            DEMO_PASSWORD=PASSWORD,
            TARGET_URL=target_url,
            STATE_PATH=str(path / "state.json"),
            FLOWS_WEBHOOK_URL="",
        )
        children = []

        def start_target(workers):
            p = subprocess.Popen(
                [str(binary), "target"],
                env=dict(
                    env, LISTEN_ADDR=f"127.0.0.1:{target_port}", WORKERS=str(workers)
                ),
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
            )
            children.append(p)
            return p

        def incident(kind, status):
            i = request(monitor_url + "/api/state").get("incident") or {}
            return i.get("kind") == kind and i.get("status") == status

        target = start_target(2)
        monitor = subprocess.Popen(
            [str(binary), "monitor"],
            env=dict(env, LISTEN_ADDR=f"127.0.0.1:{monitor_port}"),
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
        children.append(monitor)
        try:
            wait_for(
                lambda: request(target_url + "/readyz")["status"] == "healthy",
                "initial license accepted",
            )
            wait_for(
                lambda: request(monitor_url + "/api/login", {"password": PASSWORD})[
                    "authenticated"
                ],
                "password login creates a session",
            )
            wait_for(
                lambda: request(monitor_url + "/api/state")["observation"]["reachable"],
                "independent monitor connected",
            )
            request(monitor_url + "/api/fault/license", {})
            wait_for(
                lambda: incident("dependency", "open"),
                "license deletion detected independently",
            )
            shutil.copyfile(ROOT / "demo-license.json", license_file)
            wait_for(
                lambda: incident("dependency", "recovered"),
                "license recovery verified with real work",
            )
            request(monitor_url + "/api/traffic", {"rate": 22})
            wait_for(
                lambda: incident("capacity_up", "open"),
                "real load exceeds two-worker capacity",
            )
            target.terminate()
            target.wait(timeout=10)
            target = start_target(8)
            wait_for(
                lambda: incident("capacity_up", "recovered"),
                "eight workers recover under unchanged load",
            )
            request(monitor_url + "/api/traffic", {"rate": 2})
            wait_for(
                lambda: incident("capacity_down", "open"),
                "low load requests scale-down after cooldown",
                timeout=60,
            )
            target.terminate()
            target.wait(timeout=10)
            target = start_target(2)
            wait_for(
                lambda: incident("capacity_down", "recovered"),
                "baseline capacity verified",
            )
            target.terminate()
            target.wait(timeout=10)
            wait_for(
                lambda: incident("outage", "open"), "actual process outage detected"
            )
            target = start_target(2)
            wait_for(
                lambda: incident("outage", "recovered"), "process restart verified"
            )
        finally:
            for child in children:
                if child.poll() is None:
                    child.terminate()
            for child in children:
                child.wait(timeout=10)


if __name__ == "__main__":
    main()

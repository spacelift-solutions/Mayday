"""Check encrypted Secrets, then validate the manifests Flux builds."""

import subprocess
from pathlib import Path

import yaml


def main():
    kms = yaml.safe_load(Path(".sops.yaml").read_text())["creation_rules"][0]["kms"]
    for path in Path("kubernetes").glob("*.yaml"):
        for document in yaml.safe_load_all(path.read_text()):
            if document.get("kind") != "Secret":
                continue
            recipients = document.get("sops", {}).get("kms", [])
            if not any(key.get("arn") == kms for key in recipients):
                raise ValueError(f"{path}: Secret must use the Flux SOPS key")
            for field in ("data", "stringData"):
                if not all(
                    isinstance(value, str)
                    and value.startswith("ENC[")
                    and value.endswith("]")
                    for value in document.get(field, {}).values()
                ):
                    raise ValueError(f"{path}: Secret contains unencrypted values")
    rendered = subprocess.run(
        [
            "flux",
            "build",
            "kustomization",
            "mayday",
            "--path",
            "kubernetes",
            "--kustomization-file",
            "validation/mayday.yaml",
            "--dry-run",
        ],
        capture_output=True,
        text=True,
        check=True,
    )
    subprocess.run(
        [
            "kubeconform",
            "-strict",
            "-summary",
            "-kubernetes-version",
            "1.35.0",
        ],
        input=rendered.stdout,
        text=True,
        check=True,
    )


if __name__ == "__main__":
    main()

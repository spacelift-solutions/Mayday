# Project name

Describe what this project does and who it is for.

## Getting started

Clone the repository, then install the shared Spacelift Solutions Git hooks:

```sh
git clone <repository-url>
cd <repository>
make hooks
```

`make hooks` clones or updates
[`spacelift-solutions/git-hooks`](https://github.com/spacelift-solutions/git-hooks)
and runs its installer. The local hooks run the same repository checks used in
CI. Repository rulesets and the CI workflow are configured separately when the
repository is created.

Document any additional prerequisites, setup steps, and the first command a new
contributor should run.

## Development

Document the commands used to build, test, lint, and run the project locally.

## Releases

The [release workflow](.github/workflows/release.yml) uses
[`NerdsWhoFish/quill`](https://github.com/NerdsWhoFish/quill)
to validate release mechanics on every pull request.

To publish, run the workflow manually from `main`, choose **Release** or
**Release Candidate**, and select a `patch`, `minor`, or `major` increment. The
workflow calculates versions from final `vMAJOR.MINOR.PATCH` tags. Release
candidates use `vMAJOR.MINOR.PATCH-rc.N`; they increment `N` without consuming
the corresponding final version.

The starter publishes a source-only GitHub Release. Projects that build release
artifacts can add their publisher configuration and change `publish: none` in
the workflow to a supported Quill publisher such as `goreleaser`. Quill
restricts releases to `main`, validates before tagging, and removes a failed
release's tag so the version can be retried.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for the contribution workflow.

## Architecture decisions

Significant technical decisions are recorded as Architecture Decision Records
(ADRs) in [adr/](adr/README.md). Add an ADR when a decision has meaningful
alternatives, is costly to reverse, and will need its reasoning preserved for
future contributors.

## License

This project is licensed under the [MIT License](LICENSE).

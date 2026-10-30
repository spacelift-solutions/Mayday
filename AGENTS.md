# AGENTS.md

Guidance for coding agents working in this repository. Keep this file short and
update it as project conventions become established.

## Start here

- Read [README.md](README.md) for the project's purpose and setup.
- Read [CONTRIBUTING.md](CONTRIBUTING.md) before making changes.
- Read the accepted decisions in [adr/](adr/README.md) before changing
  architecture.
- Treat the repository as the source of truth when an issue or task conflicts
  with the code or documentation.

## Working agreement

- Work on a branch and open a pull request; do not commit or push to `main`.
- Keep changes focused and avoid unrelated cleanup.
- Match existing patterns before introducing new abstractions.
- Never commit credentials, private keys, tokens, or generated secrets.
- Add or update tests when behavior changes.
- Update documentation when interfaces or workflows change.
- Run the relevant checks before considering work complete.
- Do not edit generated files directly; update their source and regenerate them.

## Architecture decisions

Follow accepted ADRs. Create a new ADR only when a decision has meaningful
alternatives, is costly to reverse, and needs its reasoning preserved. Routine
implementation choices do not need an ADR.

## Project-specific guidance

Add the project's language, directory layout, commands, testing conventions,
and other non-obvious constraints here as they become known.

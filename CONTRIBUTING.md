# Contributing

Thank you for contributing.

## First clone

1. Clone the repository and create a branch from `main`.
2. Run `make hooks` to install the shared repository checks.
3. Install the project's documented dependencies.
4. Run the project's checks before making changes.

Add project-specific setup and check commands here as the repository takes
shape.

## Developing

- Keep each change focused on one concern.
- Match the conventions already established in the repository.
- Add or update tests when behavior changes.
- Update documentation when interfaces or workflows change.
- Never commit credentials, private keys, or generated secrets.

## Pull requests

Before opening a pull request:

1. Run the relevant format, lint, test, and build commands.
2. Review the complete diff for unrelated or generated changes.
3. Explain the problem and the fix in the pull request description.
4. Call out follow-up work explicitly instead of hiding it in the change.

All paths are owned by the team listed in
[CODEOWNERS](.github/CODEOWNERS), whose review may be required before merge.

## Architecture decisions

Record decisions with meaningful alternatives and long-term consequences in
[adr/](adr/README.md). Do not create an ADR for routine implementation details.

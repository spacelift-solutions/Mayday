# Architecture Decision Records

Use [Markdown Architectural Decision Records](https://adr.github.io/madr/) to
document significant technical decisions, their context, and their
consequences.

## Index

Keep this table current when adding or superseding an ADR.

| ADR | Title | Status |
| --- | --- | --- |
| _None yet_ | | |

## When to write an ADR

Write an ADR when a decision has real alternatives, is costly to reverse, and
is likely to make a future contributor ask why the system works this way.
Routine implementation details belong in code or project documentation.

## Process

1. Copy the template below into `NNNN-short-title.md` using the next available
   number.
2. Describe the context, alternatives, decision, and consequences.
3. Add the ADR to the index above.
4. Discuss the decision in its pull request.
5. When a later ADR supersedes it, update the index and link both decisions.

## Template

```markdown
---
status: proposed
date: YYYY-MM-DD
---

# Decision title

## Context and Problem Statement

What decision needs to be made, and why?

## Considered Options

- Option one
- Option two

## Decision Outcome

Chosen option: "...", because ...

## Consequences

- Good, because ...
- Bad, because ...
```

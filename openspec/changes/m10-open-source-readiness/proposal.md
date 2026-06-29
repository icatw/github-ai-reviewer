## Why

M9 made the deployed service operationally manageable, and the project is now strong enough to show on a resume. The remaining gap before public open source is packaging: a new reader should understand what the project does, how to configure a GitHub App safely, how to run or deploy it, what is intentionally out of scope, and what files must never be committed.

## What Changes

- Rewrite the repository README as a public-facing project entry point with clear scope, architecture, quick start, configuration, deployment, operations, safety, and roadmap sections.
- Add open-source setup material for GitHub App permissions, webhook configuration, local smoke testing, production deployment, and systemd/nginx operation, linking to existing production and operations runbooks.
- Add a publication safety check that detects tracked/staged secrets, local config, generated binaries, private evidence, and missing public-readiness documentation.
- Add public project metadata such as an explicit license file and contribution guidance if absent.
- Preserve product behavior: M10 does not change webhook semantics, GitHub permissions, LLM review behavior, workspace checkout, analyzer execution, PR comments, Check Run policy, or live deployment behavior.

## Capabilities

### New Capabilities

- `open-source-readiness`: Public README, setup/deployment guidance, contribution/license metadata, and release safety checks for publishing the repository.

## Impact

- Affected docs: `README.md`, possible `CONTRIBUTING.md`, existing deployment/operation docs cross-links.
- Affected scripts: add or update a safety/publication check under `scripts/`.
- Affected metadata: add a license file if missing.
- No production service restart is required unless documentation validation discovers a configuration issue.
- No changes to review logic, GitHub API behavior, LLM prompts, checkout behavior, analyzer commands, comments, Check Runs, dashboard, billing, or blocking policy.

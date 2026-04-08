# Skill Registry

**Delegator use only.** Any agent that launches sub-agents reads this registry to resolve compact rules, then injects them directly into sub-agent prompts. Sub-agents do NOT read this registry or individual SKILL.md files.

See `_shared/skill-resolver.md` for the full resolution protocol.

## User Skills

| Trigger | Skill | Path |
|---------|-------|------|
| When user says "judgment day", "judgment-day", "review adversarial", "dual review", "doble review", "juzgar", "que lo juzguen". | judgment-day | /home/gary/.agents/skills/judgment-day/SKILL.md |
| When writing Go tests, using teatest, or adding test coverage. | go-testing | /home/gary/.agents/skills/go-testing/SKILL.md |
| When user asks to create a new skill, add agent instructions, or document patterns for AI. | skill-creator | /home/gary/.agents/skills/skill-creator/SKILL.md |
| When creating a pull request, opening a PR, or preparing changes for review. | branch-pr | /home/gary/.agents/skills/branch-pr/SKILL.md |
| When creating a GitHub issue, reporting a bug, or requesting a feature. | issue-creation | /home/gary/.agents/skills/issue-creation/SKILL.md |

## Compact Rules

Pre-digested rules per skill. Delegators copy matching blocks into sub-agent prompts as `## Project Standards (auto-resolved)`.

### judgment-day
- Parallel Blind Review: Launch TWO independent judges via `delegate` (async); they must not know about each other.
- Verdict Synthesis: Compare results; Confirm (both), Suspect (one), Contradiction (disagree).
- Warning Classification: `WARNING (real)` (affects production) vs `WARNING (theoretical)` (contrived/malicious scenario).
- Theoretical warnings are reported as INFO and do NOT block or trigger re-judgment.
- Fix & Re-judge: 0 confirmed CRITICALs + 0 confirmed real WARNINGs = APPROVED.
- Convergence: Only re-judge if confirmed CRITICALs remain after Round 1. Fix real WARNINGs/SUGGESTIONs inline.

### go-testing
- Use Table-Driven Tests for multiple test cases (name, input, expected, wantErr).
- Test Bubbletea models by simulating `Update` calls with `tea.KeyMsg` or `tea.Msg`.
- Use `teatest.NewTestModel` for full interactive TUI flow integration tests.
- Use Golden File testing (`testdata/*.golden`) for visual output/View() validation.
- Mock system dependencies (OS, ARM, HomeDir) via interfaces for controlled environments.
- Commands: `go test ./...` (run all), `go test -update` (update goldens), `go test -short` (skip integrations).

### skill-creator
- Create skills only for repeatable patterns or complex project-specific workflows.
- Skill Structure: `skills/{name}/SKILL.md` (required), `assets/` (templates), `references/` (local docs).
- SKILL.md must have frontmatter (name, description + trigger, license: Apache-2.0, author, version).
- Compact Rules section in SKILL.md is CRITICAL: 5-15 lines of actionable patterns.
- `references/` MUST point to LOCAL files (e.g., `docs/*.md`), never external web URLs.
- Register new skills by adding them to `AGENTS.md`.

### branch-pr
- Branch Naming: `type/description` (e.g., `feat/user-login`). Allowed: feat, fix, chore, docs, style, refactor, perf, test, build, ci, revert.
- Conventional Commits: `type(scope): description`. Breaking changes use `!`.
- Every PR MUST link an approved issue (`Closes #N`) with `status:approved` label.
- Every PR MUST have exactly one `type:*` label (type:feature, type:bug, type:docs, type:refactor, type:chore, type:breaking-change).
- Automated checks (Issue Reference, Approval, Type Label, Shellcheck) must pass before merge.

### issue-creation
- Blank issues are disabled; use Bug Report or Feature Request templates.
- Every issue automatically gets `status:needs-review`.
- A maintainer MUST add `status:approved` before any PR can be opened for that issue.
- Bug Reports require: Pre-flight checks, Description, Steps to Reproduce, Expected vs Actual, OS, Agent, Shell.
- Search for duplicates before creating. Questions go to Discussions, not Issues.

## Project Conventions

| File | Path | Notes |
|------|------|-------|
| AGENTS.md | AGENTS.md | Index — references SDD lifecycle and Engram protocols |

Read the convention files listed above for project-specific patterns and rules. All referenced paths have been extracted — no need to read index files to discover more.

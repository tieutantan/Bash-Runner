# Agent Instructions

## UI/UX standards

Any frontend, component, or layout work follows [.agents/rules/ui-ux.md](.agents/rules/ui-ux.md) by default, without needing to be asked. It covers spacing and type scales, color tokens, responsive layout, micro-interactions, component architecture, accessibility, and loading/empty/error states.

For design-system choices and focused lookups, use the skill at [.agents/skills/ui-ux-pro-max/SKILL.md](.agents/skills/ui-ux-pro-max/SKILL.md). It requires Python 3 and runs locally with no network access.

## Build after code changes

After any change to Go code (`main.go`, `go.mod`, or assets embedded in the app such as `icon.png`), rebuild the app without waiting to be asked:

```bash
./build.sh
```

This regenerates `Bash Runner.app` in the project root, so the app the user opens reflects the latest code. Before reporting the task as done, confirm the build succeeds. If it fails, report the error output instead of claiming the change is ready.

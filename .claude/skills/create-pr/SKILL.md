---
name: create-pr
description: Use when asked to create, open, or submit a pull request on GitHub. Guides pre-collection, self-assessment, template filling, and PR submission via GitHub MCP or gh CLI.
---

# Create PR

## When to Use

Use this skill whenever asked to create, open, or submit a pull request.

## Workflow

### Step 1: Pre-collection

Gather context before writing anything.

Run:
```bash
git diff main...HEAD --stat
git log main...HEAD --oneline
```

Identify the linked issue number from the branch name, recent commits, or conversation context.

### Step 2: Self-assessment

Before filling the template, answer these questions internally:

1. Which decisions did I make that I am uncertain about?
2. What alternatives did I consider and why did I not use them?
3. What tests exist that cover this change?
4. What did I actually run or verify during implementation?

These answers become the content of **AI Confidence Notes** and **AI Verification Done**.

### Step 3: Fill the template

Fill every `[AI]` section in `.github/pull_request_template.md`. No section may be left blank or skipped.

| Section | What to write |
|---|---|
| **Closes** | Replace `#<issue number>` with the actual GitHub issue number (e.g., `Closes: #42`) identified in Step 1 |
| **Summary** | One paragraph describing what this PR does |
| **What Changed** | What was changed and why |
| **Impact Scope** | Which components/APIs/users are affected and how significantly |
| **Implementation Decision** | Key decisions and why this approach was chosen over alternatives |
| **Test Coverage** | What tests cover the purpose of this PR |
| **AI Verification Done** | What was actually run (build, tests, flash, manual check, etc.) |
| **AI Confidence Notes** | Uncertainties or decisions needing human judgment. Write "No significant uncertainties." if fully confident. |

### Step 4: PR creation

**Title:** Write a natural, descriptive title that accurately represents the change. Do not use Conventional Commits format.

#### Priority: GitHub MCP

Use the `create_pull_request` MCP tool when available:

```
create_pull_request(
  owner: <repo owner>,
  repo: <repo name>,
  title: "<descriptive title>",
  body: "<filled template body>",
  head: "<current branch>",
  base: "main"
)
```

#### Fallback: gh CLI

If MCP is unavailable, use:

```bash
gh pr create --title "<descriptive title>" --base main --body "$(cat <<'EOF'
<filled template content>
EOF
)"
```

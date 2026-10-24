---
name: commit-message
description: 'Write a Git commit message for staged changes. Use when: writing commit messages, committing code, drafting commit text, summarizing changes for a commit.'
argument-hint: 'Optional context about why the change was made'
---

# Write Commit Message

## When to Use

- User asks to write, draft, or generate a commit message
- User is preparing to commit and wants a summary of changes
- User asks to describe their staged or current changes for a commit

## Procedure

1. **Gather changes**: Use the `get_changed_files` tool with `sourceControlState: ['staged']` to retrieve the staged diffs. If the tool is unavailable, fall back to `git diff --cached --stat` and `git diff --cached`. If nothing is staged, fall back to `['unstaged']` and note that the changes are not yet staged. If the tool is unavailable there too, use `git diff --stat` and `git diff`.

2. **Analyse the diff**: Read through all changed files. Identify:
   - The primary intent of the change (bug fix, feature, refactor, chore, docs, etc.)
   - Which components or subsystems are affected
   - Why the change was made — prioritize the user-provided argument first, then code comments, then commit history

3. **Write the commit message** following these rules:

### Subject Line

Format: `<type>(<scope>): <description>` (Conventional Commits)

1. Pick a type: `feat`, `fix`, `refactor`, `chore`, `docs`, `test`, `ci`, `build`, `perf`, `style`
2. Pick a scope (optional but encouraged): the package, app, or subsystem name
3. Write the description in **imperative mood**, **50 chars or fewer**, no trailing period
   - For `fix` commits, describe the **problem**, not the solution
     (e.g., `fix(authz): global admins denied access`)
   - Describe the problem as observed, not its internal cause
     (e.g., `fix(gce): quotas never alert`, not `fix(gce): keys built from response metric`)
4. **Never** add "Co-authored-by" trailers

### Body

- Separate from subject with a blank line
- Hard-wrap each body paragraph to **72 characters or fewer per line**
- Reflow paragraphs manually before output; do not emit a single long line
- **Keep it short** — 1–2 sentences per topic, no more
- Use **proper sentences**, not bullet points
- Use a **separate paragraph** for each distinct topic
- Explain **why** at a high level — do not describe implementation detail
  (e.g. "which query was changed" or "which files were edited")
- The **what** should be obvious from the subject; the body adds motivation
- Before returning the message, validate it with the following
  commands and revise it if either reports a violation:

  ```bash
  # Subject must be 50 characters or fewer
  echo "$SUBJECT" | awk 'length > 50 {print "Subject too long ("length" chars): "$0; exit 1}'

  # Every body line must be 72 characters or fewer
  echo "$BODY" | awk 'length > 72 {print "Line "NR" too long ("length" chars): "$0; err=1} END{exit err}'
  ```

### Example

```
fix(authz): global admins denied access

Ensure global admins have access to all projects without
requiring per-project policy updates.

Add X-Request-ID to auth responses so access failures are
easier to trace across services.
```

4. **Present the message**:
  - If `.git/COMMIT_EDITMSG` is open in the editor, insert the commit message at the top of that file and stop at the first blank line so the existing commented git context remains below it.
  - Otherwise, output the commit message in a fenced code block so the user can review and copy it.

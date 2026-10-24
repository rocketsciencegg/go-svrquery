---
applyTo: '**/COMMIT_EDITMSG'
---

# Commit Message Conventions

- Follow conventional commits specification
- Title must be 50 characters or fewer
- Body must be short: 1–2 sentences per topic, no more
- Use proper sentences instead of bullet points
- Use a separate paragraph for each distinct topic
- Do not describe implementation detail — explain why at a high level
- The subject says what; the body says why
- For `fix` commits, the subject describes the **problem that was fixed**, not what was done to fix it (e.g., "global admins denied access" not "allow global admins access")
- The problem is the one a user or operator observes, not the internal cause (e.g., "quotas never alert" not "keys built from the response metric")
- Do NOT add any "Co-authored-by" trailers to commit messages

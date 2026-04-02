---
name: No git commands
description: User handles all git operations - do not run git add, commit, push, or other git commands
type: feedback
---

Do not run git commands (add, commit, push, etc). User handles all git operations themselves.

**Why:** User explicitly rejected a git commit and said "do not run git commands."

**How to apply:** Skip all commit steps in plans. Only implement code and verify it compiles/passes tests. Report what files changed so the user can stage and commit.

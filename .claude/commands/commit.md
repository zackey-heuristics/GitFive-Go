Review the working tree before committing.

1. Run `git status` to inspect all changed, staged, and untracked files.
2. Run `git diff --staged` and `git diff` to review both staged and unstaged changes.
3. Run `git log --oneline -5` to match recent commit message style.
4. Stage only the files that are part of the logical change by explicit path. Never use `git add -A` or `git add .`. Do not stage files that likely contain secrets (.env, credentials.json, tokens, etc.) — warn the user if such files appear in the working tree.
5. Write a conventional commit message using prefixes like `feat:`, `fix:`, `docs:`, `ci:`, `chore:`, or `test:`.
6. Keep the commit message to one line, lowercase, imperative mood, and under 72 characters.
7. If `$ARGUMENTS` is provided, treat it as guidance for the commit message.
8. Commit the staged changes and include the standard Co-Authored-By trailer for the current Claude model.
9. If the commit fails due to a pre-commit hook, fix the issue, re-stage, and create a NEW commit. Never use --amend after a hook failure.
10. Ask the user whether they want to push to the remote. If yes, push with `git push origin HEAD -u` and report the commit hash, branch name, and remote URL.

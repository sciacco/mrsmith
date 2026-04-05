# Plan: Save feedback memory about audit verification practice

## Context
During the Appsmith audit, I delegated page audits to 8 parallel agents and synthesized their reports into a summary table. I incorrectly marked 2 of 4 hidden pages as visible because I relied on reading verbose agent reports instead of cross-checking the raw source files. The user caught this and wants to ensure trustworthy output going forward.

## Action
Save a feedback memory capturing the lesson: when producing summary tables from multi-agent or multi-source synthesis, always run a targeted verification query against the raw data before writing the final output.

## Files to modify
- `/Users/sciacco/devel/mrsmith/.claude/projects/-Users-sciacco-devel-mrsmith/memory/feedback_verify_summaries.md` (new)
- `/Users/sciacco/devel/mrsmith/.claude/projects/-Users-sciacco-devel-mrsmith/memory/MEMORY.md` (add index entry)

## Verification
- Check that the memory file exists and has correct frontmatter
- Check that MEMORY.md includes the new entry

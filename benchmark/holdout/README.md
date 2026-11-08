# Held-out test calls

These calls measure whether Chaukanna works on **situations it was never tuned on**.

## Rules (they make the numbers honest)

1. **New calls only.** Never put a call here that was used to fix rules or prompts
   (calls #1–#12 from the first speakerphone session are already in `../cases/`).
2. **Never tune on these.** Don't change phrases, rules or prompts because of a
   call in this folder. If you must fix something, *move* the call to `../cases/`
   and record a fresh one for here.
3. **Friends use their own words.** Describe the scam type ("pretend to be from the
   electricity board and get money"), don't hand them a script.
4. **Include normal calls too** (about 1 in 3), so false alarms are measured.

## Current status (3 November 2026)

- Sets A and B (70 calls) were moved to `../dev/` and used to tune the rules, following rule 2. **Scores on them are tuned scores, not held-out.**
- Set C (the 40 calls here) was scored once with the frozen rules: see `../results.md`. It has now been used, so **new calls from friends are the next honest test**.
- Set C was also run once with old rules plus gpt-oss-120b as a baseline. Those per-call results were not used for tuning.

## Adding calls

Label new calls in the dashboard, and export them to `../fresh` so they stay apart from set C: `go run ./cmd/exportcalls -labelled -from <first call number> -out ../benchmark/fresh`. Report scripted calls separately. The older way:

Make the calls on the Speakerphone page, note their numbers (`#14`, `#15` … on the
Calls page), then:

```bash
cd backend
go run ./cmd/exportcalls -scam 14,15,17 -legit 16,18
```

## Measuring

```bash
cd backend
BENCHMARK_DIR=../benchmark/holdout go run ./cmd/benchmark           # rules (+ Evon if configured)
```

Report these numbers as the **held-out** result, separately from `../cases/`.

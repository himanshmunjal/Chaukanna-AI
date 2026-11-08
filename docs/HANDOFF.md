# Chaukanna: handoff for a new chat

Updated 5 November 2026. Start a new chat with: "Read docs/HANDOFF.md and continue."

## What Chaukanna is

An AI guardian for phone scams, for the Gnani AI Great Indian AI Internship Challenge. It listens to a call, warns out loud in the person's language, and alerts the family. Go backend, React dashboard. See `README.md`.

## Competition deadline

**Final submissions close at 11:59 PM IST on 10 November 2026** (gnani.ai/internship). Needs a public LinkedIn or X post that is live before submitting, with the demo video, what was built and the Gnani technology used, both hashtags (#GreatIndianAIInternshipChallenge, #GnaniAI) and a tag of Gnani AI. The submission form needs the video, a written explanation, models used, each member's contribution and one nominated post link per platform. Teams are at most two. The LinkedIn post is in `PITCH.md`. It quotes ₹22,495 crore (ThePrint, MHA data); the user's ₹22,925 wasn't found in any source.

## Rules to keep

- **Keep scores honest.** Don't tune rules or prompts on any call in `benchmark/holdout/` or `benchmark/fresh/`, or on the 30 `llm_test` calls in `benchmark/simple-calls.csv`. Don't read `benchmark/results/gpt-oss/holdout-gpt-oss-120b.json`. Report scripted calls separately from friends' own-words calls.
- **Label models truthfully.** `backend/.env` uses `openai/gpt-oss-120b` on Groq. Never call those results Evon. Only report Evon results that were actually run, and name the build. The user asked on 5 November about claiming a RunPod test that hadn't happened, and was told not to.
- **Benchmarks run rules-only unless asked:** put `EVON_BASE_URL= EVON_MODEL=` before the command. Groq allows 8,000 tokens a minute.
- **Test servers need every alert channel off** and their own `SQLITE_PATH`, because `.env` has live Telegram and Twilio settings. zsh doesn't split `$VAR` into words, and unmatched globs abort commands (`setopt nonomatch`).
- **Check the file before editing:** gofmt changes spacing.
- **Permanently deleting data is the user's step.** Archive first, and give the command. Git and deployment are also the user's steps.

## The Evon test is finished

Run on 5 November on a rented A100 with vLLM. Results are in `benchmark/results.md`, the README and `PITCH.md`. Report as Evon v3.3 30B full precision, replayed text, no speed quoted because another project shared the GPU. The user was told to terminate the pod and delete the volume: confirm that happened, because billing continues otherwise.

**Live calls switched on 5 November.** `LLM_PLUS_RULES` in `backend/.env`: `false` means no model is called anywhere, including Screen Mode; `true` means the rules' warnings stand and the model can add more (`-mode adds`), which caught the most scams on every set (26/28, 19/22, 59/90; false alarms 3/12, 2/12, 5/60). `.env` still points at gpt-oss-120b on Groq: on the 34 staged calls that scored 20/22, 3/12 false alarms, 8/12 counting cautions, with 7 of 72 requests failing. Don't quote Evon's numbers for it. Fuller private charts come from `benchmark/scripts/make-review-charts.py` into `benchmark/charts/review/`, which isn't committed. Not yet tried on a live call. A lower warning threshold for Evon hasn't been tried; don't choose it on set C.

**Benchmark folders:** sets stay in `benchmark/` (`cases`, `dev`, `holdout`, `staged`, `simple`, `noisy`, `.tts-cache`). Scores are in `benchmark/results/{rules,evon,gpt-oss,noise}/`, charts in `benchmark/charts/`, scripts in `benchmark/scripts/`. In `results/evon/`, `*-evon.json` is "Evon reviews rules", `*-evon-adds.json` is "Rules + Evon", and `*-evon-only.json` is Evon alone. `cmd/benchmark` defaults to `-mode adds`.

The user asked for only good results in the README. They were told the false-alarm numbers stay, because leaving them out would mislead.

## Results so far

Details and caveats are in `benchmark/results.md`.

| Set | Rules only | Rules + gpt-oss-120b (replay) |
|---|---|---|
| Set C, sealed, typed, older rules | 22/28 scams, 2/12 false alarms | Not run on current code |
| 34 staged calls in `benchmark/staged/` | 18/22, 2/12 wrongly warned, 5/12 counting cautions | 19/22, 3/12, 8/12; 1.8 s median |
| `benchmark/simple-calls.csv`, 150 typed | 45/90, 4/60 | Not run |

- The live scam figure is 16/22, because three of those calls (#40, #46, #50) were used to fix the rules. #60-71 are normal calls, live on rules only.
- **gpt-oss-120b, reviewing rule hits, didn't fix false alarms.** Its reasons treated warnings about scams, a police verification and a shop's QR code as scams.
- Evon: see the section above.

## Decisions waiting on the user

1. **Change the LLM's instructions before the 30 live calls?** Recommended yes: say that talking about a scam isn't a scam, that being sent to a branch or counter isn't a money demand, and stop showing the model the rules' red flags. That last idea is an untested guess. Tuning this on the 34 staged calls means they no longer count as unseen for the LLM, and the 30 `llm_test` calls become the fair score. Prompt not changed yet, so Evon runs on the same prompt as gpt-oss.
2. **The 30 live LLM calls** (`llm_test = yes`, 21 scam, 9 normal). Start the backend with:

```bash
cd backend
EVON_INTERVAL_SECONDS=15 DASHBOARD_PASSWORD=choose-a-password go run ./cmd/server 2>&1 | tee /tmp/chaukanna.log
```

3. **Cleanup:** `sqlite3 chaukanna.db < archive/delete-test-calls.sql` from `backend/`, with the backend stopped. At last check it hadn't been run. It keeps #36-40 and #43-71. **Calls after #71 would be deleted**, so add any new numbers to its lists first and take a fresh backup. The newest backup is `backend/archive/chaukanna-before-cleanup-2026-11-05.db`.

## Open problems

- **Exotel warning not heard on #31:** cause unknown. On a fake Exotel client the audio left the server valid. `exotel.go` logs `sent clip-N` and `played clip-N` from Exotel's `mark` reply, but the `played` path has never been confirmed live, because no log was saved. Show the warning on speakerphone in the demo unless it's heard on Exotel again. If `played` appears but nobody hears it, look at the merge. If it's missing or instant, send audio at speaking pace and check the Exotel flow is Voicebot.
- **`DASHBOARD_PASSWORD` not set at last check,** so `/api/calls` answered without login through ngrok.
- **Prisma changes words:** on #58 "jurmana" became "पक्का जरूर", and on #57 the final demand is missing. No LLM can recover those.
- **False-warning risk that stays even with an LLM:** a direct-demand rule hit can't be overruled. Mixed-script safety advice can raise a flag, as on #57 ("OTP share मत कीजिए").

## How detection works now

With `LLM_PLUS_RULES=true` and a model configured, the rules run on every line and their warnings stand, and the model checks each new rule hit and every `EVON_INTERVAL_SECONDS`, and can raise a warning. If a request fails, the rules carry on. On Groq use `EVON_INTERVAL_SECONDS=15`, or rate-limited requests fall back to rules. `SetModelDecides` and `SetModelOnly` remain for benchmarks only.

## State of the rest

| Area | Status |
|---|---|
| Speakerphone, Telegram alerts | Working on real calls |
| Exotel (ExoPhone 08047290103, 7-day trial) | Worked on #28-30, then the problem above. Token goes in the URL path: `/ws/exotel/<token>` |
| Screen Mode on a real forwarded call | Not tested |
| Demo mode, key per visitor, login, rate limits, encryption, Docker image | Tested locally, not hosted |
| Phone app and 10 interface languages | Checked at phone size in headless Chrome, not on a real phone. Machine-translated and unreviewed; `npm run check:i18n` guards them. Advice text in `backend/internal/report/report.go` is matched by its exact English |
| Regional warnings | Not reviewed by native speakers |
| Reminder, read-out interrupt, talk-out reply, follow-up alerts, Safe button | Replayed text only, not on a real call |
| Kavach | The competitor is github.com/sarabjeetsingh1708/kavach. Other products share the name |

Test call scripts: `docs/test-call-scripts.csv` (scripts 1-44 appear to be used, matched by content up to #71). The 10 own-words calls from friends haven't been made; those are the fairest numbers to quote. Calls #31-35, #41 and #42 don't count. Scripted calls were not labelled in the dashboard at last check; `benchmark/staged/` holds labels from the user's notes.

## Still to do

- Finish the Evon test and report it
- Demo video labelled as staged, and fill the placeholders in `PITCH.md` (real-call results, demo link, video link, team)
- Team names in the README, native-speaker review, Git and deployment by the user
- Change `STORE_TRANSCRIPTS=all` to `flagged` once calls are exported
- Deploy with `DASHBOARD_PASSWORD`, `ENCRYPTION_KEY`, `DEMO_MODE=true`, `DASHBOARD_URL`, `TRUSTED_PROXIES`

Calls #14-27 and their archive were deleted by the user on purpose. Nothing more to do there.

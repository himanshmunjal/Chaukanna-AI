# Results

Measured 3 November 2026. **Rules only, with no LLM.** Rules were frozen before set C was scored, and set C was scored once.

**The current rules have moved on slightly.** Since this score, the detector also reacts when someone starts to comply on a doubtful call, which can raise such a call to a scam. Set C was re-run once on the current rules on 5 November, totals only, for the Evon comparison, and rules alone still scored 22/28 and 2/12. The rules changed again on 4 November after staged calls #40, #46 and #50: see "First staged batch" below. The friends' calls will be the first score of the current version.

## The number to quote: calls the rules never saw (set C, 40 calls)

| | Old rules | New rules, typed transcripts | New rules, audio, clean | New rules, audio, 5 dB street noise |
|---|---|---|---|---|
| Scams caught | 5/28 (18%) | **22/28 (79%)** | **23/28 (82%)** | **23/28 (82%)** |
| Warned or cautioned | not measured | 24/28 (86%) | not reported | not reported |
| False alarms | 1/12 | **2/12 (17%)** | 2/12 | 2/12 |
| Median time to detect | 6 s | 6 s | 11.8 s | 11.8 s |

Audio means Timbre voices → 8 kHz μ-law phone line → street noise → Prisma v3 → detector.

**How far to trust this**

- It is a small sample: 22/28 has a 95% range of roughly 60-90%, and 2/12 roughly 5-45%.
- False alarms went up by one call, which is too few to read as a trend, but they did not improve.
- The audio is synthetic. These are not real people on real phone lines, and synthetic noise is cleaner than the real thing.
- Set C was not perfectly blind. The earlier report listed its call titles, and a few of its lines were seen while checking file formats. Its misses were not read.
- A few set C audio lines appeared in a progress log during the run. The rules were already frozen by then.

## Tuned sets: not evidence of generalising

| Set | Old rules | New rules |
|---|---|---|
| `dev/` (sets A and B, 70 calls, tuned on) | 14/51, false alarms 2/19 | 51/51, false alarms 0/19 |
| `cases/` (20 calls, tuned on) | 14/14, 0/6 | 14/14, 0/6 |

The drop from 100% on tuned calls to about 80% on unseen ones is the real overfitting gap.

## Not measured

- Any LLM on a live call. The LLM results below replay text. The reasoning layer, Evon or gpt-oss-120b, is **not** in the table at the top. `results/gpt-oss/holdout-gpt-oss-120b.md` is a separate baseline: old rules plus gpt-oss-120b.
- A scored test with real human callers and real phone lines. It's in progress: see below.
- Regional languages at scale. Almost all of set C is Hindi, Hinglish or English. Regional vocabulary is written, but only the warnings were tested.

## Real calls so far: working, not scored

A few staged calls were made through Exotel on 3 November 2026. They show the system works on a real phone line. They're too few and too staged to be a score.

| Call | What it showed |
|---|---|
| #28, direct call | Connected, recognised the protected person, transcribed Hindi clearly, and stayed silent while fraud was only being discussed |
| #29, direct call | Warned 8.7 s in, at the OTP demand, and sent a Telegram alert. It missed Dadi reading out "1 2 3", which led to a fix |
| #30, merged call | Add call → Merge worked, with both voices heard and the warning played. The caller then talked Dadi out of it, which led to the talk-out reply and follow-up alerts |

The fixes from #29 and #30 have been tested with replayed text, not yet on a real call.

## First staged batch: #36-50, live result before any fix

Measured 4 November 2026, rules only, speakerphone. **Not Exotel, and not a final score: this is one small batch of staged scam calls, with no normal calls yet.**

| | Live result |
|---|---|
| Scams warned during the call | **11/13** |
| Missed | #40 stayed at 50% risk, #46 at 25% |
| Caught late | #50, warned at 29.3 s of a 41 s call |

#41 and #42 were abandoned after 13 s and 3 s, and #35 had no speech, so they don't count.

**Use these numbers, not a re-run.** The rules were then changed because of #40, #46 and #50, so re-running these calls now flags all 13 (#46 at 14.3 s, #40 at 15.4 s, #50 at 16.0 s). That shows the fix works on the calls it was written for, not that the rules generalise. The tuned sets stayed at 51/51 with 0/19 false alarms and 14/14 with 0/6. Set C wasn't re-run. The added rules haven't been tested on new normal calls, so false alarms on those are the thing to watch.

## Second staged batch: #51-59, rules frozen

Measured 4 November 2026, speakerphone, staged scam calls, with the rules as changed after the first batch. Nothing was tuned on this batch. The live calls ran rules only. The LLM column is a replay of the saved transcripts, so it is not a live result, and it includes no normal calls, so it says nothing about false alarms.

| | Rules only, live | Rules + gpt-oss-120b on Groq, replay |
|---|---|---|
| Flagged | 5/9 | 7/9 |
| Not flagged | #55, #56, #58, #59 | #55 (risk 0.35), #58 (risk 0.55, one of its LLM requests failed) |

The 5/9 includes #57, which was flagged on a line of safety advice ("OTP share मत कीजिए"), not on the demand. The rules misread that line, and the same line from a genuine bank would raise the same warning. Prisma also changed some words: on #58 "jurmana" became "पक्का जरूर", and on #57 the final demand isn't in the transcript. An LLM can't recover words that aren't there.

## Staged calls with normal calls: #36-40, #43-59 and #60-71

Measured 5 November 2026. #60-71 are normal calls made live on rules only, and they are that group's first result. The LLM column is a replay of saved transcripts with `EVON_INTERVAL_SECONDS=15`, on a new setting where the model decides on rule hits that aren't direct demands. That setting was prompted by these normal calls, and the LLM column is not a live result.

| | Rules only | Rules + gpt-oss-120b |
|---|---|---|
| Scams warned | 18/22 (16/22 live) | 19/22 |
| Normal calls wrongly warned | 2/12 | 3/12 |
| Normal calls wrongly warned or cautioned | 5/12 | 8/12 |
| Median time to warn | 16 s | 16.6 s |
| LLM median response | n/a | 1.8 s |
| LLM requests | n/a | 70 ok, 2 failed; failed decisions used rules |

Both false-alarm ranges overlap widely (2/12 is about 5-45%, 3/12 about 9-53%), so this doesn't show that either is better. It doesn't support the idea that this model and prompt fix false alarms. The model's reasons treated warnings about scams, a police verification and a shop's QR code as scams.

## Evon v3.3 30B, full precision

Measured 5 November 2026 with `gnani/gnani-evon-v3.3-30B-A3B` on vLLM on a rented A100 80 GB, thinking off, checks every 15 seconds. The prompt was not changed for Evon. Replays of text only, not live calls.

| Set | | Rules only | Rules + Evon | Evon reviews rules | Evon only |
|---|---|---|---|---|---|
| Set C, 28 scams, 12 normal | Scams warned | 22 | 26 | 13 | 7 |
| | Warned or cautioned | 24 | 28 | 28 | 27 |
| | Normal wrongly warned | 2 | 3 | 2 | 0 |
| | Normal warned or cautioned | 2 | 3 | 3 | 1 |
| Staged, 22 scams, 12 normal | Scams warned | 18 | 19 | 14 | 7 |
| | Warned or cautioned | 18 | 21 | 20 | 18 |
| | Normal wrongly warned | 2 | 2 | 0 | 0 |
| | Normal warned or cautioned | 5 | 7 | 5 | 6 |
| Simple, 90 scams, 60 normal | Scams warned | 45 | 59 | 45 | 16 |
| | Warned or cautioned | 46 | 79 | 80 | 73 |
| | Normal wrongly warned | 4 | 5 | 1 | 1 |
| | Normal warned or cautioned | 5 | 5 | 4 | 3 |

**Rules + Evon:** the rules' warnings stand, and Evon can add more (`-mode adds`, saved as `*-evon-adds.json`). **Evon reviews rules:** Evon decides on rule hits, except direct demands and read-out codes (`-mode decides`, saved as `*-evon.json`). **Evon only:** no rules (`-mode only`).

Tuned sets, Evon reviews rules: `dev/` 34/51 warned, 51/51 warned or cautioned, 0/19 false alarms; `cases/` 11/14, 14/14, 0/6. Evon only and Evon adds weren't run on these.

**How far to trust this**

- Evon often found the scam but scored it under the 0.8 needed for a warning, so "warned" and "warned or cautioned" differ a lot. Changing that threshold hasn't been tried.
- Rules + Evon was the best at catching scams on all three sets, so live calls now use this logic when `LLM_PLUS_RULES=true`. Evon itself hasn't run on a live call, and these numbers don't carry over to another model.
- Set C was re-run for this on current rules, totals only, so it is no longer scored just once. Rules-only still gave 22/28 and 2/12. Choosing a setup after seeing set C would make its number slightly optimistic, though the other two sets agree on the order.
- These samples are small: 26/28 has a 95% range of roughly 77-98%, and 3/12 roughly 9-53%.
- 1 of 1,631 Evon requests failed, in Evon only on the simple set, and that decision used no rule.
- Another project shared the GPU during these runs, so no speed is quoted.
- The test calls and rules have the same author, except where stated. No friends' own-words calls have been scored.

## Rules + gpt-oss-120b on the 34 staged calls

Measured 5 November 2026, replaying text with `-mode adds`, the same logic as live calls with `LLM_PLUS_RULES=true`, using `openai/gpt-oss-120b` on Groq and checks every 15 seconds. **20/22 scams warned, 3/12 normal calls wrongly warned, 8/12 counting cautions.** 7 of 72 requests failed, 6 of them timeouts or dropped connections, and those decisions used rules alone, so this isn't a pure LLM score. On the same calls, rules + Evon scored 19/22, 2/12 and 7/12; rules alone 18/22, 2/12 and 5/12. The sample is too small to separate these.

## Simple typed set: 150 calls, rules only

Measured 5 November 2026 on `simple-calls.csv`: **45/90 scams warned (50%, range 40-60%), 4/60 normal calls wrongly warned (7%, range 3-16%)**, 46/90 and 5/60 counting cautions. Written by the same assistant as the rules, in Prisma's mixed script, with no audio, and in families of 6 similar calls, so 150 overstates how independent the sample is. The calls marked `llm_test` (21 scam, 9 normal) are set aside for live LLM calls and haven't been run through any LLM.

## Warnings: spoken and transcribed back

`go run ./cmd/warningcheck` on all 10 languages: 88-97% match on warnings and cautions, except the Hindi warning at 55% and caution at 83%, where the transcript is correct but Prisma writes English words in Latin script. This shows the audio is clear, not that the wording is natural.

## Reproduce

```bash
cd backend
EVON_BASE_URL= EVON_MODEL= BENCHMARK_DIR=../benchmark/holdout go run ./cmd/benchmark
EVON_BASE_URL= EVON_MODEL= BENCHMARK_DIR=../benchmark/holdout go run ./cmd/noisebench -snr clean,5 -out ../benchmark/results/rules/holdout-c-audio.md
```

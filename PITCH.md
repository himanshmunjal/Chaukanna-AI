# Chaukanna: a guardian that joins the call

**₹22,495 crore. You heard it right: that's the least Indians lost to cyber fraud in 2025, from investment scams to "digital arrest".**

Scam calls are won in the moment, while the victim is alone on the line. Chaukanna joins that moment: it listens, warns out loud in the person's own language, and brings the family in before money moves.

## The problem

"Digital arrest", KYC-expiry and "your son is in custody" calls follow a script: authority, fear, isolation, urgency, extraction. Their targets are often elderly, alone and speaking Hindi, a regional language or Hinglish. Family usually finds out after the money has gone.

## What it does

1. **It joins the call.** The person taps Add call → Chaukanna → Merge, with no app needed. Or they put the call on speaker next to a second device. Unknown callers who aren't picked up can be forwarded to Chaukanna to answer.
2. **It recognises the scam as it unfolds.** Safety rules run first, and a reasoning model can add to them but can't overrule a direct demand for an OTP.
3. **It warns out loud.** Everyone on the call hears a pre-written warning, never model output, because a frightened person may act on every word.
4. **It doesn't stop there.** If the caller asks again or says the warning is false, it answers. If digits are being read out, it interrupts.
5. **It brings the family in.** They get a Telegram alert, can watch the call live, and can step in with their own words. Afterwards they get the steps to take, 1930, and a complaint draft to check and file themselves.

## How Gnani AI powers it

| Model | Used for |
|---|---|
| **Prisma** | Live speech-to-text on phone audio, including reconnecting past its session limit and adding up an OTP split across lines |
| **Timbre** | Warnings in 10 languages, an optional family member's cloned voice with recorded consent, and the voice of the practice scammer |
| **Evon** | The reasoning model, self-hosted for our benchmarks because it isn't on Gnani's API yet. Not yet tried on a live call |

## What we measured

On 40 calls the rules had never seen, rules only, with no LLM:

| | Typed transcripts | Synthetic audio with 5 dB street noise |
|---|---|---|
| Scams caught | 22/28 (79%) | 23/28 (82%) |
| False alarms | 2/12 | 2/12 |

This is a small sample: 22/28 has a 95% range of roughly 60-90%, and the audio is synthetic. The rules have changed slightly since this score. On the calls the rules were tuned on the score is 100%, which is why we quote the unseen set instead. Results and caveats are in [`benchmark/results.md`](benchmark/results.md).

**With Evon v3.3 30B, on replayed text:** with Evon adding to the rules, Chaukanna warned on **26/28** scams on the same unseen set, against 22/28 on rules alone, with 3/12 false alarms against 2/12. On 34 staged phone calls it warned on 19/22 scams, with 2/12 false alarms. Evon hasn't yet run on a live call. Charts and details are in the [README](README.md#evon-measured).

**So far on a real phone line:** staged calls through Exotel, with the warning played on the call. All demo calls are staged re-enactments.

## What's different

- **It joins the live call**, instead of only listening beside it.
- **It answers back** when the caller attacks the warning, and interrupts when someone starts to comply.
- **The family can step in**, rather than only being told.
- **People can practise** with Scam Drill, a pretend call that gives feedback on each answer.
- **It's honest about its numbers:** one test set was held back until the rules were frozen, and our caveats are published with the score.

## What isn't done yet

Regional detection isn't measured at scale, and the regional wording isn't reviewed by native speakers. Screen Mode hasn't been tested on a real forwarded call. On one Exotel call the warning wasn't heard, and the cause isn't known yet. The product also needs legal review before real-world use.

## Try it

- **Live demo:** _add the link after deploying_
- **Demo video:** _add the link, labelled as staged_
- **Team:** _add names_

`#GnaniAI` `#GreatIndianAIInternshipChallenge`

---

## LinkedIn post

Upload the demo video directly to the post, and tag Gnani AI by typing `@Gnani` and choosing the company page.

```text
₹22,495 crore.

That's how much Indians lost to cyber fraud in 2025, and that's only the minimum, according to Ministry of Home Affairs data.

Sometimes, all it takes is one phone call:

"Your son is in police custody."
"Your KYC has expired."
"You are under digital arrest."

By the time the family finds out, the money is often gone.

So how do we stop it? There are many ways, like calling 1930 and talking to family before paying anyone. Today, I'm introducing one more:

Chaukanna (चौकन्ना): an AI guardian that listens to a suspicious call and speaks up while it's still happening.

Here's how it works:

👂 It listens to the call on speakerphone, using Gnani AI's Prisma for speech-to-text
🧠 It looks for scam patterns, using safety rules plus Gnani AI's Evon as the reasoning layer
🔊 It warns out loud with Gnani AI's Timbre, with warnings written in 10 languages
👨‍👩‍👧 It alerts the family on Telegram, so they can watch live and step in
📝 Afterwards, it gives the next steps, the 1930 helpline, and a complaint draft to check and file

What we measured:

On a test set of 40 typed calls held back while tuning the rules, Chaukanna warned on 26 of 28 scams with Evon, up from 22 on rules alone, with 3 false alarms on 12 normal calls.

To be clear: these results are from replayed text, and the calls in the video are staged. It hasn't been tested on real scam calls yet, and there's a lot more to improve.

Watch the video and tell me: who in your family would you want this for?

Built for the Great Indian AI Internship Challenge by @Gnani AI.

#GreatIndianAIInternshipChallenge #GnaniAI #CyberSecurity #ArtificialIntelligence #DigitalArrest
```

Source for the figure: [ThePrint, 21 February 2026](https://theprint.in/india/cybercrime-saw-24-spike-in-2025-indians-lost-rs-22495-crore-mainly-in-investment-scams/2859930/). Investment fraud made up about 76% of that money, so the total isn't all from phone calls.

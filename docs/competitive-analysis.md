# How Chaukanna compares

Written and updated 3 November 2026. **Kavach's repository and the voice-cloning part of gnani-dub were read as code.** Everything else comes only from the competitors' own posts, so their products were not tried and their claims were not checked.

## Who the competition really is

| Project | What it is | Overlap with Chaukanna | Idea worth taking |
|---|---|---|---|
| **Kavach** | AI scam-call shield on speaker, Telegram alerts | **Direct competitor** | See below |
| Polyglot | Live translation inside a normal phone call | Same telephony approach | Barge-in, streaming TTS |
| Dukaan | Hinglish voice ordering | None | Live URL visitors can try with their own key |
| Mic Drop Arcade | Voice-controlled RPG | None | Voice as a game → **Scam Drill** |
| CivicVoice | Voice civic complaints | Low | Generate the form, review before submit → **complaint draft** |
| JalVaani | Noise-resilient STT with human review | Low | Consent screens; publish WER |
| gnani-dub | Dubs YouTube, optional voice cloning | Low | Gnani's clone API → **family voice** |
| BoloBodh | Voice learning platform | None | Learning by practice |
| Orbita | Video meetings with voice translation | None | None for this product |
| Bulblu | AI and human companionship | Indirect (the isolation scammers exploit) | None directly |

## Chaukanna against Kavach

| | Kavach | Chaukanna |
|---|---|---|
| Joins a call with no app or spare device | ❌ Stated as future work | ✅ Add call → Merge worked on a staged call through Exotel |
| Answers a caller who discredits the warning | ❌ | ✅ And alerts the family |
| AI answers unknown callers | ❌ | ✅ Screen Mode |
| Remembers scam numbers | ❌ | ✅ |
| Family can step into the live call | ❌ | ✅ Warn now, or speak their own words |
| Warning in a family member's voice | ❌ | ✅ With recorded consent |
| Practice calls | ❌ | ✅ Scam Drill |
| Complaint draft for cybercrime.gov.in | ❌ | ✅ |
| Interrupts when the person starts to comply | ❌ | ✅ |
| Public demo, safe for strangers to try | Not documented | ✅ Isolated visitors, own key, no real alerts |
| Streaming speech-to-text | ❌ 4-second chunks | ✅ |
| Test set written by someone else | ❌ They say the same person wrote scripts and rules | ✅ Held-out sets, one kept sealed |
| Check a recording after the call | ✅ Automatic from the phone (MacroDroid) | 🟡 Manual upload, no automation |
| Installable phone app | ✅ Single-screen PWA: one big listen button, 1930 button, offline app shell. Install through the browser menu | ✅ PWA with bottom navigation, app shortcuts, an install prompt with iPhone steps, offline advice with a 1930 button, vibration on the warning. More screens, so less simple than Kavach's single page |
| Interface in the elder's language | ✅ 4 languages, all screens | ✅ 10 languages on the screens used during and after a call, with a first-launch picker. Setup and test screens are in English, and translations aren't reviewed by native speakers |
| Measured with an LLM on noisy audio | ✅ 264 runs with Evon | ❌ **Not yet: see below** |

### Where Kavach is still ahead

1. **Measured LLM results.** Kavach published Evon's figures on noisy audio. Chaukanna's published figures are rules-only so far.
2. **Automatic recording checks.** The family does nothing after setup.
3. **A published demo video** with real app footage.

Be careful comparing headline numbers. Kavach's 97–100% and Chaukanna's sealed result are not on the same test: Kavach's are on calls its author wrote, and Chaukanna's are on calls its rules never saw. Our own tuned score is 100%, so a tuned score says little.

## Phone numbers without paying

No provider found offers a permanently free phone number with call streaming. They offer trials or limited free tiers:

| Option | What's free | Catch |
|---|---|---|
| **Exotel trial** (best fit) | [7 days, about ₹500 of usage](https://exotel.com/?p=1809), [no KYC to start](https://developer.exotel.com/docs/getting-started/trial-account) | Short, and credits can't be topped up. Chaukanna already supports its Voicebot stream |
| LiveKit Cloud | [One US number, 50 inbound minutes](https://docs.livekit.io/telephony/start/phone-numbers) | US number, so calling it from India costs money, and Chaukanna would need new integration code |
| Telnyx trial | [Trial credit](https://developers.telnyx.com/pricing) | [Calls only to and from your verified number](https://developers.telnyx.com/docs/account-setup/using-trial-account.md), and new integration code |
| Speakerphone mode | Everything, no number needed | The call must be on speaker beside the device |

## What was taken from whom

| Idea | From | In Chaukanna |
|---|---|---|
| Never speak model output; speak checked lines | Kavach (Evon once gave bad advice) | Warnings are pre-written; the model only decides and explains |
| Normalise nukta and half-nasal spellings | Kavach | `prefilter.normalize` |
| Caution tier below the alarm | Kavach | Soft spoken caution, no family alert |
| Speak it, transcribe it back | Kavach | `cmd/warningcheck`. It caught "1930" being read as a quantity |
| Telegram alerts | Kavach | Private per-person links |
| Voice cloning | gnani-dub | Family voice |
| Voice as a game | Mic Drop Arcade | Scam Drill |
| Generate, then review before submitting | CivicVoice | Complaint draft; a person always files it |
| Consent screens | JalVaani | Consent recorded per person and per voice |

Deliberately not copied: Kavach's rule that red needs both layers to agree. It cost Kavach some recall (100% to 97%), and it can't be judged here until Chaukanna has LLM results.

## Noisy-call benchmark

20 calls · noise: **babble** · Timbre timbre-v2.5 → 8 kHz μ-law phone line → Prisma prisma-v3.0 · rules only · 2026-11-02

| Noise level | Scams caught | False alarms | Median time-to-detect | Transcript match vs clean |
|---|---|---|---|---|
| Clean phone line | 100% (14/14) | 0% (0/6) | 14.4 s | 100% (reference) |
| 10 dB (crowded room) | 93% (13/14) | 0% (0/6) | 15.8 s | 90% |
| 5 dB (crowded market) | 86% (12/14) | 0% (0/6) | 15.8 s | 72% |
| 0 dB (another voice as loud as the caller) | 29% (4/14) | 0% (0/6) | 16.0 s | 24% |

<details><summary>Per call</summary>

| Call | Label | clean | 10 | 5 | 0 |
|---|---|---|---|---|---|
| legit-01-emi-reminder-english | legit | ✅ 0% | ✅ 0% | ✅ 0% | ✅ 0% |
| legit-02-delivery-hinglish | legit | ✅ 0% | ✅ 0% | ✅ 50% | ✅ 0% |
| legit-03-family-hindi | legit | ✅ 0% | ✅ 0% | ✅ 0% | ✅ 0% |
| legit-04-clinic-appointment-english | legit | ✅ 0% | ✅ 0% | ✅ 0% | ✅ 25% |
| legit-05-police-verification-hinglish | legit | ✅ 25% | ✅ 25% | ✅ 45% | ✅ 45% |
| legit-06-sbi-emi-reminder | legit | ✅ 25% | ✅ 25% | ✅ 25% | ✅ 25% |
| scam-01-digital-arrest-hinglish | scam | ✅ 95% | ✅ 95% | ✅ 95% | ✅ 95% |
| scam-02-fedex-parcel-hindi | scam | ✅ 95% | ✅ 95% | ✅ 95% | ❌ 0% |
| scam-03-kyc-otp-hinglish | scam | ✅ 95% | ✅ 95% | ✅ 95% | ❌ 15% |
| scam-04-anydesk-refund-english | scam | ✅ 95% | ✅ 95% | ✅ 95% | ✅ 90% |
| scam-05-electricity-cutoff-hinglish | scam | ✅ 90% | ✅ 90% | ❌ 0% | ❌ 25% |
| scam-06-son-in-custody-hinglish | scam | ✅ 95% | ❌ 70% | ✅ 95% | ✅ 90% |
| scam-07-kyc-tamil-english | scam | ✅ 95% | ✅ 95% | ✅ 95% | ❌ 0% |
| scam-08-lottery-kbc-hinglish | scam | ✅ 90% | ✅ 90% | ✅ 95% | ✅ 95% |
| scam-09-sbi-otp-english | scam | ✅ 90% | ✅ 90% | ✅ 95% | ❌ 50% |
| scam-10-tech-support-screen-share | scam | ✅ 90% | ✅ 90% | ✅ 90% | ❌ 0% |
| scam-11-sbi-manager-otp-hindi | scam | ✅ 90% | ✅ 90% | ✅ 90% | ❌ 45% |
| scam-12-phone-repair-screen-share | scam | ✅ 90% | ✅ 90% | ✅ 90% | ❌ 45% |
| scam-13-canada-police-bail-english | scam | ✅ 95% | ✅ 95% | ✅ 95% | ❌ 0% |
| scam-14-ac-service-otp-hindi | scam | ✅ 95% | ✅ 95% | ❌ 50% | ❌ 0% |

</details>

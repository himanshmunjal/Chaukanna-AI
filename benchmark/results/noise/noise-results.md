## Noisy-call benchmark

20 calls · noise: **street** · Timbre timbre-v2.5 → 8 kHz μ-law phone line → Prisma prisma-v3.0 · rules only · 2026-11-02

| Noise level | Scams caught | False alarms | Median time-to-detect | Transcript match vs clean |
|---|---|---|---|---|
| Clean phone line | 100% (14/14) | 0% (0/6) | 14.4 s | 100% (reference) |
| 10 dB (busy street) | 100% (14/14) | 0% (0/6) | 14.4 s | 97% |
| 5 dB (loud traffic) | 100% (14/14) | 0% (0/6) | 14.4 s | 96% |
| 0 dB (noise as loud as the voice) | 100% (14/14) | 0% (0/6) | 14.3 s | 95% |

<details><summary>Per call</summary>

| Call | Label | clean | 10 | 5 | 0 |
|---|---|---|---|---|---|
| legit-01-emi-reminder-english | legit | ✅ 0% | ✅ 0% | ✅ 0% | ✅ 0% |
| legit-02-delivery-hinglish | legit | ✅ 0% | ✅ 0% | ✅ 0% | ✅ 0% |
| legit-03-family-hindi | legit | ✅ 0% | ✅ 0% | ✅ 0% | ✅ 0% |
| legit-04-clinic-appointment-english | legit | ✅ 0% | ✅ 0% | ✅ 0% | ✅ 0% |
| legit-05-police-verification-hinglish | legit | ✅ 25% | ✅ 25% | ✅ 25% | ✅ 25% |
| legit-06-sbi-emi-reminder | legit | ✅ 25% | ✅ 25% | ✅ 25% | ✅ 25% |
| scam-01-digital-arrest-hinglish | scam | ✅ 95% | ✅ 95% | ✅ 95% | ✅ 95% |
| scam-02-fedex-parcel-hindi | scam | ✅ 95% | ✅ 95% | ✅ 95% | ✅ 95% |
| scam-03-kyc-otp-hinglish | scam | ✅ 95% | ✅ 95% | ✅ 95% | ✅ 95% |
| scam-04-anydesk-refund-english | scam | ✅ 95% | ✅ 95% | ✅ 95% | ✅ 95% |
| scam-05-electricity-cutoff-hinglish | scam | ✅ 90% | ✅ 90% | ✅ 90% | ✅ 90% |
| scam-06-son-in-custody-hinglish | scam | ✅ 95% | ✅ 95% | ✅ 95% | ✅ 95% |
| scam-07-kyc-tamil-english | scam | ✅ 95% | ✅ 95% | ✅ 95% | ✅ 95% |
| scam-08-lottery-kbc-hinglish | scam | ✅ 90% | ✅ 90% | ✅ 90% | ✅ 90% |
| scam-09-sbi-otp-english | scam | ✅ 90% | ✅ 90% | ✅ 90% | ✅ 90% |
| scam-10-tech-support-screen-share | scam | ✅ 90% | ✅ 90% | ✅ 90% | ✅ 90% |
| scam-11-sbi-manager-otp-hindi | scam | ✅ 90% | ✅ 90% | ✅ 90% | ✅ 90% |
| scam-12-phone-repair-screen-share | scam | ✅ 90% | ✅ 90% | ✅ 90% | ✅ 90% |
| scam-13-canada-police-bail-english | scam | ✅ 95% | ✅ 95% | ✅ 95% | ✅ 95% |
| scam-14-ac-service-otp-hindi | scam | ✅ 95% | ✅ 95% | ✅ 95% | ✅ 95% |

</details>

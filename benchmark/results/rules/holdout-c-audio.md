## Noisy-call benchmark

40 calls · noise: **street** · Timbre timbre-v2.5 → 8 kHz μ-law phone line → Prisma prisma-v3.0 · rules only · 2026-11-03

| Noise level | Scams caught | False alarms | Median time-to-detect | Transcript match vs clean |
|---|---|---|---|---|
| Clean phone line | 82% (23/28) | 17% (2/12) | 11.8 s | 100% (reference) |
| 5 dB (loud traffic) | 82% (23/28) | 17% (2/12) | 11.8 s | 97% |

<details><summary>Per call</summary>

| Call | Label | clean | 5 |
|---|---|---|---|
| ho-c01-scam | scam | ✅ 90% | ✅ 90% |
| ho-c02-scam | scam | ✅ 95% | ✅ 95% |
| ho-c03-scam | scam | ✅ 90% | ✅ 90% |
| ho-c04-scam | scam | ✅ 90% | ✅ 90% |
| ho-c05-scam | scam | ✅ 95% | ✅ 95% |
| ho-c06-scam | scam | ✅ 90% | ✅ 90% |
| ho-c07-scam | scam | ✅ 95% | ✅ 95% |
| ho-c08-scam | scam | ❌ 45% | ❌ 45% |
| ho-c09-scam | scam | ✅ 95% | ✅ 95% |
| ho-c10-scam | scam | ✅ 90% | ✅ 90% |
| ho-c11-scam | scam | ✅ 95% | ✅ 95% |
| ho-c12-scam | scam | ✅ 95% | ✅ 95% |
| ho-c13-scam | scam | ❌ 70% | ❌ 70% |
| ho-c14-scam | scam | ✅ 95% | ✅ 95% |
| ho-c15-scam | scam | ❌ 60% | ❌ 60% |
| ho-c16-scam | scam | ✅ 90% | ✅ 90% |
| ho-c17-scam | scam | ✅ 95% | ✅ 95% |
| ho-c18-scam | scam | ❌ 50% | ❌ 50% |
| ho-c19-scam | scam | ✅ 90% | ✅ 90% |
| ho-c20-scam | scam | ✅ 95% | ✅ 95% |
| ho-c21-scam | scam | ✅ 90% | ✅ 90% |
| ho-c22-scam | scam | ✅ 95% | ✅ 95% |
| ho-c23-scam | scam | ✅ 80% | ✅ 80% |
| ho-c24-scam | scam | ✅ 95% | ✅ 95% |
| ho-c25-scam | scam | ✅ 90% | ✅ 90% |
| ho-c26-scam | scam | ❌ 45% | ❌ 45% |
| ho-c27-scam | scam | ✅ 95% | ✅ 95% |
| ho-c28-scam | scam | ✅ 95% | ✅ 95% |
| ho-c29-legit | legit | ✅ 35% | ✅ 35% |
| ho-c30-legit | legit | ✅ 0% | ✅ 0% |
| ho-c31-legit | legit | ✅ 0% | ✅ 0% |
| ho-c32-legit | legit | ✅ 45% | ✅ 45% |
| ho-c33-legit | legit | ✅ 25% | ✅ 25% |
| ho-c34-legit | legit | ✅ 0% | ✅ 0% |
| ho-c35-legit | legit | ✅ 25% | ✅ 25% |
| ho-c36-legit | legit | ✅ 50% | ✅ 50% |
| ho-c37-legit | legit | ✅ 0% | ✅ 0% |
| ho-c38-legit | legit | ❌ 80% | ❌ 80% |
| ho-c39-legit | legit | ❌ 80% | ❌ 80% |
| ho-c40-legit | legit | ✅ 0% | ✅ 0% |

</details>

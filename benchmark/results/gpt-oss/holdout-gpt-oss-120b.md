# Held-out results: rules vs rules + gpt-oss-120b

Measured Nov 3, 2026 on 110 held-out calls (79 scams, 31 normal).

**Evon was not used.** Gnani confirmed Evon is not on its API yet and allowed another LLM for the submission. These results are for `openai/gpt-oss-120b` on Groq.

| Set | Scams caught, rules | Scams caught, LLM | False alarms, rules | False alarms, LLM |
|---|---|---|---|---|
| **All** | 19/79 | 73/79 | 3/31 | 5/31 |
| A | 5/21 | 21/21 | 1/9 | 1/9 |
| B | 9/30 | 27/30 | 1/10 | 2/10 |
| C | 5/28 | 25/28 | 1/12 | 2/12 |

- Rules + LLM recall: 92% (95% interval 84–96%). False alarms: 16% (95% interval 7–33%).
- Median time to detect: 12.6 s. LLM median response: 1626 ms.
- LLM requests: 258 succeeded, 1 failed. In the failed one (`ho-c19-scam`) the model ran out of its 800-token answer limit, rules decided that step, and the call was missed.

## Errors
| Call | Error | Risk | Caught or flagged by rules? | Model's reason |
|---|---|---|---|---|
| `ho-a22-legit` | false alarm | 0.85 | yes | Multiple red‑flags (authority claim, OTP and personal data request) indicate a scam. |
| `ho-bN01-legit` | false alarm | 0.80 | yes | Caller pretends to be bank staff and mentions OTP, a typical scam pattern. |
| `ho-bN08-legit` | false alarm | 0.80 | no | Caller claims to be a LIC official, a typical impersonation red flag. |
| `ho-bS11-scam` | missed scam | 0.40 | no | The caller promises unrealistic guaranteed profits, a classic scam indicator. |
| `ho-bS15-scam` | missed scam | 0.40 | no | The call promises an unexpected subsidy and asks to scan a QR code, a common scam tactic. |
| `ho-bS29-scam` | missed scam | 0.45 | no | Caller requests immediate QR payment, a common scam tactic. |
| `ho-c08-scam` | missed scam | 0.78 | no | Caller claims a mistaken transfer and asks for money back, a classic scam. |
| `ho-c19-scam` | missed scam | 0.25 | no | Rule-based check found: authority impersonation. |
| `ho-c25-scam` | missed scam | 0.65 | no | The caller pressures for high‑value gift cards with urgent language, a classic scam pattern. |
| `ho-c29-legit` | false alarm | 0.80 | no | Only an authority claim is present without any request for money or OTP, so risk is low. |
| `ho-c38-legit` | false alarm | 0.85 | yes | The caller pretended to be a bank and asked for an OTP, classic scam behavior. |

`ho-c29-legit` was flagged even though the model's reason text says risk is low. The cause has not been investigated.

## How to read this
- These are written text transcripts, so they do not test Prisma, noise or real speech.
- Rules were frozen for this baseline: the version from before any tuning on sets A and B.
- The run stopped at 87 calls on Groq's daily token limit and resumed about 22 hours later. The original binary had been deleted, so the last 23 calls used a rebuild from saved snapshots. The rebuild reproduced every rules-only result on all 110 calls, apart from the order of two reasons in `ho-c04-scam`'s text, and its LLM client and guardian prompt are byte-identical.
- Sets A and B have since been used to tune rules, so later scores on them are development scores. Set C can be a final test only if nothing is tuned on it, and this run has scored it once.

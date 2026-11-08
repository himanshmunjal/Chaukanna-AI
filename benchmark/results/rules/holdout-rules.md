## Held-out benchmark: rules only (no Evon)

110 calls never used for tuning (79 scam, 31 normal) · text transcripts from 3 independent sets · rules and prompts unchanged · 2026-11-02

| Set | Scams caught | Marked suspicious but not warned | False alarms | Median time-to-detect |
|---|---|---|---|---|
| Set A (pasted, 30) | 5/21 (23%) | 6 | 1/9 | 10 s |
| Set B (call_scam_test_dataset.csv, 40) | 9/30 (30%) | 6 | 1/10 | 18 s |
| Set C (scam_call_test_data.csv, 40) | 5/28 (17%) | 11 | 1/12 | 6 s |
| **All** | **19/79 (24%)** | 23 | **3/31 (10%)** | 10 s |

### False alarms (normal calls warned)

- `ho-a22-legit` Official Bank Transaction Verification (set A, pasted): risk 0.95, Rule-based check found: authority impersonation, otp request, personal data request.
- `ho-bN01-legit` Genuine bank transaction alert (set B, call_scam_test_dataset.csv): risk 0.90, Rule-based check found: authority impersonation, otp request.
- `ho-c38-legit` telling family about scam (set C, scam_call_test_data.csv): risk 0.90, Rule-based check found: authority impersonation, otp request.

### Missed scams (by final rule score)

| Call | Risk | What the rules found |
|---|---|---|
| `ho-a07-scam` AI Voice Cloning / Relative in Emergency | 0.70 | authority impersonation, fear threat, urgency |
| `ho-c02-scam` bank kyc otp | 0.70 | authority impersonation, fear threat, urgency |
| `ho-bS25-scam` Fake Aadhaar update | 0.70 | personal data request, otp request |
| `ho-c17-scam` aadhaar suspended | 0.70 | personal data request, otp request |
| `ho-c23-scam` pm kisan scheme | 0.70 | otp request, personal data request |
| `ho-c15-scam` kbc whatsapp | 0.60 | too good to be true, isolation |
| `ho-a09-scam` Fake Customs / Courier Delivery Hold | 0.55 | authority impersonation, fear threat |
| `ho-bS01-scam` Electricity disconnection | 0.55 | authority impersonation, fear threat |
| `ho-bS05-scam` Courier drug parcel | 0.55 | fear threat, authority impersonation |
| `ho-c04-scam` courier customs | 0.55 | fear threat, authority impersonation |
| `ho-c06-scam` sim deactivation | 0.55 | authority impersonation, fear threat |
| `ho-c20-scam` card blocked bank manager | 0.55 | authority impersonation, fear threat |
| `ho-a10-scam` SIM Card Expiry / Porting Scam | 0.50 | otp request |
| `ho-a19-scam` Fastag Deduction / Vehicle Toll Fraud | 0.50 | otp request |
| `ho-bS18-scam` Social media account recovery | 0.50 | otp request |
| `ho-a04-scam` Part-Time Telegram Task Scam | 0.45 | money transfer |
| `ho-a14-scam` Loan Approval Without CIBIL Check | 0.45 | money transfer |
| `ho-bS03-scam` Fake lottery winner | 0.45 | money transfer |
| `ho-bS12-scam` Fake government subsidy | 0.45 | money transfer |
| `ho-c01-scam` electricity disconnection | 0.45 | money transfer |
| `ho-c16-scam` instant loan | 0.45 | money transfer |
| `ho-c22-scam` olx qr scan | 0.45 | money transfer |
| `ho-c13-scam` family emergency voice clone | 0.40 | authority impersonation, urgency |
| `ho-a16-scam` Rental Property Deposit / Defense Officer Scam | 0.25 | authority impersonation |
| `ho-a17-scam` E-Commerce Refund / Unauthorized Transaction | 0.25 | authority impersonation |
| `ho-bS11-scam` Fake investment scheme | 0.25 | too good to be true |
| `ho-bS20-scam` Work-from-home task scam | 0.25 | too good to be true |
| `ho-c03-scam` lottery win | 0.25 | too good to be true |
| `ho-c10-scam` task job offer | 0.25 | too good to be true |
| `ho-c14-scam` income tax refund | 0.25 | authority impersonation |
| `ho-c19-scam` romance gift | 0.25 | authority impersonation |
| `ho-c24-scam` ecommerce refund | 0.25 | authority impersonation |
| `ho-a01-scam` Fake Power Grid Officer | 0.15 | urgency |
| `ho-a11-scam` Wrong Credit / Accidental Money Transfer | 0.15 | urgency |
| `ho-a18-scam` Medical Emergency Fundraiser / Fake NGO | 0.15 | urgency |
| `ho-a21-scam` Fake Gas Pipeline Connection Bill | 0.15 | urgency |
| `ho-a06-scam` Stock Market Insider Tips / Advisory Group | 0.00 | nothing |
| `ho-a08-scam` Fake Job Offer / Overseas Work Visa | 0.00 | nothing |
| `ho-a12-scam` Insurance Policy Bonus / Maturity Release | 0.00 | nothing |
| `ho-a20-scam` Crypto Investment Double-Your-Money | 0.00 | nothing |
| `ho-bS06-scam` Fake UPI refund | 0.00 | nothing |
| `ho-bS07-scam` Fake job offer | 0.00 | nothing |
| `ho-bS09-scam` Fake insurance maturity | 0.00 | nothing |
| `ho-bS13-scam` Fake relative emergency | 0.00 | nothing |
| `ho-bS14-scam` Fake Google Pay support | 0.00 | nothing |
| `ho-bS15-scam` Fake electricity subsidy | 0.00 | nothing |
| `ho-bS17-scam` Fake loan approval | 0.00 | nothing |
| `ho-bS19-scam` Fake gas connection KYC | 0.00 | nothing |
| `ho-bS23-scam` Fake online shopping refund | 0.00 | nothing |
| `ho-bS24-scam` Fake scholarship | 0.00 | nothing |
| `ho-bS26-scam` Fake trading account manager | 0.00 | nothing |
| `ho-bS28-scam` Fake electricity meter replacement | 0.00 | nothing |
| `ho-bS29-scam` Fake charity / disaster relief | 0.00 | nothing |
| `ho-c08-scam` wrong upi refund | 0.00 | nothing |
| `ho-c11-scam` insurance bonus | 0.00 | nothing |
| `ho-c12-scam` gas subsidy | 0.00 | nothing |
| `ho-c18-scam` investment tips | 0.00 | nothing |
| `ho-c21-scam` fastag kyc | 0.00 | nothing |
| `ho-c25-scam` ceo gift cards | 0.00 | nothing |
| `ho-c26-scam` mobile tower rent | 0.00 | nothing |

_Generated from `holdout-rules.json`. Re-run with `BENCHMARK_DIR=../benchmark/holdout go run ./cmd/benchmark` (add the EVON_* settings for the Evon comparison)._

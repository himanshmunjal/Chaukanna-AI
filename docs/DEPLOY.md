# Deploying Chaukanna for reviewers and judges

This sets up a public link that a panel can open and use straight away. Each visitor gets their own isolated demo, and the in-app **Guide** opens on their first visit and walks them through every feature.

## What judges will be able to do

| Without anything from you | With the family password (optional) |
|---|---|
| The guided tour, opened automatically on the first visit | Everything on the left, plus: |
| **Simulator**: scam and normal calls through the real detector, with spoken warnings | **People**: add a person, consent, Telegram alerts, family voice |
| **Check a call** with pasted text, or an uploaded recording\* | Scam numbers: report and unblock |
| **Scam Drill**, typed or spoken\* | Calls that aren't deleted after an hour |
| **Speakerphone** with their microphone\* | |

\* Uses your Gnani key when `DEMO_SHARED_KEY=true` (up to `DEMO_SHARED_DAILY_LIMIT` listening sessions a day across everyone). Otherwise visitors enter their own Gnani key.

What a demo visitor **can't** do: see anyone else's calls, reach real people or numbers, or send a real alert. Their calls are deleted after an hour.

## Option A: Render (recommended, uses `render.yaml`)

1. Push the repository to GitHub.
2. In [Render](https://render.com), choose **New → Blueprint**, and pick the repository. Render reads `render.yaml`.
3. When asked, fill in:
   - `GNANI_API_KEY`: your Gnani key.
   - `DASHBOARD_URL`: leave empty for now.
   - The optional extras (`TELEGRAM_BOT_TOKEN`, `EVON_*`): leave empty unless you use them.
4. Press **Apply**. The first build takes about five minutes.
5. When it's live, copy the address (for example `https://chaukanna.onrender.com`). In the service's **Environment** tab, set `DASHBOARD_URL` to it, and save. Render redeploys.
6. Open the address in a private browser window. You should see the language picker, then the **Guide**.

The blueprint uses the **starter** plan, which stays awake. On the **free** plan the service sleeps after 15 idle minutes, and the next visitor waits about a minute for it to start, which looks broken to a judge. Free also means the database is reset on every restart. That's fine for a demo, but switch to starter for the judging period.

## Option B: any host that runs Docker (Railway, Fly.io, a VPS)

Build from the `Dockerfile` and set these environment variables:

| Variable | Value | Why |
|---|---|---|
| `DEMO_MODE` | `true` | Isolated demo for every visitor |
| `GNANI_API_KEY` | your key | Spoken warnings, and listening if shared |
| `DEMO_SHARED_KEY` | `true` | Judges don't need their own key |
| `DEMO_SHARED_DAILY_LIMIT` | `200` | Caps how many listening sessions your key pays for per day |
| `DASHBOARD_PASSWORD` | a long random value | Family login. Also required for anything other than localhost |
| `ENCRYPTION_KEY` | a long random value | Encrypts stored transcripts |
| `DASHBOARD_URL` | the public `https://` address | Links in alerts |
| `TRUSTED_PROXIES` | `10.0.0.0/8,172.16.0.0/12,192.168.0.0/16` | Rate limits see each visitor, not the host's proxy |
| `PORT` | `8080` (or what the host expects) | The port the server listens on |

The host must serve **https**: browsers allow the microphone only on https. Render, Railway and Fly provide it. Add a volume at `/data` to keep the database across restarts.

Generate random values with `openssl rand -hex 24`.

## Check it before sending the link

Open the link in a **private window** (so it's a true first visit), and check:

- [ ] The language picker appears, then the **Guide** opens by itself.
- [ ] The top of the page says it's a public demo.
- [ ] **Simulator → Fake CBI officer, 'digital arrest'** shows red flags and the warning. Press **Sound on** to hear it.
- [ ] **Genuine loan EMI reminder** stays quiet.
- [ ] **Speakerphone → Start listening** works on a phone (needs https and `DEMO_SHARED_KEY=true`, or your own key).
- [ ] `https://<your-address>/health` shows `"ok": true`, with `prisma` and `timbre` set to `true`.
- [ ] **Family login** with `DASHBOARD_PASSWORD` opens **People**.

## What to send the panel

> **Chaukanna: live demo**
> Link: `https://<your-address>`
> Open it on a laptop or phone. A short guide opens on your first visit and walks you through every feature in about 10 minutes. No key or sign-up is needed.
> To try a call with a real voice, open **Speakerphone**, press **Start listening**, and read out the scam line shown in the guide.
> Optional family view (People, Telegram alerts, family voice): press **Family login** and use the password `<DASHBOARD_PASSWORD>`.
> Source code and full manual: https://github.com/himanshmunjal/Chaukanna-AI

Only share the family password if you're happy for judges to see everything stored on that deployment. Don't use the judges' deployment for your own family's real calls.

## Costs and limits

- Gnani credits are spent on: warning clips prepared at each start (Hindi and English), spoken warnings and drill lines, and with `DEMO_SHARED_KEY=true`, each microphone session, uploaded recording and spoken drill answer, up to the daily limit.
- When the daily limit is reached, visitors see a message asking for their own key, and everything typed keeps working.
- Per-visitor limits: checks, drills, simulations and microphone sessions per minute, at most 2 demo calls at once per visitor and 20 overall, and limits on login attempts.

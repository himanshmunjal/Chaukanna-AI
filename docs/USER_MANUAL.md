# Chaukanna User Manual

**Welcome!** This manual is for anyone opening Chaukanna for the first time: a **reviewer or judge** trying the project, a **developer** running it, or a **family** using it to protect a parent.

It takes you from a fresh download to a working scam warning, step by step. Each step says what to do and what you should see.

> **What is Chaukanna?** An AI guardian that listens to a phone call with an elderly person, recognises a scam while it is happening, and **speaks a warning into the call** in their own language. It also alerts the family, who can watch the call live and step in. It is built on Gnani AI: **Prisma** (speech-to-text) and **Timbre** (text-to-speech).

> **Using a hosted demo link?** Just open it. A **Guide** appears on your first visit with the 10-minute tour, and stays in the menu afterwards. Parts 2 and 4 below have the same steps in more detail.

---

## Choose your path

| You are… | Time | Go to |
|---|---|---|
| A **reviewer** who wants to see it working, with no API keys | 10 minutes | [Part 1](#part-1-install-and-run) → [Part 2](#part-2-guided-tour-for-reviewers) |
| Someone who wants to **hear it listen and speak** on a real call | +10 minutes | [Part 3](#part-3-add-a-gnani-key-real-listening-and-speaking) |
| A **family** setting it up for a parent | 20 minutes | [Part 4](#part-4-everyday-use-for-families) |
| Stuck on something | — | [Part 5: Troubleshooting](#part-5-troubleshooting) |

---

## Contents

- [Part 1: Install and run](#part-1-install-and-run)
- [Part 2: Guided tour for reviewers](#part-2-guided-tour-for-reviewers)
- [Part 3: Add a Gnani key (real listening and speaking)](#part-3-add-a-gnani-key-real-listening-and-speaking)
- [Part 4: Everyday use for families](#part-4-everyday-use-for-families)
- [Part 5: Troubleshooting](#part-5-troubleshooting)
- [Part 6: FAQ](#part-6-faq)
- [Part 7: Glossary](#part-7-glossary)
- [Part 8: Card for the protected person](#part-8-card-for-the-protected-person)

---

# Part 1: Install and run

You can run Chaukanna in two ways. **Option A** is best for reviewing and developing. **Option B** needs only Docker.

## 1.1 What you need

| Tool | Version | Check with | Install |
|---|---|---|---|
| Git | any | `git --version` | [git-scm.com](https://git-scm.com/downloads) |
| Go | 1.26 or newer | `go version` | [go.dev/dl](https://go.dev/dl/) · macOS: `brew install go` |
| Node.js | 20 or newer | `node --version` | [nodejs.org](https://nodejs.org/) · macOS: `brew install node` |
| A browser | Chrome, Edge, Firefox or Safari | — | — |

**No API keys are needed** for the guided tour. Every integration is optional and switches on when its key is set.

## 1.2 Get the code

```bash
git clone https://github.com/himanshmunjal/Chaukanna-AI.git
cd Chaukanna-AI
```

## 1.3 Option A: run with Go and Node (recommended)

You need **two terminal windows**, both starting in the `Chaukanna-AI` folder.

**Terminal 1: the backend**

```bash
cd backend
cp .env.example .env        # Windows: copy .env.example .env
go run ./cmd/server
```

The first run downloads Go packages, which takes a minute. When it is ready you will see:

```
🛡️  Chaukanna backend listening on :8080
```

Lines such as `gnani not configured (see backend/.env)` are normal: they just list the integrations you haven't set up yet.

**Terminal 2: the dashboard**

```bash
cd web
npm install
npm run dev
```

When it is ready you will see:

```
  ➜  Local:   http://localhost:5173/
```

**Open http://localhost:5173 in your browser.** Leave both terminals running.

> **Check it worked:** http://localhost:8080/health shows `"ok": true` and which integrations are switched on.

## 1.4 Option B: run with Docker

One container runs the backend and the dashboard together.

```bash
cp backend/.env.example backend/.env
```

Open `backend/.env` and set a password. Docker needs one, because the dashboard opens without a password only for requests from the same computer, and Docker's network doesn't count as that:

```
DASHBOARD_PASSWORD=choose-any-password
```

Then build and run it:

```bash
docker build -t chaukanna .
docker run -p 8080:8080 -v chaukanna-data:/data --env-file backend/.env chaukanna
```

Open **http://localhost:8080** and log in with the password you chose.

## 1.5 Your first screen

1. **Choose a language.** The first time, Chaukanna asks which language to show. Pick **English** for the tour. You can change it any time from the language menu at the top.
2. **The menu.** At the top (or at the bottom on a phone) you'll find:

| Menu item | What it is for |
|---|---|
| **Calls** | Home page: every protected call, with headline numbers |
| **Speakerphone** | Protect a call on speaker using this device's microphone |
| **Check a call** | Paste words or upload a recording for a second opinion |
| **Scam Drill** | A practice scam call to build the habit of hanging up |
| **Simulator** | Replay scripted scam and normal calls through the real detector |
| **People** | The people being protected, family alerts, and known scam numbers |

The dot at the top right shows **Live** when the dashboard is connected to the backend.

---

# Part 2: Guided tour for reviewers

This tour needs **no API keys**. It uses the **Simulator**, which replays a scripted call through the **same detection pipeline as a real call**. Only the audio is replaced by text.

> Without a Gnani key, warnings appear as **text** instead of being spoken. Everything else works the same. Add a key in [Part 3](#part-3-add-a-gnani-key-real-listening-and-speaking) to hear them.

### Step 0: Add a test person (1 minute)

Do this first, so the family alerts show up on the calls you are about to simulate.

1. Open **People**.
2. Under **Add a person**, enter a name (for example *Kamla Devi*), any phone number, **Language: Hindi**, a family member (for example *Rahul (son)*) and any **Family phone (gets alerts)** number. Made-up numbers are fine: no message is sent until Telegram, WhatsApp or SMS is set up.
3. Tick the consent box and save.

**What you should see:** the person appears under **Protected people**, with **No consent recorded** or **Consent recorded** and **Telegram not linked** next to them.

### Step 1: Watch a scam being caught (2 minutes)

1. Open **Simulator**.
2. Leave **Live protection (listen silently, warn if scam)** selected, and the speed at **1× real time**.
3. Find **Fake CBI officer, 'digital arrest' (Hinglish)** and press **▶ Run**.
4. The call's page opens. Watch it unfold over about 40 seconds.

**What you should see**

- **Live transcript:** the "CBI officer" lines appear one by one.
- **Red flags** build up as the script unfolds: *authority impersonation* ("CBI headquarters"), *fear threat* ("arrest warrant", "digital arrest"), *isolation* and *secrecy* ("video call band mat karna, kisi ko mat batana"), and finally *money transfer*.
- The **Scam risk** gauge climbs to about 95%, with a short reason underneath.
- A red banner: **Scam detected**, then **Warning spoken on the call**, with the exact Hindi warning that was played:
  > *"चौकन्ना AI की चेतावनी। यह कॉल धोखाधड़ी लग रही है। डिजिटल अरेस्ट नाम की कोई चीज़ नहीं होती…"*
- **Alerts sent**: the alert for the family member you added in Step 0. It shows as *logged*, because no Telegram, WhatsApp or SMS is set up yet. With Telegram set up, it arrives on the family's phone.

### Step 2: See it stay quiet on a normal call (1 minute)

1. Go back to **Simulator**.
2. Run **Genuine loan EMI reminder (English)**.

**What you should see:** the transcript and a risk score near 0%, **no warning and no alert**, even though the caller says "OTP" ("never share your OTP or PIN"). A false alarm on a genuine bank call costs trust, so Chaukanna is designed to stay quiet here.

Try the other scenarios too. Names starting with "Genuine" or describing ordinary calls (delivery, clinic, family) should stay quiet. The rest are scams in Hindi, Hinglish, English and Tamil-English.

### Step 3: Write your own scam (2 minutes)

1. On **Simulator**, find the custom script box.
2. Type one sentence per line, for example:
   ```
   Main customs office se bol raha hoon
   Aapke parcel mein drugs mile hain
   AnyDesk download kijiye
   ```
3. Press **▶ Run custom script**.

**What you should see:** "AnyDesk download kijiye" is a **direct demand** to install a screen-sharing app. Chaukanna treats that as a scam on its own and warns straight away.

Now try a version **without** a demand (for example, only the first line). It should not warn, or should give only a soft caution.

### Step 4: Step in as the family (2 minutes)

1. Run any scam scenario at **1× real time**.
2. While it is running, on the call's page, find **Step in**:
   - Press **Warn now**: the full warning is played on the call immediately.
   - Type a short message under **Message to speak on the call** and press **Speak**: Chaukanna speaks your words into the call.
   - Press **This call is safe**: warnings stop for this call. Press **Start warning again** to undo.
3. When the call ends, find **What to do now**:
   - Press **It was a scam**, **Money was lost** or **False alarm**, and watch the advice change.
   - Press **Copy complaint draft**: a draft for cybercrime.gov.in, filled in from the call, ready to check and file. Chaukanna never files anything itself.

### Step 5: Let Chaukanna answer an unknown caller (1 minute)

1. On **Simulator**, change the mode to **Call screening (Chaukanna answers an unknown caller)**.
2. Run a scam scenario, then a normal one.

**What you should see:** Chaukanna's own lines appear in the transcript as it greets the caller and asks who they are and why they're calling. After the call, a **Screening result** gives the purpose and a scam score for the family.

### Step 6: Check a call (1 minute)

1. Open **Check a call**.
2. Paste into **What did the caller say?**:
   ```
   Sir aapka KYC expire ho gaya hai. Account band ho jayega.
   Abhi aapke phone par ek OTP aaya hoga, woh bata dijiye.
   ```
3. Press **Check**.

**What you should see:** a **Scam risk** of about 90%, and **Red flags** for *fear threat* ("account band ho jayega") and *OTP request*, each with the words that triggered it. Nothing is saved.

### Step 7: Try the Scam Drill (1 minute)

1. Open **Scam Drill** and pick an English practice call (the bank KYC one is a good start).
2. Under **or type your answer**, type "Yes, the OTP is 4 5 6 7" and press **Send**.
3. Start again and this time answer "I will hang up and call my bank", or press **Hang up**.

**What you should see:** each answer gets a verdict and a short lesson. Giving the code is marked **Risky: this is what the scammer wants**. The careful answer is marked safe, and pressing **Hang up** gets *"You hung up. Perfect."*

### Step 8: Look at the setup screens (1 minute)

1. On **People**, press **Set up** next to the person from Step 0. You'll see consent, **Telegram alerts (free)** and **Warn in a family member's voice**.
2. Below the list, see the known scam numbers. You can **Report** a number or **Unblock** one.
3. Switch the language from the top menu to **हिन्दी** or another language: the call screens change language.

### Step 9 (optional): Run the automated checks

In a third terminal:

```bash
cd backend
go test ./...                                  # unit tests
EVON_BASE_URL= EVON_MODEL= go run ./cmd/benchmark   # detection benchmark, rules only
```

The benchmark replays every scenario in `benchmark/cases/` and reports scams caught, false alarms and time to detect. The headline numbers and their caveats are in [`benchmark/results.md`](../benchmark/results.md).

**That's the full tour.** To hear Chaukanna listen to a real voice and speak its warning, continue to Part 3.

---

# Part 3: Add a Gnani key (real listening and speaking)

## 3.1 Add the key

1. Get an API key from your Gnani AI developer account.
2. Open `backend/.env` and set:
   ```
   GNANI_API_KEY=your-key-here
   ```
   The Prisma and Timbre addresses are filled in automatically, so leave `PRISMA_WS_URL` and `TIMBRE_URL` empty.
3. Stop the backend (**Ctrl + C** in Terminal 1) and start it again with `go run ./cmd/server`.
4. Check the key works end to end. This uses about 30 seconds of credits:
   ```bash
   cd backend
   go run ./cmd/gnanicheck -langs hi
   ```
   It speaks a Hindi sentence with Timbre, squeezes it to phone quality, transcribes it with Prisma, and shows each result.

At startup the backend now prepares the warning clips, so a warning plays instantly on a call.

## 3.2 Re-run the Simulator, with sound

Run **Fake CBI officer, 'digital arrest'** again, and press **Sound on** on the call's page. You now **hear** the warning in Timbre's voice.

## 3.3 Test with a real voice: Speakerphone mode

You need **two devices**: a phone for the call, and a laptop running the dashboard.

1. On the laptop, open **Speakerphone**.
2. Choose **Live protection: listen silently, warn if scam**, and the language (**Hindi** for the script below).
3. Press **Start listening** and allow the microphone.
4. Ask a friend to call your phone and play the scammer. Put the call **on speaker** next to the laptop. They can read:
   > *"Main CBI headquarters se bol raha hoon. Aapke naam par arrest warrant hai. Aap digital arrest mein hain. Kisi ko mat batana. Abhi apna saara paisa is safe account mein transfer kijiye."*
5. Answer as you normally would.

**What you should see and hear:** the transcript appears live on the laptop. Within a few seconds of the demand, the laptop **speaks the warning out loud**, and a full-screen red **Scam warning** appears. Your friend hears the warning through your phone too.

More scripts in many languages are in [`test-call-scripts.csv`](test-call-scripts.csv).

> **Using a phone or tablet as the listening device?** Browsers allow the microphone only on `https` or `localhost`. Run `ngrok http 5173`, set `DASHBOARD_PASSWORD` in `backend/.env`, restart the backend, then open the ngrok `https` address on that device and log in.

## 3.4 Optional extras

| What | How |
|---|---|
| **Telegram alerts** | Create a bot with @BotFather in Telegram, put its token in `TELEGRAM_BOT_TOKEN`, restart, then follow [4.3](#43-turn-on-telegram-alerts) |
| **Reasoning model** | Set `EVON_BASE_URL`, `EVON_MODEL` and `EVON_API_KEY` to any OpenAI-compatible endpoint. Rules keep working if it is slow or fails. See [`deploy/evon/README.md`](../deploy/evon/README.md) |
| **A real phone number** | Exotel or Twilio, so the person can use **Add call → Merge**. See *Connect a real phone number* in the [README](../README.md#5-connect-a-real-phone-number-optional) |

---

# Part 4: Everyday use for families

## 4.1 Who does what

| Who | What they do |
|---|---|
| **Family member** | Sets up Chaukanna, adds the person to protect, links Telegram alerts, and watches the dashboard when alerted |
| **Protected person** | Brings Chaukanna into a suspicious call, and follows its warning |

## 4.2 Add the person to protect

1. Open **People**.
2. Under **Add a person**, fill in their name, phone number and **language** (Chaukanna listens and warns in this language), the **family member** to alert, and the **family phone (gets alerts)**.
3. Tick the consent box: *"This person has agreed that Chaukanna may listen to calls they choose to protect, and alert their family."* Ask them first. It is their call.
4. Save. The person appears under **Protected people**.

## 4.3 Turn on Telegram alerts

1. On **People**, press **Set up** next to the person.
2. Under **Telegram alerts (free)**, copy the private link and send it to the family member.
3. They open it on their phone and tap **Start**. The dashboard then shows **Telegram linked**.

Each link works **once**, and alerts for a person go to **one** Telegram chat. If the link was opened on the wrong phone, press **Set up** again and send a fresh link: alerts move to whoever opens it.

## 4.4 Optional: warnings in a family member's voice

A warning in a daughter's or son's voice is harder to ignore than a stranger's.

1. In **Set up**, open **Warn in a family member's voice**.
2. Answer **Whose voice is this?**, and confirm it is your own voice and that you agree.
3. Record 5 to 30 seconds of yourself speaking naturally, and wait for **Creating the voice…** to finish.

The warning still says it comes from Chaukanna AI. Press **Delete voice** at any time to remove it. The recording itself is not kept.

## 4.5 Install it on the phone

On Android, tap **Install** on the **Install Chaukanna** card. On iPhone, tap **Share**, then **Add to Home Screen**. It then opens with one tap, like an app. Without internet, it still shows safety advice and a 1930 button.

## 4.6 Three ways to protect a call

| Mode | Best for | What the person does |
|---|---|---|
| **Speakerphone** | At home, with a second device nearby | Puts the call on speaker next to the device |
| **Guardian** | They answered and then became suspicious | Three taps: **Add call → Chaukanna → Merge** |
| **Screen** | Unknown callers they don't pick up | Nothing, once forwarding is set up |

**Speakerphone:** on the second device, open **Speakerphone**, choose the person and language, press **Start listening**, and put the call on speaker next to it. Press **Stop listening** when the call ends. The browser must be on a **different** device from the call, because the call takes over the phone's own microphone.

**Guardian (needs a Chaukanna phone number):** during a call that feels wrong, tap **Add call**, dial the Chaukanna number (save it as a favourite contact), and tap **Merge**. Chaukanna joins silently and speaks into the call only if it detects a scam.

> Three taps are hard to remember when frightened. **Practise it together** once or twice, with a family member playing the caller.

**Screen (needs a Chaukanna phone number):** have unanswered calls forwarded to the Chaukanna number. Chaukanna answers the unknown caller, asks who they are and why they're calling, then gives the family a summary and a scam score.

## 4.7 What the person will hear

| How sure Chaukanna is | What happens |
|---|---|
| Not suspicious | Nothing. It stays quiet, for example on an ordinary EMI reminder |
| Somewhat suspicious | A **soft caution**, telling them how to check for themselves. The family isn't alerted |
| Clearly a scam, or **any direct demand** (OTP, AnyDesk, scan this QR, transfer money) | The **full warning**, plus a family alert |

After the warning, Chaukanna keeps listening:

- If the person starts **reading out a code or agreeing to pay**, it interrupts with a short **"stop"**, and the family gets an urgent alert to phone them.
- If the caller **repeats the demand**, it says "stop" again.
- If the caller says **"this is not a scam"**, it answers back (up to twice per call), and the family is told.
- If the person is **still on the call 45 seconds after the warning**, the family is alerted again.

Warnings are always pre-written messages, never made up by the AI, so the advice never changes.

## 4.8 When an alert arrives

Open the link in the alert to watch the call live: the **Live transcript**, the **Red flags** with the exact words, and the **Scam risk** with its reason.

If you can, **phone the protected person yourself**. Scammers depend on them being alone. You can also use **Step in**:

| Button | What it does |
|---|---|
| **Warn now** | Plays the full warning on the call immediately |
| **Speak** | Type up to 200 characters; Chaukanna speaks them into the call. *The caller can hear these too* |
| **This call is safe** | Stops warnings on this call. **Start warning again** undoes it |

> Only press **This call is safe** after checking who is calling yourself. **Never** press it because a caller asked you to.

## 4.9 After a scam call

Open the call and find **What to do now**.

1. **Tell Chaukanna what happened:** **It was a scam**, **Money was lost**, or **False alarm**. With **False alarm**, the number is released if Chaukanna had blocked it.
2. **Follow the steps shown:** phone the person and stay with them; don't call the number back; uninstall any app such as AnyDesk and change banking passwords from a different phone; report the number.
3. **If money was lost, act immediately:**
   - **Call 1930.** The sooner it is reported, the better the chance of freezing the money.
   - Call the bank's official number (on the back of the card), and block the card, UPI and net banking.
   - Keep every SMS, screenshot and transaction ID.
4. **File a complaint:** press **Copy complaint draft**, check every detail, then submit it yourself at **cybercrime.gov.in**.

## 4.10 Scam numbers

A number caught scamming is remembered. If it calls **any** protected person again, Chaukanna warns at once, or blocks it in Screen mode. On **People** you can **Report** a known scam number, or **Unblock** one blocked by mistake.

## 4.11 Practise with Scam Drill

Once a month, sit with the protected person and do a **Scam Drill** together. They **Answer out loud** (or type), get feedback on each answer, and learn that **Hang up** is always the right answer. *You owe a stranger no explanation.*

---

# Part 5: Troubleshooting

### Installing and running

| Problem | What to do |
|---|---|
| `go: command not found` or `npm: command not found` | Install Go and Node.js ([1.1](#11-what-you-need)), then open a **new** terminal |
| Go says it needs a newer version, or tries to download one | Go 1.21 or newer downloads Go 1.26 by itself on the first run, so let it finish. With an older Go, install 1.26 from [go.dev/dl](https://go.dev/dl/) |
| `address already in use` on port 8080 or 5173 | An earlier copy is still running, or another program uses the port. Close it (**Ctrl + C** in its terminal) and start again. The dashboard expects the backend on 8080 |
| The dashboard shows errors or a blank list | Check the backend is still running in Terminal 1, and that http://localhost:8080/health opens |
| A **login page** appears | A password is needed whenever the dashboard is opened through Docker, ngrok or a hosted address. Set `DASHBOARD_PASSWORD` in `backend/.env`, restart the backend, and log in with it |
| *"login is not set up"* | No password is set. Open the dashboard on the same computer as the backend, or set `DASHBOARD_PASSWORD` |
| Warnings appear only as text | Normal without a Gnani key. Add one ([3.1](#31-add-the-key)) |

### During calls

| Problem | What to do |
|---|---|
| *"Microphone permission was denied"* | Allow the microphone from the icon in the browser's address bar, then press **Start listening** again |
| *"The microphone is in use by something else"* | The phone call is using that device's microphone. Use a **second** device for the browser |
| The microphone doesn't work on a phone or tablet | Browsers need `https`. Use ngrok ([3.3](#33-test-with-a-real-voice-speakerphone-mode)) or a hosted address |
| *"Add your Gnani API key to use the microphone"* | You're on a public demo. Enter your own key in **Your Gnani API key** and press **Use my key** |
| No warning on an obvious scam | Check the speaker volume and that the device is near the phone. Check the person's **language** on **People** matches the call |
| A genuine call got a warning | Press **This call is safe** during the call, or **False alarm** afterwards |
| Telegram alerts don't arrive | On **People**, check it says **Telegram linked**. If not, press **Set up** and send a fresh link |
| *"This link is not valid"* in Telegram | The link was already used. Make a new one with **Set up** |
| The top-right dot says **Connecting…** | The live connection dropped. Check the backend and the internet; it reconnects by itself |

---

# Part 6: FAQ

**Does Chaukanna record calls?**
No. No call audio is stored. Transcripts of flagged calls are kept for the family and deleted after 30 days by default, and they can be encrypted on the server. **Check a call** and **Scam Drill** save nothing.

**Does the scammer hear the warning?**
Yes, in Guardian and Speakerphone modes. That's intentional: the warning breaks the scammer's hold, and it often makes them hang up.

**Can the AI say something wrong on the call?**
The spoken words are always pre-written warnings. The AI only decides *whether* to warn, and explains why on the dashboard.

**What if the internet or the AI model is slow?**
Simple rules run on every sentence, and a direct demand for an OTP, an app or money triggers a warning on its own. A slow or failed reasoning model never delays a warning.

**Which languages are supported?**
Warnings and the call screens are in 10 Indian languages: Hindi, English, Bengali, Tamil, Telugu, Marathi, Gujarati, Kannada, Malayalam and Punjabi. Calls can mix English with Hindi or a regional language.

**Is it free?**
The code is free to run. Prisma and Timbre use Gnani API credits, Telegram alerts are free, and a phone number from Exotel or Twilio has its own charges.

**Do I need an app?**
No. Speakerphone mode works in a browser, and Guardian mode uses the phone's own **Add call** and **Merge** buttons. The dashboard can also be installed to the home screen.

---

# Part 7: Glossary

| Term | Meaning |
|---|---|
| **Prisma** | Gnani's speech-to-text: turns the call's audio into words, live |
| **Timbre** | Gnani's text-to-speech: speaks the warning in a natural voice |
| **Evon** | Gnani's reasoning model. Optional; adds judgement on top of the rules |
| **Red flag** | One warning sign in the call, for example *authority impersonation* or *OTP request* |
| **Direct demand** | A request for an OTP, a screen-sharing app, a QR scan or a money transfer. Enough on its own to warn |
| **Soft caution** | A gentle spoken reminder on a doubtful call, without alerting the family |
| **Guardian mode** | Chaukanna joins an ongoing call through **Add call → Merge** |
| **Screen mode** | Chaukanna answers unknown callers the person didn't pick up |
| **Speakerphone mode** | A second device listens to the call on speaker |
| **Simulator** | Replays scripted calls through the real detector, with no phone needed |
| **1930** | India's national cyber-fraud helpline |

---

# Part 8: Card for the protected person

Print this and keep it near the phone.

```
┌──────────────────────────────────────────────────────┐
│                 🛡️  CHAUKANNA  🛡️                     │
│                                                      │
│  A call feels wrong?                                 │
│    1. Tap  ADD CALL                                  │
│    2. Call CHAUKANNA (saved in favourites)           │
│    3. Tap  MERGE                                     │
│                                                      │
│  At home? Put the call on SPEAKER next to the        │
│  Chaukanna device.                                   │
│                                                      │
│  NEVER share an OTP. NEVER install an app a caller   │
│  asks for. Police NEVER arrest on a video call.      │
│                                                      │
│  Hear a warning? HANG UP. Then call your family.     │
│                                                      │
│  Money gone?  Call 1930 at once.                     │
└──────────────────────────────────────────────────────┘
```

कोई OTP मत बताइए। कोई ऐप इंस्टॉल मत कीजिए। शक हो तो फ़ोन काटिए और **1930** पर कॉल कीजिए।

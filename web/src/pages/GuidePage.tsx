// The in-app user manual. It opens by itself on a first visit, so a reviewer or a
// family member knows where to start, and stays in the menu as "Guide".
// The full manual, including running it yourself, is docs/USER_MANUAL.md.

import { useEffect, type ReactNode } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useAccess } from '../lib/access'
import { Button, Card } from '../components/ui'
import { markGuideSeen } from '../lib/guide'

const REPO = 'https://github.com/himanshmunjal/Chaukanna-AI'
const FULL_MANUAL = `${REPO}/blob/main/docs/USER_MANUAL.md`

interface Step {
  title: string
  to: string
  open: string
  do: ReactNode
  see: ReactNode
}

export function GuidePage() {
  const navigate = useNavigate()
  const { access } = useAccess()
  const visitor = access.demo && !access.family
  const canListen = !visitor || !!access.shared_key

  useEffect(markGuideSeen, [])

  const start = () => navigate('/')
  const scrollToTour = () => document.getElementById('tour')?.scrollIntoView({ behavior: 'smooth', block: 'start' })

  const steps: Step[] = [
    ...(access.family
      ? [{
          title: 'Add a test person',
          to: '/users',
          open: 'Open People',
          do: <>Under <b>Add a person</b>, enter a name, any phone number, <b>Language: Hindi</b>, a family member and any family phone. Tick the consent box and save.</>,
          see: <>The person appears under <b>Protected people</b>. Simulated scam calls now produce a family alert for them.</>,
        }]
      : []),
    {
      title: 'Watch a scam being caught',
      to: '/simulator',
      open: 'Open Simulator',
      do: <>Keep <b>Live protection</b> and <b>1× real time</b>. Press <b>▶ Run</b> on <b>Fake CBI officer, 'digital arrest' (Hinglish)</b>.</>,
      see: <>Over about 40 seconds the transcript appears line by line, red flags build up (authority, fear, isolation, secrecy, money transfer), the risk climbs to about 95%, and Chaukanna plays its Hindi warning. {visitor ? 'The call page shows the family alert that would have been sent.' : 'The family alert appears under Alerts sent.'}</>,
    },
    {
      title: 'See it stay quiet on a normal call',
      to: '/simulator',
      open: 'Open Simulator',
      do: <>Run <b>Genuine loan EMI reminder (English)</b>.</>,
      see: <>Risk stays near 0% and there is <b>no warning</b>, even though the caller mentions "OTP". A false alarm on a genuine bank call costs trust.</>,
    },
    {
      title: 'Write your own scam',
      to: '/simulator',
      open: 'Open Simulator',
      do: <>In the custom script box, type one sentence per line, e.g. <i>Main customs office se bol raha hoon</i> / <i>Aapke parcel mein drugs mile hain</i> / <i>AnyDesk download kijiye</i>. Press <b>▶ Run custom script</b>.</>,
      see: <>"AnyDesk download kijiye" is a <b>direct demand</b>, so it warns straight away. Try only the first line: it stays quiet.</>,
    },
    {
      title: 'Step in as the family',
      to: '/simulator',
      open: 'Open Simulator',
      do: <>While a scam call runs, use <b>Warn now</b>, type a message and press <b>Speak</b>, or press <b>This call is safe</b>. When it ends, open <b>What to do now</b>.</>,
      see: <>Your words are spoken into the call. After the call: the steps to take, the 1930 helpline, and a <b>complaint draft</b> to copy. Chaukanna files nothing by itself.</>,
    },
    {
      title: 'Let Chaukanna answer an unknown caller',
      to: '/simulator',
      open: 'Open Simulator',
      do: <>Switch the mode to <b>Call screening</b> and run a scam, then a normal call.</>,
      see: <>Chaukanna greets the caller, asks who they are and why they're calling, then gives a <b>Screening result</b> with a scam score.</>,
    },
    {
      title: 'Check a call',
      to: '/check',
      open: 'Open Check a call',
      do: <>Paste: <i>Sir aapka KYC expire ho gaya hai. Account band ho jayega. Abhi aapke phone par ek OTP aaya hoga, woh bata dijiye.</i> and press <b>Check</b>.</>,
      see: <>A scam risk of about 90%, with red flags for <b>fear threat</b> and <b>OTP request</b> and the words that triggered them. Nothing is saved.</>,
    },
    {
      title: 'Practise with Scam Drill',
      to: '/drill',
      open: 'Open Scam Drill',
      do: <>Pick a practice call. Type "Yes, the OTP is 4 5 6 7", then try again with "I will hang up and call my bank", or press <b>Hang up</b>.</>,
      see: <>Each answer gets a verdict and a short lesson. Giving the code is marked <b>Risky</b>; hanging up gets <i>"You hung up. Perfect."</i></>,
    },
    {
      title: 'Try it with a real voice',
      to: '/speakerphone',
      open: 'Open Speakerphone',
      do: canListen
        ? <>Choose <b>Hindi</b>, press <b>Start listening</b> and allow the microphone. Read out loud: <i>"Main CBI headquarters se bol raha hoon. Aapke naam par arrest warrant hai. Abhi apna saara paisa is safe account mein transfer kijiye."</i></>
        : <>Add your own Gnani API key on that page, then choose <b>Hindi</b>, press <b>Start listening</b> and read a scam line out loud.</>,
      see: <>Your words appear live, and within a few seconds of the demand the warning is <b>spoken out loud</b> with a full-screen red alert. On a real call, put the phone on speaker next to this device.</>,
    },
  ]

  return (
    <div className="space-y-8">
      <section className="rounded-3xl border border-slate-800 bg-slate-900 p-6 sm:p-8">
        <p className="text-sm font-medium text-cyan-400">User guide</p>
        <h1 className="mt-1 text-3xl text-slate-100 sm:text-4xl">Welcome to Chaukanna</h1>
        <p className="mt-3 max-w-3xl text-slate-300">
          An AI guardian that listens to a phone call with an elderly person, recognises a scam while it is happening, and{' '}
          <b>speaks a warning into the call</b> in their own language. The family is alerted, can watch the call live, and can step in.
          Built on Gnani AI: <b>Prisma</b> hears the call and <b>Timbre</b> speaks the warning.
        </p>
        {visitor && (
          <p className="mt-3 max-w-3xl text-sm text-slate-400">
            This is a public demo. Your calls are visible only to you and deleted after an hour, and no real alerts are sent.
            {access.shared_key ? ' The microphone and recordings work without a key, within a daily limit.' : ' The tour works without any key; the microphone and recordings need your own Gnani key.'}
          </p>
        )}
        <div className="mt-5 flex flex-wrap gap-3">
          <Button onClick={scrollToTour}>Take the 10-minute tour ↓</Button>
          <Button variant="ghost" onClick={start}>Skip, go to the dashboard</Button>
        </div>
      </section>

      <section id="tour" className="scroll-mt-24 space-y-4">
        <div>
          <h2 className="text-2xl text-slate-100">Try it in 10 minutes</h2>
          <p className="mt-1 text-sm text-slate-400">
            The <b>Simulator</b> replays a scripted call through the same detection as a real call; only the audio is replaced by text.
            Each step says what to do, and what you should see.
          </p>
        </div>
        <ol className="space-y-3">
          {steps.map((step, i) => (
            <li key={step.title} className="rounded-2xl border border-slate-800 bg-slate-900 p-5">
              <div className="flex flex-wrap items-start gap-3">
                <span aria-hidden className="grid h-8 w-8 shrink-0 place-items-center rounded-full bg-cyan-500/15 text-sm font-semibold text-cyan-300">{i + 1}</span>
                <div className="min-w-0 flex-1">
                  <h3 className="text-lg font-medium text-slate-100">{step.title}</h3>
                  <p className="mt-2 text-sm text-slate-300"><span className="font-medium text-slate-100">Do: </span>{step.do}</p>
                  <p className="mt-1.5 text-sm text-slate-300"><span className="font-medium text-emerald-300">You should see: </span>{step.see}</p>
                </div>
                <Link to={step.to} className="rounded-full border border-slate-700 px-3.5 py-1.5 text-sm text-cyan-300 hover:bg-slate-800">{step.open} →</Link>
              </div>
            </li>
          ))}
        </ol>
      </section>

      <div className="grid gap-5 md:grid-cols-2">
        <Card title="Three ways to protect a call">
          <dl className="space-y-3 text-sm">
            <Item term="Speakerphone">Put the call on speaker next to a laptop or second phone running this page. No phone number needed.</Item>
            <Item term="Guardian">During a suspicious call, tap <b>Add call</b>, dial the Chaukanna number, tap <b>Merge</b>. Chaukanna joins silently and speaks only if it's a scam.</Item>
            <Item term="Screen">Unanswered unknown calls are forwarded to Chaukanna, which asks who's calling and why, and tells the family.</Item>
          </dl>
        </Card>

        <Card title="What the person hears">
          <dl className="space-y-3 text-sm">
            <Item term="Normal call">Nothing. Chaukanna stays quiet.</Item>
            <Item term="Doubtful call">A soft caution on how to check for themselves. The family isn't alerted.</Item>
            <Item term="Scam, or any direct demand">The full warning (OTP, AnyDesk, QR code or money transfer is enough on its own), and the family is alerted.</Item>
            <Item term="After the warning">If they start reading out a code or agree to pay, a short "stop". If the caller says "this is not a scam", a reply.</Item>
          </dl>
        </Card>

        <Card title="Pages in this app">
          <dl className="space-y-3 text-sm">
            <Item term={<Link className="text-cyan-300" to="/">Calls</Link>}>Every protected call, with the transcript, red flags and risk.</Item>
            <Item term={<Link className="text-cyan-300" to="/speakerphone">Speakerphone</Link>}>Protect a call on speaker with this device's microphone.</Item>
            <Item term={<Link className="text-cyan-300" to="/check">Check a call</Link>}>Paste words or upload a recording for a second opinion.</Item>
            <Item term={<Link className="text-cyan-300" to="/drill">Scam Drill</Link>}>A practice scam call, to build the habit of hanging up.</Item>
            <Item term={<Link className="text-cyan-300" to="/simulator">Simulator</Link>}>Replay scripted scam and normal calls through the real detector.</Item>
            {access.family && <Item term={<Link className="text-cyan-300" to="/users">People</Link>}>Who is protected, family alerts on Telegram, family voice, scam numbers.</Item>}
          </dl>
        </Card>

        <Card title="Good to know">
          <dl className="space-y-3 text-sm">
            <Item term="Is call audio recorded?">No. Transcripts of flagged calls are kept for the family and deleted after 30 days by default.</Item>
            <Item term="Can the AI say something wrong?">No. The spoken words are always pre-written warnings; the AI only decides whether to warn.</Item>
            <Item term="Does the scammer hear it?">Yes, on purpose. It breaks the scammer's hold, and often makes them hang up.</Item>
            <Item term="Languages">Warnings and call screens in 10 Indian languages. Change it from the language menu at the top.</Item>
          </dl>
        </Card>
      </div>

      <section className="rounded-2xl border border-red-500/30 bg-red-500/5 p-5 text-sm text-slate-300">
        <b className="text-red-300">The one rule:</b> no real bank, police officer or government office will ever ask for an OTP, ask you to install an app,
        or arrest you over a video call. If in doubt, hang up and call <b>1930</b>, India's cyber-fraud helpline.
      </section>

      <div className="flex flex-wrap items-center gap-3">
        <Button onClick={start}>Start using Chaukanna</Button>
        <a href={FULL_MANUAL} target="_blank" rel="noreferrer" className="text-sm text-cyan-300 underline underline-offset-4">Full user manual (setup, families, troubleshooting) ↗</a>
        <a href={REPO} target="_blank" rel="noreferrer" className="text-sm text-cyan-300 underline underline-offset-4">Source code ↗</a>
      </div>
    </div>
  )
}

function Item({ term, children }: { term: ReactNode; children: ReactNode }) {
  return (
    <div>
      <dt className="font-medium text-slate-100">{term}</dt>
      <dd className="mt-0.5 text-slate-400">{children}</dd>
    </div>
  )
}

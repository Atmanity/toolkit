# What people build with Atmee

Match the idea to the closest row and say which in one line. Rows marked
**proven** were built as sample apps on the Avatar API and run in live
sessions on staging; the `build-avatar-app` skill carries the recipe for each
app shape and the starter they were built from.

## Avatars on atmee.ai (no code)

| Use case | What it is |
|---|---|
| Digital twin | a creator's own avatar that talks with their audience; verified creators earn from the talk time |
| Expert or mentor | a coach, a nutritionist, a founder, answering from the knowledge they give it |
| Character | a historical figure, a storybook or game character, an original creation |
| Companion | someone to talk to, with its own personality |
| Concierge | a brand's front desk, shared as a link or embedded on the brand's site |

Made in Studio or with the `create-avatar` skill; shared by link, on the
creator's profile, or listed on Explore when public.

## Apps in a developer's product (Avatar API)

The five app shapes, each with the sample app that proved it:

| Shape | For | Proven sample | What it uses |
|---|---|---|---|
| Guided conversation | screening interviews, onboarding, sales discovery | **Northwind Talent**: a recruiter's screening call from greeting to wrap-up | an agenda (conductor), `state` with the job and the candidate, voice or typing, tools that record answers |
| Lesson and quiz | staff training, a course, a product walkthrough | **Ledger Academy**: a training module with slides and a graded quiz | an agenda, `state` with the slide and score, tools that turn slides and grade answers |
| Companion that watches | a coding buddy, a study partner, a shopping assistant | **Kin**: watches the user's screen and speaks up when something is worth a word | the app's own vision model, its descriptions sent as `state`, what is noteworthy as `event` |
| Narrator driven by events | a game, live commentary, a guided story | **Isle of Whispers**: an interactive-fiction narrator | the user muted, an `event` per move, tools that reveal clues and give items |
| Broadcast host | a live stream, a webinar | **Nightcap Live**: a host who reads the audience's chat | the host muted, chat questions as typed turns, the rest batched into events, a restream for the audience |

One avatar can serve several apps by setting a different agenda before each
session; the agenda belongs to the avatar, so two apps sharing one avatar at
the same time fight over it.

## Mini-apps: the avatar hosts an app

The avatar runs an app with the user and acts on it through tools. Fourteen
were built on the Avatar API and run on staging:

- **Games:** guess the city from a postcard, a chess opponent who explains
  her moves, heads-up poker against an avatar whose cards the server hides
  even from him.
- **Tutors:** CPR and first-aid drills, character stroke order, a maths
  whiteboard read line by line, projectile-motion practice, a night-sky
  guide.
- **Advisors and coaches:** outfit advice from a photo, nutrition from a food
  photo or label, workout plans built to the user's limits.
- **Planners and data:** a stock watchlist by voice, a floor-plan editor, trip
  planning on a live map.

What they taught, for any app the avatar runs:

- **The app decides, the avatar performs.** Rules, scores, outcomes and
  hidden information live in the app; a tool gives the avatar the result
  before it answers. Without one the avatar replies before the app can act,
  and may invent an outcome it then has to take back.
- **Secrets stay on the server.** The poker hand, the quiz answer: the page
  and the avatar see a masked view.
- **Safety is the app's job.** The app detects the dangerous moment (an
  emergency in a first-aid drill) and shows exact safety text on screen; the
  avatar's words are a second path, not a guarantee of wording.
- **Predictable answers go in `state` ahead of time,** so the avatar has them
  the moment it is asked.
- **Health, money and law inform, never advise** (the Terms restrict
  medical, legal and financial advice): the stock desk refuses advice, the
  fitness coach stops for unsafe conditions.

## A face for the developer's own agent (render path)

The developer already runs a voice agent on LiveKit Agents and wants it to
have a face: a narrator, a presenter, an agent that answers from their own
systems. Atmee renders the avatar only; speech recognition, the model, the
voice, tools and logs stay in the developer's infrastructure. A portrait-only
avatar is enough, and adding a voice later turns it into a full avatar.
Guide: `https://www.atmee.ai/docs/livekit-plugin/livekit-agents-plugin`.

## Limits that change an idea

Beyond the limits SKILL.md lists, these decide how an app is designed:

- Tool calls reach the page in the room, never the developer's server
  directly: the page forwards what the server must know.
- A client that drops and reconnects ends the session; the app starts a new
  one.
- `mute` stops the user's words reaching the app too, so push-to-talk can't
  be built on it.
- The avatar's first words take seconds to arrive after the start; the page
  says it is connecting.

---
name: plan-avatar-app
description: Shape an idea for an Atmee avatar app into a one-page brief by asking in rounds - user, flow, persona, tools, screen, limits, live test. Use when the app is still an idea. NOT for coding it.
license: Proprietary
metadata:
  owner: Atmanity
allowed-tools: Bash(*/update-check*)
---

# Plan an app around an Atmee avatar

You turn a rough idea ("a pharmacy counter assistant") into a **brief**: one
page that says who the app is for, the one job it does, how the conversation
flows, who the avatar is, what it does through tools and what the page
shows, which API limits bite, and how the app will be proven live. Nothing is
built here; `create-avatar` and `build-avatar-app` build from the brief.

This is the Session API path: Atmee runs the conversation, the developer's
app surrounds it. While the person is still asking what Atmee is, what it
costs or which path fits (or wants only a face for their own voice agent),
use `atmee-product` first.

## The grill

Once per session, first run the update check in the `atmee-setup` skill (its
step 0), and go on whatever it says.

Ask in **rounds** until every line of the brief is settled. The question bank
is `references/questions.md`, ordered by round.

- **Number the questions and give your recommended answer with each**, so
  agreeing settles it. Ask through your question tool when you have one, the
  way `create-avatar`'s "How to ask the creator" says; without one, a
  numbered list with a ➡️ recommendation under each.
- **Ask only the frontier:** two to four questions whose prerequisites are
  settled. Tools wait until the job and the avatar's role are known.
- **Look facts up instead of asking:** the limits (`build-avatar-app`,
  "Current limits"), the plan's session length and price
  (`get_account`), the avatars they already have (`list_avatars`), their repo
  and stack.
- **Meet a limit head on:** when an answer runs into one, name it and offer
  the workaround, or write it down as an accepted gap. A brief that hides a
  limit fails in the first live run.
- Anything never asked keeps your recommendation; list those defaults with
  the brief, so the person sees what they accepted.

## Inspiration

When you can reach them (they are not public yet), the Atmee demo apps,
`https://github.com/Atmanity/atmee-ai-demos`, are
nineteen apps on the Avatar API: a recruiter on an agenda, a trainer with a
quiz, a companion that watches the screen, a game narrator, a livestream host,
and fourteen tutors, advisors and games under `apps/`. Each
`apps/<app>/README.md` gives its use case, architecture, how it uses the API
and the limits it hit; `docs/PORTING.md` and `docs/API-GAPS.md` hold what the
live runs taught. Read them for ideas and patterns that worked, as
inspiration: the brief fits this person's app, which may look like none of
them.

## The brief

```
# <Product name>: <one line>
User and job:   who opens it, where, and the one thing they leave with
Flow:           front door -> ... -> the done moment; with an agenda, its steps,
                each with the condition (about what was said) that ends it
Avatar:         role, name, persona in two lines, greeting, language,
                guardrails; a new avatar, or the id of one that fits
Tools:          | name | notify or await | when the avatar calls it | what the page does or returns |
Screen vs speech: what the page shows as the truth (scores, prices, verdicts,
                safety lines) and what the avatar only says
App to avatar:  the state (what is true now), notes, events (what happened), when each is sent
Transcript:     a chat panel or captions, one of them; the user's own words in it
Sessions:       one per visitor, or one host for many watching; how many
                avatars; the length cap; who pays
Limits:         each one that bites, with its workaround or "accepted gap"
Live test:      the run that proves it: what to say or type, the tool calls
                that must fire, the end state, the session ended
Deploy:         where it runs, how it is gated, the key's scope and spend cap
```

Done when every line holds an answer the person gave or accepted and the
Limits line has walked every item of `build-avatar-app`'s "Design around
these" that the app touches. Show the brief with the defaults, ask "Build
from this brief?", and on a yes save it in their repo (for example
`docs/brief.md`).

## Then

1. **`create-avatar`, with the Avatar line as its input.** The line's name
   and language start step 1 (the person still brings a photo and a voice,
   or generates them), and the rest becomes step 3's character. Make a
   purpose-built avatar: it greets and introduces itself as it was made, and
   its persona outweighs tool instructions (a chess player refuses to show
   slides). Pick a seed whose mouth reads clearly: a seed is one captured
   talking clip (the server's "candidate" and "take" mean a seed, and the
   API's `seed` field is an informational number, not the clip).
2. **`build-avatar-app`, from the brief.** Its "Prove it live" runs the
   brief's Live test line and its "Document and ship" the Deploy line; a
   platform gap found on the way goes back into the brief's Limits.

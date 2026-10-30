# Question bank

Draw each round's questions from here, skip what the idea, the repo or a
lookup already answers, and put your recommendation with each. The notes under
a question are what to weigh before recommending; they are not to be read out.

## Round 1: the product

1. **Who opens it, where, and why now?** One kind of user in one moment beats
   "anyone".
2. **What is the one job, the thing they leave with?** A booked slot, a
   screened candidate, a score, an answer. It becomes the done moment.
3. **One visitor at a time, or one host for many watching?** One live session
   per avatar, one user in it: one-to-one needs as many avatars as visitors
   at once. Many watching means one page in the room and a restream for the
   audience, with their chat relayed through the backend (no viewer tokens).
4. **Voice, typing, or both?** Voice is the product's point; typing is the
   fallback and what live tests run on.

## Round 2: the flow and the avatar

1. **Free conversation, or steps?** Steps become an agenda (a conductor):
   each step ends on a condition about what was said, never on a click or a
   tool call. The agenda can't see tool calls and the page is not told the
   step, so a page that follows the steps needs a tool that marks them.
2. **Who is the avatar?** A role that makes the app's actions natural for it
   (a croupier deals, a coach sets the plan). Name, two lines of persona,
   language. New, or one from `list_avatars` that already fits.
3. **How does it open?** It greets as it was made, before the app can say
   anything; an opening the app sends must not greet again.
4. **What must it never say or do?** In health, law or money it informs and
   points to a professional. Where a missed moment could hurt someone, the
   page detects it itself and shows it; the avatar's words are a second path.

## Round 3: actions and the screen

1. **What must the avatar DO, not just say?** Each action on the app is a
   tool, named the way this avatar would say it. Few, distinct tools.
2. **For each tool: an action (`notify`) or an answer (`await`)?** `await`
   holds the reply for the page's answer, at most 28 seconds, with a filler
   line the avatar picks before the result exists, so results lead with the
   outcome ("Not yet: ..."). Slow work is `notify`, then an event when done.
3. **What is the truth on screen?** Spoken words can't be forced: prices,
   verdicts, scores and safety lines are shown by the page; the app decides,
   the avatar performs.
4. **What does the app tell the avatar, and when?** `state` for what is true
   now (the screen), notes for what holds all session, events for what just
   happened. A button acts at once and tells the avatar by an event.
5. **Where does the conversation show?** A chat panel or captions, one of
   them, with the user's own words in it.

## Round 4: limits, proof, deploy

1. **The limits.** Read `build-avatar-app`'s "Current limits" and, for
   each item the app touches, settle a workaround or an accepted gap with the
   person. Most often: session length, one live session per avatar, the idle
   check-in, the audience, the persona being the avatar's.
2. **What run proves it works?** A script of what to say or type, the tool
   calls that must fire and in what order, the done moment, and the session
   ending with the reason you expect.
3. **Where does it run, and who can reach it?** Every start is a billed
   session: behind a sign-in or a shared password, a rate limit, and a key
   scoped to `sessions` and the app's avatar, with a spend cap.

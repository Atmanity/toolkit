# Agendas (conductor)

A conductor gives an avatar's conversations a fixed agenda: steps, each with
the avatar's instructions while the conversation is in it, and edges that say
when it moves on. Use one when the app's conversation has steps (a screening
interview, a lesson and quiz, an onboarding). A free conversation needs none;
`state` carries what the app knows.

## How it runs

- At the end of every turn a judge model checks each condition (`when`) on the
  current step's edges; the likeliest edge is taken. An edge without a
  condition is the default, usually back to the same step: "stay here until
  the condition holds".
- An edge marked `speak` lets the avatar go on into the next step without
  waiting for the user (`speakAfterS` seconds after its turn, 0.1 to 10). Chain
  them for a lecture; end the chain in a step that waits.
- A step without edges is the end.
- The app's `state` is added after the current step's instructions: put the
  fixed script in the graph and the changing facts (the job description, the
  slide on screen, the score) in `state`.
- The page is not told when the step changes. To follow the avatar (turn the
  slide, light up the agenda), match its captions; for verdicts, instruct a
  fixed opening ("Begin your verdict with Correct or Incorrect") and read it.

## Set it

The conductor belongs to the avatar, not the session: it applies from the next
session and stays until replaced. So keep the graph as a file in the
developer's repo and set it when it changes, not on every session start:

```bash
atmee avatars conductor set <avatarId> --graph agenda.json --speak-after 1.5
atmee avatars conductor get <avatarId>
atmee avatars conductor delete <avatarId> --yes
```

Or `set_conductor` / `get_conductor` from the avatars server, or `PUT
/v1/avatars/{avatarId}/conductor` with `{"graph": ..., "speakAfterS": 1.5}`
from a key with `avatars:write`. Two apps that need different agendas need two
avatars. v2 avatars only (`409 not_v2_avatar`).

The graph is checked when it is set and a refusal names the field (`400
invalid_conductor`, `/graph/nodes/2/edges/0/when: ...`); fix that field. The
`set_conductor` tool's description lists every rule and limit.

## Shapes that worked

A screening interview: one step per topic, each staying until its condition
holds (condensed from a recruiter app that ran on staging).

```json
{
  "start": "greet",
  "nodes": [
    {"id": "greet", "instructions": "Greet the candidate, say the call takes about ten minutes, and ask them to introduce themselves.",
     "edges": [{"to": "background", "when": "The candidate has introduced themselves."}, {"to": "greet"}]},
    {"id": "background", "instructions": "Ask about their current role and one relevant project. One question at a time.",
     "edges": [{"to": "wrap", "when": "You have heard about their role and one concrete project."}, {"to": "background"}]},
    {"id": "wrap", "instructions": "Thank them, say the next step comes within a week, and close warmly.", "emotion": "warm"}
  ]
}
```

A lesson: `speak` edges carry the avatar through the slides on its own, then a
step waits for questions and a quiz step asks and judges.

```json
{"id": "slide1", "instructions": "Teach slide 1 in about four sentences: ... Then move on.",
 "edges": [{"to": "slide2", "speak": true}]}
```

```json
{"id": "quiz1", "instructions": "Ask: \"...\" Wait for the answer. Begin your verdict with the word Correct or Incorrect (the answer is: ...), then say why in one sentence.",
 "edges": [{"to": "quiz2", "when": "The trainee has answered and you have said whether it was right."}, {"to": "quiz1"}]}
```

Write conditions about what was said ("The candidate has introduced
themselves"), which the judge can tell from the conversation, not about time
or clicks on the page.

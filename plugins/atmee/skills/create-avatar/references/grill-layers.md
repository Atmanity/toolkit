# Grill layers

The grill sharpens the character in up to three layers, one per round, so the
creator never faces every question at once. The avatar already exists and
talks: every layer is applied to it with `update_avatar` (Apply the character
in the drafting reference). All three layers are optional.

## How to ask

- Ask 2–4 questions per round, in one call of your question tool (SKILL.md,
  How to ask the creator).
- Put your recommended answer first in each question's options, marked
  "(Recommended)": "Should it discuss pricing?" with "No, point people to the
  sales team (Recommended)" and "Yes, quote list prices".
- Only ask questions whose prerequisites are settled. Don't ask about the
  greeting's tone before you know who the avatar talks to.
- Skip anything the creator's material already answers.
- The creator can pick an option or type their own answer to any question.

## The layers

**Layer 1: who it's for and its limits**

- Who will talk to the avatar, and why.
- What it must never say or do (guardrails).

**Layer 2: what it knows and how it opens**

- What goes in the what prompt's "What you know" section (the drafting
  reference): names, dates, prices, what it offers. It can change any time.
- Its greeting and first impression.

**Layer 3: how it speaks and who can find it**

- Speaking habits: how chatty, how formal, filler words, questions back.
  Describe them in plain words, then set them with `update_avatar`'s
  `conversationalHabits`, a map from each habit to
  `CONVERSATIONAL_HABIT_SCALE_LOW`, `CONVERSATIONAL_HABIT_SCALE_MEDIUM` or
  `CONVERSATIONAL_HABIT_SCALE_HIGH`.
- Visibility: private (only the creator) or unlisted (anyone with the link).
  It was created private. Public comes only later, on request (the
  people-and-publishing reference).
- Aliases and how the name is pronounced.

The language was settled in step 1. Change it only when the creator asks,
after saying that it may make new seeds (the default preview sentence
follows the language); then check `list_candidates` and judge any new seed
like the first (the candidates reference).

## After each round

1. Apply the answers to the draft and save an iteration with `applied: no`.
2. Show what changed as a short diff: field, before, after.
3. Apply the changed fields (Apply the character in the drafting reference),
   then mark the iteration `applied: yes`.
4. Ask with your question tool and wait: "Sharpen another layer?" with "Test
   <name> now (Recommended)" and "One more layer first". Never start the next
   layer on your own.

Anything you never asked keeps your recommended default. When the creator
moves on to the test, list those defaults in one short block, so they know
what they accepted without being asked.

---
iteration: 0
step: create # create | candidate-selected | draft | grill-layer-1|2|3 | refine | update | replace-portrait | replace-voice | test
applied: null # draft, grill, refine and update files: no when saved, yes once live on the avatar
date: 2026-10-19
environment: real # mock | real
avatars_server: https://api.atmanity.us/mcp
avatar_id: null # set from the create step on
real_person: null # null when not a real person; else { name: Jamie, permission: confirmed }
assets:
  # generated: upload_id + provider; the creator's own file: source (sources/me.jpg) + upload_id of this version's upload; a public link: url
  portrait: { upload_id: null, provider: null, source: null, url: null, copy: null } # copy: media/faces/face_1.jpg once saved
  voice: { upload_id: null, provider: null, source: null, url: null, copy: null }
  knowledge: [] # [{ source, upload_id, description }]
selected_candidate: null
paid_calls: [] # one per paid call made since the previous file: [{ tool, date, request_id, inputs, status, credits }]; status pending before the call, then done (credits set), failed or lost
---

# <Avatar name>: <step>

## Spec

- **Name:**
- **Preferred alias / spoken name / known aliases:**
- **Language:**
- **Visibility:**
- **Who prompt (who it is):**
- **What prompt (what it does with the person):**
- **Greeting:**
- **Self-introduction:**
- **Guardrails:**
- **Preview message:** (the default sentence unless one was set)
- **What it knows:** (the what prompt's "What you know" section)
- **Speaking habits:**
- **Title / description / tags (public only):**

## What changed

| Field | Before | After |
|---|---|---|

## Inputs used

Files, links and answers this step drew on.

## Note

One line on why, or what was learned (for example, "flat voice: lips barely
moved in two batches").

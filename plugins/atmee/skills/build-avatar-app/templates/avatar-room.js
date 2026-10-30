// The browser half of an Atmee avatar app: start a session through your own
// backend (server.mjs), join its LiveKit room as the user, wait until the
// avatar can talk, then talk to it and tell it what the app knows.
//
//   import { joinAvatar } from './avatar-room.js';
//   const avatar = await joinAvatar({ video, audio, mic, tools, onPhase, onCaption, onSpeaking, onVideo, onAudioBlocked, onEnd });
//   avatar.say('What does the blue plan cost?');            // a typed user turn, held while the avatar has the floor
//   avatar.app({ type: 'state', text: 'Cart: 2 items' });   // state | note | event | mute | unmute
//   const outcome = await avatar.sendFiles(input.files, { caption, onStatus });  // [{ name, status, reason }]
//   await avatar.setMicMuted(true);
//   await avatar.end();
//
// Call joinAvatar from a click: the session ends unless the user joins within
// 60 seconds of the avatar. The click may be too long ago to allow sound by the
// time the avatar arrives (Safari): onAudioBlocked(true, resume) asks for one
// more tap, which calls resume().
//
// The avatar can talk once its voice arrives (its audio track, or its first
// caption), and that is when the session goes live. Its face is not a gate: a
// session with no video at all (a voice-only runtime) still goes live, and the
// video plays whenever it arrives. onVideo(true) says the face is playing,
// onVideo(false) that it went away; show a voice-only placeholder until then.
// waitForVideo: true goes live only once the face plays, for apps that must
// not start without it.
//
// tools answers the app tools the backend declared (server.mjs, APP_TOOLS_FILE):
// one function per tool name, given the call's arguments and its id. What it
// returns is the result an await tool gives the avatar (an object is sent as
// JSON); what it throws, the avatar hears as "error: <message>".
//   tools: { show_slide: ({ n }) => { showSlide(n); }, get_score: async () => ({ score }) }
//
// sendFiles drops 1 to 10 files (a FileList or an array of File) into the call
// for the avatar to read and talk about, with an optional caption (cut to 500
// characters). It resolves once every file is ready or refused, one { name,
// status, reason } per file in order; onStatus({ index, name, status, reason })
// follows each file on the way (received, reading, ready, refused). It
// rejects, sending nothing, with too_many_files, or with files_unsupported
// when the avatar's runtime takes no files: check acceptsFiles() before
// offering a drop.
// Needs livekit-client 2.x (npm i livekit-client, or an import map) and floor.mjs.
import { Room, RoomEvent, RpcError, Track } from 'livekit-client';
import { createFloor } from './floor.mjs';

const CHAT = 'lk.chat';                 // a typed user turn; the avatar answers it
const APP = 'atmee.app';                // the app's state, notes, events, mute
const TRANSCRIPTION = 'lk.transcription';
const TOOL = 'atmee.tool';              // an app tool call, as an RPC; the reply is its result
const FILE = 'atmee.file';              // a drop of files, to the agent alone; never lk.chat, which drops attachments
const FILE_STATUS = 'atmee.file.status'; // the agent's answers, one per file and status
const ARRIVAL_TIMEOUT_MS = 90_000;      // from joining the room to the avatar's voice (or, with waitForVideo, its face)
const WARMING_LIMIT_MS = 6 * 60_000;    // how long to keep retrying a 503 (no rendering capacity yet)
const LINK_DELAY_MS = 2_500;            // atmee.app and file drops are heard only once the agent links the user
const FILES_ATTRIBUTE = 'atmee.files';  // "1" on an agent that takes files
const MAX_FILES = 10;                   // in one drop; the agent checks types and sizes
const MAX_CAPTION = 500;                // characters; the agent reads no more of a drop's caption
const FILE_WAIT_MS = 120_000;           // a sent drop this long without an answer is refused (failed)

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
const failure = (code, message) => Object.assign(new Error(message || code), { code });
const withReason = (fields, reason) => (reason ? { ...fields, reason } : fields);

// Phases: starting -> warming (retrying) -> connecting -> arriving -> live
// (detail: maxSeconds, and video: whether the face is playing yet).
// Rejects with an Error whose .code is the API's error (avatar_busy,
// avatar_not_ready, insufficient_credits, ...) or arrival_timeout (the avatar
// never joined: no voice, no caption, no face).
export async function joinAvatar({
  api = '/api', video, audio, mic, tools, waitForVideo = false, arrivalTimeoutMs = ARRIVAL_TIMEOUT_MS,
  onPhase = () => {}, onCaption = () => {}, onSpeaking = () => {}, onVideo = () => {}, onAudioBlocked = () => {}, onEnd = () => {},
} = {}) {
  const stopMic = () => { if (mic) mic.getTracks().forEach((t) => t.stop()); };
  onPhase('starting');
  const session = await startSession(api, onPhase).catch((err) => { stopMic(); throw err; });
  // From here the avatar is waiting in the room: a closed tab must end the session.
  const endUrl = `${api}/session/${session.sessionId}/end`;
  let ended = false;
  const onPageHide = () => { if (!ended) navigator.sendBeacon(endUrl); };
  addEventListener('pagehide', onPageHide);
  onPhase('connecting');

  const room = new Room({ adaptiveStream: true, dynacast: true });
  let linked = false, lastState, micTrack, silence;
  const outbox = [];   // [topic, text] sent before the agent links the user
  const send = async (topic, text) => {
    if (!linked) {
      outbox.push([topic, text]);
      return;
    }
    await room.localParticipant.sendText(text, { topic });
  };
  const floor = createFloor((text) => send(CHAT, text));
  const drops = createDrops();
  const endOnServer = () => fetch(endUrl, { method: 'POST', keepalive: true }).catch(() => {});
  const release = () => {
    ended = true;
    removeEventListener('pagehide', onPageHide);
    floor.close();
    stopMic();
    silence?.close();
    drops.refuse('unreadable');
  };
  const finish = (reason) => {
    if (ended) return;
    release();
    onEnd(reason);
  };

  // The avatar has joined when its voice (audio, or a first caption) or its
  // face arrives; with waitForVideo, only its face counts.
  let heard, seen, faceOn = false;
  const voice = new Promise((resolve) => { heard = resolve; });
  const face = new Promise((resolve) => { seen = resolve; });
  const showFace = (on) => {
    if (on === faceOn) return;
    faceOn = on;
    if (on) seen();
    onVideo(on);
  };

  // The avatar's words, one stream per utterance. Every stream is marked
  // interim, so a stream that has finished is a finished utterance. Streams
  // sent as this participant are the user's own words, transcribed.
  room.registerTextStreamHandler(TRANSCRIPTION, async (reader, participant) => {
    if (participant?.identity === room.localParticipant.identity) return;
    heard();
    floor.utterance();
    let text = '';
    try {
      for await (const chunk of reader) {
        text += chunk;
        onCaption(text, false);
      }
    } catch {
      // the stream was cut off; keep what arrived
    }
    if (text.trim()) onCaption(text, true);
  });

  // The agent's answers for dropped files, one JSON object per file and
  // status, taken from the agent alone.
  room.registerTextStreamHandler(FILE_STATUS, async (reader, participant) => {
    const agent = agentIn(room);
    if (!agent || participant?.identity !== agent.identity) return;
    let answer;
    try {
      answer = JSON.parse(await reader.readAll());
    } catch {
      return;
    }
    if (typeof answer?.batchId === 'string') drops.answer(answer);
  });
  const acceptsFiles = () => agentIn(room)?.attributes[FILES_ATTRIBUTE] === '1';
  // Sends a drop to the agent alone, the caption as text and each file as
  // its own stream on the same topic. sendText resolves once every file is
  // out; its id is the drop's batchId.
  const sendDrop = async (drop, caption) => {
    const agent = agentIn(room);
    if (!acceptsFiles()) {
      drop.fail(agent ? failure('files_unsupported', 'This avatar does not take files') : failure('no_agent', 'The avatar is not in the room'));
      return;
    }
    const cut = caption == null ? undefined : Array.from(String(caption)).slice(0, MAX_CAPTION).join('');
    const info = await room.localParticipant.sendText(JSON.stringify({ caption: cut }), {
      topic: FILE, attachments: drop.files, destinationIdentities: [agent.identity],
    }).catch(() => null);
    if (info) drops.sent(drop, info.id);
    else drop.refuse('failed');
  };

  // App tool calls, registered before connecting so none finds the page without
  // a handler. The call and the reply are each at most 15 KiB.
  if (tools) {
    room.registerRpcMethod(TOOL, async ({ payload }) => {
      const { id, name, arguments: args } = JSON.parse(payload);
      const run = Object.hasOwn(tools, name) ? tools[name] : null;
      if (typeof run !== 'function') throw new RpcError(1500, `the app has no tool ${name}`);
      try {
        const result = await run(args, { id });
        if (result === undefined || result === null) return 'ok';
        return typeof result === 'string' ? result : JSON.stringify(result);
      } catch (err) {
        throw new RpcError(1500, String(err?.message ?? err).slice(0, 200));
      }
    });
  }

  if (video) {
    Object.assign(video, { muted: true, playsInline: true, autoplay: true });
    video.addEventListener('playing', () => showFace(true));
  }
  room.on(RoomEvent.TrackSubscribed, (track) => {
    if (track.kind === Track.Kind.Video) {
      if (!video) return showFace(true);
      track.attach(video);
      video.play().catch(() => {});
    } else if (track.kind === Track.Kind.Audio) {
      if (audio) track.attach(audio);
      heard();
    }
  });
  room.on(RoomEvent.TrackUnsubscribed, (track) => {
    if (track.kind === Track.Kind.Video) {
      if (video) track.detach(video);
      showFace(false);
    }
  });
  room.on(RoomEvent.ActiveSpeakersChanged, (speakers) => {
    const now = speakers.some((p) => p.identity !== room.localParticipant.identity);
    if (floor.speaking(now)) onSpeaking(now);
  });
  room.on(RoomEvent.AudioPlaybackStatusChanged, () => onAudioBlocked(!room.canPlaybackAudio, () => room.startAudio()));
  // A full reconnect cuts off the files still streaming: a drop still
  // uploading is unreadable. Drops already out wait on; the agent reads them.
  room.on(RoomEvent.Reconnecting, () => drops.refuseUploading('unreadable'));
  room.on(RoomEvent.Disconnected, (reason) => { linked = false; finish(reason); });

  let arrivalTimer;
  try {
    await room.connect(session.serverUrl, session.userToken);
    await room.startAudio().catch(() => {});
    if (mic) {
      const published = await room.localParticipant.publishTrack(mic.getAudioTracks()[0], { source: Track.Source.Microphone });
      micTrack = published.track;
    } else {
      silence = await publishSilence(room);
    }
    onPhase('arriving');
    const timeout = new Promise((resolve) => { arrivalTimer = setTimeout(resolve, arrivalTimeoutMs, 'timeout'); });
    const arrival = waitForVideo ? face : Promise.race([voice, face]);
    if ((await Promise.race([arrival, timeout])) === 'timeout') {
      throw failure('arrival_timeout', waitForVideo ? 'The avatar did not appear in time' : 'The avatar did not join in time');
    }
  } catch (err) {
    release();
    room.disconnect();
    await endOnServer();
    throw err.code ? err : failure('connect_failed', err.message);
  } finally {
    clearTimeout(arrivalTimer);
  }

  await sleep(LINK_DELAY_MS);
  linked = true;
  while (outbox.length) await send(...outbox.shift());
  onPhase('live', { maxSeconds: session.maxSeconds, video: faceOn });

  return {
    session, room,
    say: (text) => floor.say(text),
    async app(message) {
      if (message.type === 'state' && message.text === lastState) return;
      // An event counts before it goes, so a turn typed right after waits for it.
      const withdraw = message.type === 'event' ? floor.event(message.ttl_s) : null;
      try {
        await send(APP, JSON.stringify(message));
      } catch (err) {
        withdraw?.();
        throw err;
      }
      // A state counts once it is out, so one that failed can be sent again.
      if (message.type === 'state') lastState = message.text;
    },
    // Whether the avatar's runtime takes files (its agent's atmee.files attribute).
    acceptsFiles,
    async sendFiles(files, { caption, onStatus = () => {} } = {}) {
      const list = [...(files ?? [])];
      if (list.length > MAX_FILES) throw failure('too_many_files', `At most ${MAX_FILES} files in one drop`);
      if (!list.length) return [];
      const drop = openDrop(list, onStatus);
      drops.add(drop);
      if (ended) drop.refuse('unreadable');
      else sendDrop(drop, caption);
      return drop.done;
    },
    async setMicMuted(muted) {
      if (micTrack) await (muted ? micTrack.mute() : micTrack.unmute());
    },
    async end() {
      if (ended) return;
      finish('ended');
      // End on the server first: leaving first settles the session as user_left.
      await endOnServer();
      await room.disconnect();
    },
  };
}

// One drop of files, each answered once: answer() takes the agent's status
// messages, refuse() answers every file still open on the page's behalf. done
// resolves when every file is ready or refused, and rejects (fail) when the
// drop could not go at all.
function openDrop(files, onStatus) {
  const outcome = files.map(() => null);
  let resolve, reject, timer;
  const done = new Promise((res, rej) => { resolve = res; reject = rej; });
  const close = () => { drop.over = true; clearTimeout(timer); };
  // The agent reads files in turn, so a drop may wait long between answers:
  // it fails only after FILE_WAIT_MS without any.
  const wait = () => {
    clearTimeout(timer);
    timer = setTimeout(() => drop.refuse('failed'), FILE_WAIT_MS);
  };
  const drop = {
    files, done, batchId: null, over: false,
    answer({ index, name, status, reason }) {
      if (drop.over || !Number.isInteger(index) || !files[index] || outcome[index]) return;
      name = name || files[index].name;
      onStatus(withReason({ index, name, status }, reason));
      if (status === 'ready' || status === 'refused') outcome[index] = withReason({ name, status }, reason);
      if (outcome.every(Boolean)) {
        close();
        resolve(outcome);
      } else if (drop.batchId) {
        wait();
      }
    },
    refuse(reason) {
      files.forEach((file, index) => drop.answer({ index, name: file.name, status: 'refused', reason }));
    },
    fail(err) { close(); reject(err); },
    // Every file is out: from now on the drop waits FILE_WAIT_MS at most for each answer.
    sent(batchId) {
      drop.batchId = batchId;
      if (!drop.over) wait();
    },
  };
  return drop;
}

// The drops not yet answered in full. LiveKit resolves sendText, and so tells
// the page a drop's batchId, only once every file is out, so the agent's first
// answers can come before it: they wait while a drop is still uploading.
function createDrops() {
  const open = new Set();
  let early = [];
  const uploading = () => [...open].some((drop) => !drop.batchId);
  return {
    add(drop) {
      open.add(drop);
      drop.done.finally(() => open.delete(drop)).catch(() => {});
    },
    answer(answer) {
      const drop = [...open].find((d) => d.batchId === answer.batchId);
      if (drop) drop.answer(answer);
      else if (uploading()) early.push(answer);
    },
    sent(drop, batchId) {
      drop.sent(batchId);
      const mine = early.filter((answer) => answer.batchId === batchId);
      early = early.filter((answer) => answer.batchId !== batchId);
      if (!uploading()) early = [];
      mine.forEach((answer) => drop.answer(answer));
    },
    // Refuses what is not answered yet: in every drop, or in those still uploading.
    refuse(reason) { open.forEach((drop) => drop.refuse(reason)); },
    refuseUploading(reason) { open.forEach((drop) => { if (!drop.batchId) drop.refuse(reason); }); },
  };
}

// The conversation's agent. The renderer publishes the avatar on its behalf
// and names it in lk.publish_on_behalf; with no renderer, it is the agent
// participant that publishes on nobody's behalf.
function agentIn(room) {
  const others = [...room.remoteParticipants.values()];
  const named = others.map((p) => p.attributes?.['lk.publish_on_behalf']).find(Boolean);
  return room.remoteParticipants.get(named) ?? others.find((p) => p.isAgent && !p.attributes?.['lk.publish_on_behalf']);
}

// A 503 no_capacity means no rendering capacity yet: wait Retry-After (at least
// 5 s; 30 s when it is missing) and start again, for up to WARMING_LIMIT_MS.
// Any other failure, a bodyless 503 (embeds turned off) included, is final.
async function startSession(api, onPhase) {
  const since = Date.now();
  for (;;) {
    const res = await fetch(`${api}/session`, { method: 'POST' }).catch(() => null);
    if (!res) throw failure('backend_unreachable', 'Could not reach the app server');
    const body = await res.json().catch(() => ({}));
    if (res.ok) return body;
    const waited = Date.now() - since;
    const wait = body.retryAfter ? Math.max(body.retryAfter, 5) : 30;
    if (body.error !== 'no_capacity' || waited + wait * 1000 > WARMING_LIMIT_MS) throw failure(body.error || `http_${res.status}`, body.message);
    onPhase('warming', { retryIn: wait, waited: Math.round(waited / 1000) });
    await sleep(wait * 1000);
  }
}

// Typing only: publish a silent microphone, as the tested typing-only sessions
// did. A stream destination with nothing connected to it is silent. Returns
// the AudioContext, to close when the session ends.
async function publishSilence(room) {
  const ctx = new AudioContext();
  const out = ctx.createMediaStreamDestination();
  await room.localParticipant.publishTrack(out.stream.getAudioTracks()[0], { name: 'silence' });
  return ctx;
}

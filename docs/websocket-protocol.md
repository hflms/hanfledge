# WebSocket protocol

Hanfledge uses WebSocket connections for real-time AI dialogue
streaming between students and the multi-agent orchestrator. This
document specifies the WebSocket protocol, message formats, and
event types.

## Connection

### Endpoint

```
ws://<host>/api/v1/sessions/:id/stream
```

Replace `:id` with the session ID obtained from
`POST /api/v1/activities/:id/join`.

### Authentication

Pass the JWT token as a query parameter:

```
ws://localhost:8080/api/v1/sessions/42/stream?token=eyJhbGciOi...
```

The server validates the token before upgrading the HTTP connection
to WebSocket. An invalid or expired token results in an HTTP 401
response (the connection is not upgraded).

### Connection parameters

| Parameter | Value | Description |
|-----------|-------|-------------|
| Pong wait | 60 seconds | Client must respond to pings within this window |
| Ping interval | 30 seconds | Server sends pings at this interval |
| Write wait | 10 seconds | Maximum time for a write operation |

The server sends periodic ping frames. If the client does not respond
with a pong within 60 seconds, the server closes the connection.

## Message format

All messages use JSON encoding. Every message includes an `event`
field that identifies the message type.

### Client to server

```json
{
  "event": "<event_type>",
  "payload": { ... },
  "timestamp": 1711900000
}
```

### Server to client

```json
{
  "event": "<event_type>",
  "payload": { ... }
}
```

## Client events

These events are sent from the client (student) to the server.

### `user_message`

Sends a text message from the student to the AI dialogue system.

```json
{
  "event": "user_message",
  "payload": {
    "text": "What is the Pythagorean theorem?"
  },
  "timestamp": 1711900000
}
```

The server processes this message through the multi-agent pipeline
(Strategist, Designer, Coach, Critic) and streams the response back
as a sequence of server events.

### `voice_start`

Signals the start of a voice recording. Used with the ASR (Automatic
Speech Recognition) feature.

```json
{
  "event": "voice_start",
  "payload": {},
  "timestamp": 1711900000
}
```

### `voice_data`

Sends a chunk of base64-encoded audio data.

```json
{
  "event": "voice_data",
  "payload": {
    "data": "<base64-encoded-audio-chunk>"
  },
  "timestamp": 1711900000
}
```

### `voice_end`

Signals the end of voice recording. The server transcribes the
accumulated audio and processes it as a text message.

```json
{
  "event": "voice_end",
  "payload": {},
  "timestamp": 1711900000
}
```

## Server events

These events are sent from the server to the client.

### `agent_thinking`

Indicates that the AI agent is processing. The client can display a
loading indicator.

```json
{
  "event": "agent_thinking",
  "payload": {
    "agent": "coach"
  }
}
```

The `agent` field indicates which agent is currently active
(`strategist`, `designer`, `coach`, or `critic`).

### `token_delta`

Streams a token (text fragment) from the AI response. The client
appends each token to build the complete response incrementally.

```json
{
  "event": "token_delta",
  "payload": {
    "token": "The Pythagorean "
  }
}
```

### `ui_scaffold_change`

Notifies the client that the scaffold level has changed for the
current knowledge point. The client updates the UI to reflect the
new scaffold level.

```json
{
  "event": "ui_scaffold_change",
  "payload": {
    "scaffold_level": "medium",
    "kp_id": 42
  }
}
```

Scaffold levels: `high` (maximum guidance), `medium`, `low`
(minimal guidance, student-led).

### `turn_complete`

Signals that the AI has finished generating its response for the
current turn. The client re-enables the input field.

```json
{
  "event": "turn_complete",
  "payload": {
    "interaction_id": 123,
    "tokens_used": 256,
    "skill_id": "socratic-questioning"
  }
}
```

### `skill_ui`

Sends skill-specific UI data for rendering in the client. Different
skills produce different UI components (quiz questions, role-play
scenarios, presentation slides).

```json
{
  "event": "skill_ui",
  "payload": {
    "skill_id": "quiz-generation",
    "component": "QuestionCard",
    "data": {
      "question": "What is the value of c in a 3-4-5 triangle?",
      "options": ["3", "4", "5", "7"],
      "type": "multiple_choice"
    }
  }
}
```

### `error`

Indicates a server-side error. The client displays the error message.

```json
{
  "event": "error",
  "payload": {
    "message": "Session has ended",
    "code": "session_completed"
  }
}
```

### `teacher_intervention`

Indicates that a teacher has intervened in the session. This happens
when a teacher uses the takeover or whisper feature.

```json
{
  "event": "teacher_intervention",
  "payload": {
    "type": "whisper",
    "message": "Try guiding the student toward the concept of similar triangles."
  }
}
```

Intervention types:

| Type | Description |
|------|-------------|
| `takeover` | Teacher takes control of the AI dialogue |
| `whisper` | Teacher injects a hint into the AI context (invisible to student) |
| `release` | Teacher releases control back to the AI |

### `voice_transcript`

Returns the transcription result from a voice recording.

```json
{
  "event": "voice_transcript",
  "payload": {
    "text": "What is the Pythagorean theorem?",
    "confidence": 0.95
  }
}
```

## Message flow

A typical dialogue turn follows this sequence:

```
Client                              Server
  │                                    │
  │─── user_message ──────────────────>│
  │                                    │
  │<─── agent_thinking (strategist) ───│
  │<─── agent_thinking (designer) ─────│
  │<─── agent_thinking (coach) ────────│
  │                                    │
  │<─── token_delta ───────────────────│
  │<─── token_delta ───────────────────│
  │<─── token_delta ───────────────────│
  │           ... (streaming) ...      │
  │                                    │
  │<─── ui_scaffold_change ────────────│  (optional)
  │<─── skill_ui ──────────────────────│  (optional)
  │                                    │
  │<─── turn_complete ─────────────────│
  │                                    │
```

## Voice input flow

Voice input uses a three-phase protocol:

```
Client                              Server
  │                                    │
  │─── voice_start ───────────────────>│
  │─── voice_data (chunk 1) ──────────>│
  │─── voice_data (chunk 2) ──────────>│
  │           ... (streaming) ...      │
  │─── voice_end ─────────────────────>│
  │                                    │
  │<─── voice_transcript ──────────────│
  │                                    │
  │  (transcript is then processed     │
  │   as a user_message automatically) │
  │                                    │
  │<─── agent_thinking ────────────────│
  │<─── token_delta ───────────────────│
  │           ...                      │
  │<─── turn_complete ─────────────────│
```

## Teacher intervention flow

Teachers can intervene in active sessions through the REST API
(`POST /api/v1/sessions/:id/intervention`). The intervention is
pushed to the student's WebSocket connection:

```
Teacher (REST)                Student (WebSocket)
  │                                    │
  │── POST intervention ──> Server ────│
  │                            │       │
  │                            │──────>│ teacher_intervention
  │                            │       │
  │                            │  (AI adjusts behavior    │
  │                            │   based on intervention) │
```

## Connection management

### Reconnection

If the WebSocket connection drops, the client is responsible for
reconnecting. The recommended strategy:

1. Attempt reconnection after 1 second.
2. Use exponential backoff up to 30 seconds.
3. Limit to 8 reconnection attempts.
4. On successful reconnection, reload session state via
   `GET /api/v1/sessions/:id`.

### Session lifecycle

| State | Description |
|-------|-------------|
| `active` | Session is in progress, WebSocket accepts messages |
| `completed` | Session has ended normally, WebSocket is closed |
| `abandoned` | Session was abandoned, WebSocket is closed |

When a session transitions to `completed` or `abandoned`, the server
sends a `turn_complete` event and closes the WebSocket connection.

## Security

All WebSocket connections require a valid JWT token. The server
validates the following before processing any message:

1. Token is valid and not expired.
2. The authenticated user is the session's student (or a teacher
   with intervention access).
3. The session is in `active` status.
4. Student input passes the injection guard check (blocks prompt
   injection attempts).
5. Student input is processed by the PII redactor before reaching
   the LLM (removes personal information).

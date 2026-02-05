# Tools

## Purpose

Define how interactive tools (e.g., Career Copilot) are registered, gated, and tracked in Explore.

## Tool registry

`tools` is the catalog. Each tool stores:

- name + slug
- category
- active flag
- paid-only flag
- `config_json` (tool-specific behavior)

`stores.EnsureDefaultTools()` seeds defaults on startup (currently Career Copilot).

## Tool config JSON

### Free rules

- `max_free_responses`: number of user-initiated responses allowed (Career Copilot = 2).
- `max_free_audio_inputs`: number of voice inputs allowed (Career Copilot = 1).
- `single_use_free_flow`: once the free flow is used, resume upload is blocked until upgrade.
- `require_sign_in`: tools require auth before use.

### Tracking

Flags for which actions are logged (`track_uploads`, `track_responses`, `track_audio`, `track_stages`).

### Stages

Ordered stage keys + labels used by the UI and analytics (upload → analysis → focus → subfocus → questions → mentor_response → chat).

## Usage state

`tool_usages.usage_state_json` stores per-user counters:

- `has_used_free_flow`
- `free_user_responses_used`
- `free_audio_inputs_used`
- `completed_stages`

The backend enforces limits in `services/tool_service.go`.

## APIs

- `GET /api/tools`: list active tools
- `GET /api/tools/:slug`: tool + config (+ usage_state if signed in)
- `POST /api/tools/:slug/action`: records usage and enforces limits
- `POST /api/tools/:slug/resume`: upload PDF resume, run Gemini analysis, return ATS score + quick wins + resume_text
- `POST /api/tools/:slug/mentor`: generate initial mentor response from resume_text + analysis + selections
- `POST /api/tools/:slug/chat`: follow-up chat response using resume_text + analysis + history (returns `reply_markdown`)
- `POST /api/tools/:slug/transcribe`: audio upload -> text transcript (voice input)

Actions used by Career Copilot:

- `session_start`
- `resume_upload`
- `stage_complete` (with `stage_key`)
- `flow_complete`
- `response` (with `used_audio`)

## Career Copilot flow

1. Resume upload (blocks if `has_used_free_flow` and no entitlement).
2. ATS + resume analysis (free, non-counted).
3. Focus + subfocus selections (focus stage now shows all focus->subfocus paths so users can preview or preselect a path before moving on).
4. Context questions (one-by-one).
5. Initial mentor response (free, non-counted). Resume text and answers are stored client-side for later prompts.
6. Follow-up chat (2 free responses, 1 free voice input) with optional voice transcription.

### Career Copilot guided paths

- `Career Growth -> Promotion roadmap`
- `Career Growth -> Leadership skills`
- `Career Growth -> Salary negotiation`
- `Career Growth -> Visibility strategy`
- `Skill Development -> Tech stack depth`
- `Skill Development -> Product instincts`
- `Skill Development -> Data + analytics`
- `Skill Development -> Design thinking`
- `Career Switching -> Role transition`
- `Career Switching -> Industry shift`
- `Career Switching -> Remote relocation`
- `Career Switching -> First-time manager`
- `Resume Improvement -> ATS optimization`
- `Resume Improvement -> Storytelling`
- `Resume Improvement -> Portfolio alignment`
- `Resume Improvement -> Project impact`
- `Interview Preparation -> Behavioral interviews`
- `Interview Preparation -> System design`
- `Interview Preparation -> Case interviews`
- `Interview Preparation -> Portfolio walkthrough`
- `General Career Advice -> Clarity + direction`
- `General Career Advice -> Work-life balance`
- `General Career Advice -> Confidence boost`
- `General Career Advice -> Networking strategy`

Paywall appears on:

- resume upload after free flow is used
- 3rd user-initiated response
- 2nd voice input

## Analytics rollup

Tool events are aggregated daily into `tool_daily_metrics`. A background job runs every 24 hours to roll up the previous day and delete older `tool_events` based on the analytics retention window (scoring window + 7 days).

## Gemini configuration

Set these in `.env` (see `sample.env`):

- `GEMINI_API_KEY` (required for resume analysis, mentor response, chat, and transcription)
- `GEMINI_MODEL` (defaults to `gemini-1.5-flash`)
- `GEMINI_API_BASE` (optional override for API base URL)

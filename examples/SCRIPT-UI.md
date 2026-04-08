Here’s a roadmap that matches the **v1 implicit-mode e2e** you chose (shared sessions + control plane), while still giving you a **weekly “impact unit” you can ship**.

I’m organizing it around what Trek *is* (targeted, time-bounded verbosity without global log level changes) , what it’s *not* (no storage/querying platform) , and the paid tier value (“control and safety”: shared sessions, RBAC, audit, policies, notifications, env separation) .

---

## Roadmap principle

**Every week ends with an externally legible artifact** (tagged release, demo app, doc, fixture set, or hosted endpoint). And you follow the PRD’s rollout order: **local mode → dogfood → shared store/control plane** .

---

## Phase 0: Truth anchor (Week 1)

**Impact artifact:** “Trek Conformance Suite v0”

* Lock the evaluator semantics now (Decision fields, reason codes, tie-breaking). 
* Build the fixture runner contract: every SDK must expose `decide(now, request_context, sessions) -> decision` and run the same JSON fixtures. 
* Include at least: match/no-match, expiry, tie-break scenarios, and (even if unused) token cases so the semantics are frozen. 

Why first: it prevents cross-language drift and gives you a “definition of done” for the core brain.

---

## Phase 1: Free v0 (Weeks 2–4)

This is the smallest end-to-end loop where the product’s success criteria is true: “extra logs only for matched requests; expiration; guardrails.” 

### Week 2 — Evaluator library v0.1

**Impact artifact:** tagged release + fixtures passing

* Implement selector matching + TTL expiry + tie-break determinism. 
* Emit Decision + reason codes. 

### Week 3 — Local store + CLI v0.1

**Impact artifact:** demoable CLI

* Local sessions (in-memory + optional file) for v0. 
* CLI: start/stop/list/inspect request context. 

### Week 4 — Host integration v0.1 (pick one stack)

**Impact artifact:** “hello-trek” demo service

* One integration point + request-scoped logger handle. 
* Critical constraint: per-request behavior without changing global logger state. 
* Ship safety defaults: TTL required/bounded, caps enabled, safe failure, redaction hook interface. 
* Add basic metrics (sessions active, requests matched, decision latency). 

---

## Phase 2: Dogfood hardening (Weeks 5–6)

**Impact artifact each week:** a documented incident runbook + one hardening release

### Week 5 — “Incident story” dogfood

* Run the core user story against your own demo service: “target one user/request for 10–30 minutes, auto-off.” 
* Write the runbook as a README section (commands + what you see in logs).

### Week 6 — Guardrails + ergonomics

* Make sure caps + safe failure are real and tested. 
* Improve “why didn’t it match?” via `inspect` + reason codes. 

This phase buys you confidence and lowers the “obvious failure” fear *before* you touch SaaS.

---

## Phase 3: v1 core backend (Weeks 7–9)

This is where “team mode” starts: sessions apply across instances via a shared store. 

### Week 7 — Shared session store + API v0.1

**Impact artifact:** hosted endpoint + SDK fetch path behind a flag

* Implement the paid v1 “shared store via control plane API.” 
* Minimal endpoints: create/revoke/list/get sessions. 
* Add caching in the host integration so decision latency stays “negligible.” 

### Week 8 — Auth/RBAC + audit trail v0.1

**Impact artifact:** audit log view via API

* Team mode must have auth + RBAC + audit. 
* Enforce “who did this and why” as part of paid tier value. 
* Start requiring `created_by` / `reason` (your PRD marks these as paid-tier fields). 

### Week 9 — Policy enforcement + notifications v0.1

**Impact artifact:** org policy config + webhook firing

* Policy knobs: max TTL, allowed selectors, caps defaults. 
* Notifications (Slack/webhook) are explicitly part of paid tier. 

---

## Phase 4: v1 control plane UX (Weeks 10–11)

You can keep this minimal: Trek is not an “observability UI,” it’s a sessions control plane. 

### Week 10 — Minimal web UI (or a “team CLI”)

**Impact artifact:** usable control surface

* Create session (selector + level + TTL + caps + labels) 
* List active + revoke + view audit events. 

### Week 11 — Environment separation

**Impact artifact:** prod/stage/dev partitioning

* Environment separation is part of paid tier value. 
* Make it impossible to accidentally start a prod session from “dev” context.

---

## Phase 5: “Sellable v1” packaging (Week 12)

**Impact artifact:** “v1 incident story” + hosted demo org

* Write the single narrative incident story you asked for (now backed by real endpoints + UI). (This is the doc you can hand someone.)
* Clean install docs: host integration, required request context extraction, safety defaults.  

---

## What to explicitly defer (so you don’t rebuild Traverzer-hell)

* Mode B token-required sessions: keep in spec but don’t build unless asked (you already decided). Token semantics can stay frozen via fixtures even if unshipped. 
* “More dynamic” capture modules: nice later, but only after v1 is stable, because they increase PII/cost risk without careful defaults. 
* WASM core: explicitly a “later leverage move,” not v0/v1. 

---

## If you want the simplest “impact week” starting next week

Do **Week 1 + Week 2** first: conformance suite + evaluator passing fixtures. It’s extremely “you”: design-y, deterministic, and it becomes the spine for everything else. 

If you tell me what your first implementation stack is (Go is the obvious default given your background, but your call), I’ll translate Week 1–4 into a concrete repo structure + exact deliverables per week (files that should exist, commands that should work, and the “done definition” for each).

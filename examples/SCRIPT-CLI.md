Yes. You can do a **fully CLI-driven v1 demo** that’s just as “wow” as the UI version—because v1’s “impressive” part is the *control plane behavior* (shared sessions + governance), not a dashboard. The CLI is simply a different front-end to the same control API surface (start/stop/list/get), which you already framed as “CLI/HTTP/gRPC—your choice.” 

Below is a tight script that proves: **targeted verbosity**, **TTL auto-off**, **caps/safe failure posture**, and **team governance** (RBAC/audit/policy/env separation) while still relying on your existing log sink (because Trek is not log storage/query).  

---

## CLI-only v1 demo: “The 2:13am incident” (implicit mode only)

### What’s running

* Demo “shop” service behind a load balancer with **2 instances** (to prove sessions apply across instances via shared store) 
* Your normal logs (stdout tail, or whatever you already use). Trek stays out of storage/query. 

### Terminal A — “Viewer”

Tail logs (whatever your stack is). The point is: logs are unchanged until you create a session.

### Terminal B — “Control” (`trek`)

#### 1) Prove governance exists (fast)

Show environment separation + policies (paid additions). 

```bash
trek env switch prod
trek policies get
```

Now prove policy enforcement is real (max TTL is explicitly a control-plane policy). 

```bash
# intentionally too long; should fail
trek session create --tenant t1 --route /checkout --level debug --ttl 2h --reason "INC-4821"
# => ERROR: ttl exceeds org max (policy)
```

#### 2) Start a real debug session (the core act)

A Debug Session is selector + level + TTL (+ caps/labels), with “who/why” in paid tier. 

```bash
trek session create \
  --tenant t1 \
  --route /checkout \
  --level debug \
  --ttl 15m \
  --label incident=INC-4821 \
  --reason "Checkout intermittent 500s for tenant t1"
# => session_id: s_abc123
```

List sessions to show it’s active:

```bash
trek session list
```

#### 3) Generate traffic (Terminal C, or a second split)

Hit `/checkout` as tenant `t1` and as tenant `t2`.

**What you show in Terminal A (logs):**

* For `t1`/`/checkout`, debug lines suddenly appear.
* For `t2`/`/checkout`, logs stay normal (targeted verbosity without global level changes). 

This works because matching is defined as: *all specified selector fields match request context* (tenant + route here). 

#### 4) Prove explainability: “why did this match?”

Your CLI already includes an `inspect --request-context <json>` concept. 
Show the Decision fields (`matched`, `effective_level`, `reason_code`, etc.). 

```bash
trek inspect --request-context '{
  "tenant_id":"t1",
  "route":"/checkout",
  "request_id":"r-777",
  "user_id":"u-9",
  "custom":{"plan":"pro"}
}'
# => matched=true, session_id=s_abc123, effective_level=debug, reason_code=MATCHED
```

#### 5) Prove “auto-off” + audit trail

TTL mandatory + auto-expiry is a core goal. 
So for demo, make TTL tiny (e.g., 30s) or just revoke manually.

```bash
# either wait for expiry…
trek session list --watch

# …or revoke
trek session revoke s_abc123 --yes
```

Then show audit (paid addition). 

```bash
trek audit list --since 30m
# => created_by, reason, created_at, revoked_at/expired_at
```

---

## Why this is “just as impressive”

Because it demonstrates the real product promises:

* **Targeted, time-bounded verbosity** without global log level changes 
* **Shared sessions across instances via control plane** (v1) 
* **Org governance**: policies + RBAC/audit + env separation are explicitly paid-tier value  
* Still **not** becoming a log storage/query platform 

If you want a single “killer moment” in the CLI demo: make the *first* `session start` fail due to TTL policy, then succeed with a compliant TTL, and immediately show the targeted debug logs appear only for the affected tenant+route. That lands instantly.

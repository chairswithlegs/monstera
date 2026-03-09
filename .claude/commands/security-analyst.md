# Security Analyst

Analyzes plans and code for security vulnerabilities in REST APIs and ActivityPub systems. Focus on auth, visibility, blocks, and federation safety.

## When to Apply

- Security review of new or changed endpoints, services, or federation logic
- Implementing or changing auth, authorization, visibility, or block checks
- Questions about 404 vs 403, viewer identity, or service-layer enforcement

---

## 1. Authorization and Authentication

- **Auth guards**: Every non-public endpoint must enforce authentication (e.g. require valid token/session). Public endpoints (e.g. public timeline, actor profile, webfinger) may use optional auth when behavior differs for logged-in users.
- **Scope**: Confirm that the authenticated identity is used for authorization decisions (e.g. "can this user perform this action on this resource?"), not only for presence checks.
- **Federation**: Inbox/outbox and actor dereference may be unauthenticated from remote servers; distinguish server-to-server (S2S) requests (HTTP Signatures, actor in request) from client-to-server (C2S) and apply correct auth model.

---

## 2. Visibility (Critical for Status and Timelines)

Visibility must be enforced in the **service layer**, not in the store or raw SQL.

- **Where**: All status read paths — single status, context/thread, favourited_by, reblogged_by, home/public/hashtag/list timelines — must use a service-layer visibility check (e.g. `canViewStatus` / `CanViewStatus`).
- **Viewer identity**: The viewer (who is asking) must be explicit. Handlers derive `viewerID` from request context (or nil when unauthenticated) and pass it into service methods, e.g. `GetByIDEnriched(ctx, id, viewerID)`. Do not infer visibility from store-only logic.
- **Unauthenticated viewers**: Must not see private or direct statuses. Return **404**, not 403 — do not reveal existence of the resource.
- **Placement**: Do not push visibility rules into SQL or store layer; keep them in service logic so all call sites (API, workers, internal) behave consistently.

---

## 3. User Blocks

Blocks must be applied in the **same** service-layer check as visibility.

- **When**: For any "can this viewer see this status?" decision, apply block relationships: if the viewer has blocked the author, or the author has blocked the viewer, the status is not visible.
- **How**: Use store support (e.g. `IsBlockedEitherDirection`) inside the visibility helper so a single code path enforces both visibility and blocks.
- **Consistency**: Single-status read, context/thread, favourited_by, reblogged_by, and list/home timelines must all use this same logic so blocks are respected everywhere. Return 404 when blocked (same as "not visible").

---

## 4. REST API Security

- **IDOR**: Ensure every request that operates on a resource by ID checks that the authenticated user is allowed to access that resource (owner, shared list, visibility, blocks).
- **Input validation**: Validate and sanitize all inputs (IDs, limits, cursor, body). Reject invalid or oversized payloads; use allowlists for enums and formats.
- **Injection**: Use parameterized queries and avoid building SQL or activity payloads from unsanitized user input. Same for JSON-LD and ActivityStreams — validate types and required fields.
- **Sensitive data**: Do not log tokens, passwords, or full request bodies. Redact in error messages and logs.
- **Rate limiting**: Consider rate limits on auth, registration, and expensive or write endpoints to mitigate abuse.

---

## 5. ActivityPub / Federation Security

- **HTTP Signatures**: Inbox and other S2S endpoints must verify HTTP Signatures (Draft Cavage). Reject requests with missing, malformed, or invalid signatures. Use the key from the actor document for the `keyId` in the signature.
- **Inbox authorization**: Only accept activities from senders that are allowed to deliver to the target (e.g. sharedInbox vs inbox, block checks). Validate that the activity's actor matches the signature.
- **Object validation**: Validate incoming ActivityStreams objects (type, id, actor, object). Reject or ignore unknown types and required-field violations to avoid malformed data affecting internal state.
- **Recursion and size**: Limit depth of embedded objects and size of incoming payloads to prevent DoS and resource exhaustion.
- **Redirects and SSRF**: When fetching remote actors or objects, do not follow redirects to private or arbitrary hosts; use allowlists and timeouts.

---

## 6. Review Checklist

When performing a security review:

- [ ] Non-public endpoints have auth guards; public endpoints correctly treat auth as optional where needed.
- [ ] Status read paths enforce visibility in the service layer with an explicit viewer identity (e.g. `viewerID`).
- [ ] Unauthenticated users never see private/direct statuses; response is 404, not 403.
- [ ] Block relationships are applied in the same visibility helper; single-status, context, and timelines all use it.
- [ ] No visibility or block logic only in SQL/store; service layer is the single place for "can view?".
- [ ] IDOR: resource access is gated by ownership/visibility/blocks for the requesting identity.
- [ ] Inputs validated and parameterized; no injection from user-controlled data.
- [ ] Federation: HTTP Signatures verified; inbox authorization and object validation in place; limits on size/depth and SSRF mitigations.

---

## Feedback Format

- **Critical**: Vulnerability or clear violation (e.g. visibility in store only, missing auth, 403 for private status, blocks not applied).
- **High**: Significant weakness (e.g. IDOR risk, missing signature verification, sensitive data in logs).
- **Medium**: Improvement that would harden the design (e.g. rate limiting, stricter validation).
- **Low**: Minor or defensive hardening.

When suggesting fixes, point to the correct layer (e.g. "move visibility check to service; pass viewerID from handler") and align with project conventions (e.g. `api.HandleError`, domain errors, no HTTP in service/store).

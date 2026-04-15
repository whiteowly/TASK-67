# CampusRec — Prompt Clarification Questions

This document lists points in the original CampusRec product prompt that
were genuinely ambiguous and required interpretation. Every entry is a
real ambiguity in the prompt language, **not** a derived design or
implementation choice.

Implementation choices that were *made* (vs. choices that the prompt
*forced*) live in `docs/assumptions.md`.

> **Format:** for each entry — *Prompt text that is ambiguous* → *what
> the ambiguity is* → *which interpretation was adopted, in one
> sentence*. The full implementation rationale lives alongside the
> chosen interpretation in `docs/assumptions.md`.

---

## 1. Session management strategy

**Prompt text:** "Local username/password authentication."

**Ambiguity:** the prompt prescribes credential format and that
authentication is local, but does not say whether sessions are
server-side or stateless tokens.

**Interpretation adopted:** server-side sessions in Postgres, with
immediate revocation on logout.

---

## 2. Monetary value storage representation

**Prompt text:** session/product pricing and WeChat Pay amounts.

**Ambiguity:** the prompt references monetary values but does not
specify the storage type (decimal, integer minor units, float).

**Interpretation adopted:** `BIGINT` minor units (fen) for all monetary
columns.

---

## 3. Account-lockout counter semantics

**Prompt text:** "lockout after 5 failed attempts for 15 minutes."

**Ambiguity:** "5 failed attempts" can mean cumulative lifetime, rolling
window, or a counter that resets on success — each has different
security and UX trade-offs.

**Interpretation adopted:** counter resets on successful login;
`locked_until` is a 15-minute absolute timestamp set when the counter
hits 5.

---

## 4. Concurrency-control mechanism for seat deduction

**Prompt text:** "atomic seat deduction with database transactions to
prevent overselling."

**Ambiguity:** the prompt says "atomic" but does not specify *how* —
SELECT FOR UPDATE, optimistic version, CHECK constraint, or app-level
mutex.

**Interpretation adopted:** DB-level `CHECK (reserved_seats <=
total_seats)` plus `version` column for optimistic locking.

---

## 5. Waitlist promotion when next user is no longer eligible

**Prompt text:** "automatically promote the next user from the waitlist
within 30 seconds when a seat is released."

**Ambiguity:** the prompt does not say what to do if the next person on
the waitlist has been deactivated or banned since joining.

**Interpretation adopted:** skip inactive/banned users automatically and
record the skip reason; recurse until an eligible candidate is found or
the waitlist is empty.

---

## 6. WeChat Pay callback signature scheme

**Prompt text:** "WeChat Pay callback signature verification using
locally stored merchant keys."

**Ambiguity:** the prompt does not specify the signature
algorithm/format, and on a private network the standard public WeChat
protocol may not apply.

**Interpretation adopted:** HMAC-SHA256 over
`{gatewayTxID}|{merchantRef}|{amountMinor}` with a locally stored
shared key.

---

## 7. Payment expiry enforcement: scheduled job vs. lazy

**Prompt text:** "15-minute countdown" and "the order is auto-closed."

**Ambiguity:** the prompt does not say whether the close happens via a
background job or only when the order is next read.

**Interpretation adopted:** scheduled background job (`ExpirePayments`)
so close occurs even if the user never returns to the UI.

---

## 8. No-show cancellation window: global constant vs. per-session

**Prompt text:** "no-show is auto-canceled after 10 minutes past start
time."

**Ambiguity:** the prompt does not say whether 10 minutes is fixed or
overridable.

**Interpretation adopted:** 10 minutes is the default, overridable per
session via `session_policies.noshow_cancel_minutes`.

---

## 9. "Released" semantics when temporary leave expires

**Prompt text:** "one 10-minute break per hour, after which the seat is
released."

**Ambiguity:** "released" is undefined — does it mean cancellation,
flag-and-keep, or something else?

**Interpretation adopted:** flag the leave event as `exceeded` and
raise an `occupancy_exception` for staff review; do not cancel the
registration outright.

---

## 10. SLA "business hours" definition

**Prompt text:** "initial response within 4 business hours, resolution
within 3 calendar days."

**Ambiguity:** "business hours" is not defined for a recreation
facility that may operate 7 days a week, and timezone semantics are
not specified.

**Interpretation adopted:** treat both windows as calendar hours
(`now + 4h`, `now + 72h`) computed in `FACILITY_TIMEZONE`. Full
business-hour calendar handling deferred.

---

## 11. Posting rate limit scope

**Prompt text:** "posting frequency limits (e.g., 5 posts per hour)."

**Ambiguity:** the prompt does not say whether the limit is per
session, per user globally, or per IP.

**Interpretation adopted:** per user globally, regardless of session,
sliding 1-hour window over `posts.created_at`.

---

## 12. Canary cohort assignment determinism

**Prompt text:** "canary release by user cohort percentage (e.g., 10%
of staff accounts)."

**Ambiguity:** the prompt does not say whether a user's cohort
membership is stable across requests.

**Interpretation adopted:** deterministic via
`SHA256({flagKey}:{userID}:{flagVersion}) mod 100 < cohort_percent`;
incrementing `flag_version` re-randomizes when expanding rollout.

---

## 13. Referential integrity after archival

**Prompt text:** "moving closed orders and tickets older than 24 months
into an archive schema while keeping masked lookup fields for
reporting."

**Ambiguity:** the prompt does not say how to keep audit-log and other
foreign keys valid once the source row has moved.

**Interpretation adopted:** keep masked projection rows in the main
schema (`archive_lookup_projection`); move full rows to the archive
schema.

---

## 14. Import dedup level: file vs. row

**Prompt text:** "duplicate detection and file fingerprinting" for
imports.

**Ambiguity:** the prompt does not say whether dedup is at the file
level, the row level, or both.

**Interpretation adopted:** file-level via SHA-256 in `file_artifacts`
to block re-uploads; per-row template-specific dedup belongs to
`ValidateImport`.

---

## 15. Proof-of-delivery image storage on a private network

**Prompt text:** "proof-of-delivery (signature image or typed
acknowledgment)."

**Ambiguity:** with no cloud object storage available, the prompt does
not say where signature images live or how access is restricted.

**Interpretation adopted:** local filesystem under `UPLOAD_DIR`, with
the relative path stored in `delivery_proofs.signature_path`; access
gated by staff+ RBAC.

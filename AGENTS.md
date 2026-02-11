# agents.md — Codex Operating Rules (Tripit / Go Backend)

This repository is built with AI assistance. Codex must follow the rules below when proposing or implementing any change.

---

## 1) Core Principles

1. **Backend quality is non-negotiable**

   * Backend is written in **Go** using the **Gin** framework.
   * Backend changes must be **lint-clean** and **readable**.

2. **Frontend is flexible**

   * Frontend work may follow best effort and does not require strict backend-level standards.

3. **Human readability > cleverness**

   * Prefer simple, explicit code.
   * Keep functions/files small.
   * Create folders/files that map cleanly to responsibilities (routes/modules/etc).

4. **Docs must always match reality**

   * Docs updates are mandatory for every code change.
   * Docs must never become stale.

---

## 2) Git Workflow Requirements

Codex must use Git properly:

1. If the repo is not initialized:

   * Initialize Git
   * Create an initial commit (after adding baseline files)

2. For each feature or fix:

   * Create a branch (unless instructed otherwise)
   * Make small commits with clear messages
   * Do not commit code that fails lint

3. Each completed feature must end with:

   * Lint passing
   * Docs updated
   * A final commit that includes code + doc updates

---

## 3) Linting & Pre-Commit Hook (Go)

Backend Go code must follow `golangci-lint`.

1. Codex should ensure `golangci-lint` is configured and used.
2. Add a Git hook (pre-commit) so commits fail if lint fails.
3. If lint fails:

   * Fix issues
   * Re-run lint until clean
   * Only then commit

---

## 4) Go Code Style Rules (Readability)

### 4.1 Line length

* Keep lines reasonably short (use golangci-lint recommendations / common standards).
* Break long expressions into intermediate variables.

### 4.2 “One assignment before control blocks”

To improve readability:

* Do not stack multiple assignments immediately above `if/for/switch`.
* Prefer **at most one assignment** right before a control statement.
* If more setup is needed, separate it with spacing and/or extract a helper function.

### 4.3 Spacing rules

* Add **one blank line** between logical blocks.
* Avoid dense “wall of code”.

### 4.4 Function size limit

* Functions should be short.
* If a function grows beyond ~10–20 lines, Codex must refactor into smaller helpers.
* Each function should do one thing.

### 4.5 File size limit

* Avoid giant files.
* If a file becomes large or mixes responsibilities:

  * Split into multiple files
  * Possibly split into subfolders/modules
  * Use clear naming conventions

### 4.6 Naming & folder structure

* Prefer folder structure that mirrors the domain:

  * `routes/` or `handlers/` for HTTP entry points
  * `services/` for business logic
  * `stores/` or `repos/` for DB access
  * `payments/` module for payment logic
  * `users/` module for user logic
* If each route maps cleanly to a file, do it.

---

## 5) Performance & Reliability Expectations

This backend will be used by **multiple concurrent users** and includes **payments**.

Codex must:

* Avoid memory-heavy patterns that can cause OOM / “boom killed” scenarios.
* Be careful with in-memory caches, rate limiters, and large allocations.
* Prefer efficient, bounded-memory approaches for:

  * rate limiting
  * request payload processing
  * background jobs
* Consider load and concurrency as default assumptions.

---

## 6) Documentation Rules (Strict)

Docs are required and must be maintained continuously.

### 6.1 Root-level architecture doc

* Maintain a top-level document describing:

  * architecture overview
  * major modules and responsibilities
  * data flow / request flow
  * key decisions and conventions

### 6.2 docs.md per folder

* Every folder must contain a `docs.md` that explains:

  * purpose of the folder
  * what each file does
  * any important logic, contracts, and assumptions
  * how this folder fits into the larger system

### 6.3 How to handle subfolders

* A folder’s `docs.md` may describe subfolders at a high level.
* Detailed explanations live in the subfolder’s own `docs.md`.

### 6.4 Recursive doc updates (mandatory)

If Codex changes code in `path/to/module/...`, it must update:

* `path/to/module/docs.md`
* and every parent folder’s `docs.md` that references that module
* up to the root architecture doc if the change affects overall design

Docs freshness is a top priority.

---

## 7) “Explain the Change” Requirement

For every PR-sized change (or equivalent):

* Provide a clear summary of:

  * what files changed
  * what behavior changed
  * how to verify it
  * what docs were updated

This should be precise enough that a human can review quickly.

---

## 8) Non-Negotiables Checklist (Backend)

Before declaring any backend task “done”, Codex must confirm:

* [ ] golangci-lint passes with zero issues
* [ ] Pre-commit hook exists and enforces lint (or was verified)
* [ ] Code is readable (small functions, sensible modules)
* [ ] docs.md updated in the touched folder
* [ ] docs.md updated recursively upward (parents + architecture if needed)
* [ ] Git commits are clean and meaningful

---

## 9) Function Signature & Naming Standards (Go Backend)

These rules apply to all backend Go code.

### 9.1 Parameter limit (max 2)

* A function should take **at most 2 parameters**.
* If more than 2 inputs are required, define a **request struct** (or option struct) and pass that instead.
* Prefer passing `context.Context` as the first parameter for request-scoped operations.

Examples:

* ✅ `func CreateUser(ctx context.Context, createUserInput CreateUserInput) (*User, error)`
* ❌ `func CreateUser(ctx context.Context, name, email string, age int, planID string) (*User, error)`

### 9.2 Return value limit (max 2)

* A function should return **at most 2 values**.
* Preferred pattern: `(<value>, error)` or `error` only.
* No functions should return 3+ values.
* If multiple outputs are needed, wrap them in a **result struct**.

Examples:

* ✅ `func ParseToken(token string) (*Claims, error)`
* ❌ `func ParseToken(token string) (userID string, orgID string, exp time.Time, err error)`

### 9.3 Single responsibility (mandatory)

* Each function must do **exactly one thing**.
* If a function starts doing multiple steps (validate + transform + persist + notify, etc.):

  * split into helper functions
  * name helpers clearly by responsibility

### 9.4 Function names must describe behavior completely

* A function name must clearly match what it does.
* If it performs extra side effects, the name must reflect them, or the side effects must be extracted.

---

## 10) Variable Naming Standards (Go Backend)

### 10.1 No ambiguous names

* Avoid names like: `data`, `temp`, `thing`, `val`, `obj`, `input`, `output`.
* Names must make sense without needing to read surrounding code.

### 10.2 Input/output suffix rules (only when it adds clarity)

* Use meaningful suffixes only when it improves clarity.
* Avoid redundant or meaningless suffix repetition.

### 10.3 Prefer domain-specific names

Examples:

* `userEmail`
* `paymentCustomerID`
* `subscriptionPlan`
* `rateLimitKey`
* `itineraryCreateRequest`
* `itineraryCreateResponse`

---

## 11) Logging Standards (Mandatory)

### 11.1 Single logger interface

* Do not use `fmt.Println`, `log.Println`, or ad-hoc prints.
* Always log through the project logger.

### 11.2 Output targets

* Logger must write to:

  1. stdout
  2. a log file

### 11.3 Structured logging

* Prefer structured fields over string concatenation.
* Include request_id, route, user_id (when safe), error fields when relevant.

### 11.4 Logging levels

* Debug
* Info
* Warn
* Error

### 11.5 No sensitive data

* Never log secrets.
* Avoid full request bodies unless scrubbed.

### 11.6 Logs must not affect correctness

* Logging should not be part of functional logic.

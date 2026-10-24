# Knotenpunkt — five attempts, one pattern

What each version tried, how it was built, why it stalled, and what it did differently. Read this before you start a sixth. This file is self-contained: it assumes no access to the old repos.

Sources (on nogobook): `~/Development/archive/20251029_knotenpunkt` (v0), `~/Development/archive/20260408_knotenpunkt/{alpha,beta,delta,testruns,experiments,Knotenpunkt.xlsx}`, `synapse-garage/projects/knotenpunkt` (gamma), `synapse-garage/experiments/exp-002-repo-nav`, `exp-003-agentic-nav`.

## Terms

- **Knotenpunkt** (German: "junction"). A local tool that indexes a code repository and answers questions about it, for a human or an AI agent.
- **MCP** (Model Context Protocol). The protocol that lets Claude Code or Cursor call a tool server. Every version except v0 used MCP as its main interface.
- **Graph.** Nodes (functions, files, docs, people) and typed edges (calls, imports, authored_by). Every version made this graph the core idea.
- **Semantic search.** Find text by meaning with vector embeddings, not by exact words.
- **Forge / WU.** forge is the owner's script that runs one Claude session per Work Unit (WU). A WU is a small spec with a verify step. Beta and gamma were built WU by WU.
- **Oracle.** A test outside the system that decides "good enough". Example: a frozen question set with known answers. This is the rule of synapse-garage: no oracle, no project.
- **wire.** The owner's local OpenAI-compatible inference service. Gamma used it as its LLM backend.

## Timeline

| Version | Dates | Size | One-line frame |
|---|---|---|---|
| v0 | 2025-10-27 → 10-29 | 1 commit, 17k lines Go + React UI | "Developer knowledge assistant" for teams of 5–7 |
| alpha | 2025-10-29 → 2026-01-27 | 357 commits, 84k lines Go | "Code knowledge assistant": call graph, impact, MCP |
| beta | continues alpha's git history → 2026-03-30 | 608 commits (alpha's 357 included), 72k lines Go | "Project context graph": code + docs + people; Symfony pilot |
| delta | 2026-03 | docs only, 1,200 lines | Gamma's design, plus an adversarial review of it |
| gamma | 2026-03-30 → 2026-04-15 | 154 commits, 55k lines Go, ~75 WUs + Horizon 2 | "Project knowledge system": 3 layers, a graph that learns from use |

Not versions: `membrane/` is a third-party repo (Bennett Schwartz, GustyCube/membrane), an episodic memory layer for agents. Beta studied it for ideas (decay, revision, trust gating). `experiments/adaptive-lora` is a beta-era experiment, covered under beta.

---

## v0 — the platform (Oct 2025)

**Goal.** "Transform code repositories into queryable knowledge graphs that give expert-level answers in 1–3 minutes." The users were rotating teams of 5–7 developers, plus managers who wanted "visibility into technical progress". It had to be on-premise and GDPR-safe.

**How it was built.**
- Stack: Go + Gin, PostgreSQL + pgvector, Redis cache, Ollama (`nomic-embed-text`, 768-dim), LangChainGo, React + Tailwind + shadcn. Five docker-compose services (db, api, workers, mcp, ollama).
- A "modular monolith" with six services: Project (projects, repos, uploaded docs, labels), Analysis (Go AST + tree-sitter for 7 languages), Knowledge (chunks + embeddings), Search, Chat, Quality (planned).
- Data model: project → repositories, documents, labels. Code entities + dependencies in SQL tables. Background job queue in Postgres.
- Built in about three days. `docs/generated/` already holds memory-leak analyses and fix summaries.

**Why it failed.** It built a multi-tenant product for an organisation that did not exist yet. All the effort went into infrastructure (queue, upload, labels, UI). Nothing measured whether an answer was good.

**What it did differently.** It was the only version with a web UI and a manager audience. It was the only one built on Postgres-first.

---

## alpha — the code-intelligence tool (Oct 2025 → Jan 2026)

**Goal.** Code intelligence for developers and AI agents: "What calls this function?", "If I change X, what breaks?", "Who knows this module?"

**How it was built.**
- One local Go binary with a daemon, a CLI, a REST API, MCP (primary) and ACP (Zed editor, stdio JSON-RPC).
- Storage in two tiers: SQLite + chromem-go (embedded vectors) locally, and PostgreSQL + pgvector for teams.
- **Federation:** a local instance could connect to several upstream team instances over REST for cross-codebase queries.
- The daemon managed `llama-server` processes itself (e.g. Qwen2.5-Coder-14B) or used Ollama.
- Parsers: Go AST + tree-sitter (TS, JS, Python, Rust, PHP, CSS, HTML). Regex doc parsers for 7 formats (Markdown, Org, RST, AsciiDoc, MediaWiki, LaTeX, Typst).
- 11 MCP tools: `info`, `find`, `analyze`, `graph` (callers, callees, path, implementations, doc links), `impact`, `search` (semantic, keyword, hybrid), `experts`, `contributors`, `architecture` (packages, dependencies, cycles, imports), `dead_code`, `ask`.
- Process: `.work/` folders (active, backlog, done, issues), a daily CHANGELOG, DDD refactors with a `layering_test.go` that checks package boundaries.

**Evidence it produced.** On 2026-01-22, Claude rated 28 tool calls on knotenpunkt's own repo (272 files, 5,004 entities) from 1 to 5:
- `search`, `find`, `analyze`, `experts`, `dead_code`: 5/5.
- `impact` 2, `architecture dependencies` 2, `graph path` 3, doc links/backlinks/tags 2 (0 results).

The plain lookup and search tools worked. **The graph-specific tools scored worst.** That was the first signal against the graph premise, and it went unread.

**Why it failed.**
- Breadth without a judge: two storage tiers, federation, 7 doc formats, and process management — for one user.
- The only evaluation was self-judgment. An LLM rated the tool on the tool's own repo, and a "test suite" asked knotenpunkt to grade its own DDD/SOLID structure.

**What it did differently.** The widest scope of any version, and the first one to use MCP and a single binary.

---

## beta — the context graph and the pilot (Feb → Mar 2026)

**Goal.** Widen from code to "everything a project produces": code, docs, templates, people, decisions. "Turn 'I have no idea' into 'I know enough to ask the right question to the right person.'" Then prove it with a **pilot customer: a Symfony/PHP repo**.

**How it was built.**
- Same git history as alpha, restructured into `core/` (domain + ports) with adapters around it.
- Milestones M0–M19 in February, steered by `.work/ORCHESTRATOR.md` and `DISCOVERY.md` (an AI-led planning script that cites the MCP tools as evidence). Then forge WUs 01–45 in March.
- Key milestones:
  - M8 self-hosting: knotenpunkt indexes itself.
  - M4 "graph-native search": FTS5 over summaries + content + vectors, ranked by graph centrality (how many edges point at a node).
  - M12–M14 summaries. First an LLM wrote a summary per symbol at index time. This failed on a 1.7B / 4k-context model. Then summaries were built from graph data (docstring, signature, callers) with no LLM. The rule written then: **intelligence belongs at query time, not in the write path.**
  - M15 tree-sitter for PHP, Twig, TS, JS, HTML, CSS with Symfony cross-language edges (controller → template, template → PHP getter, HTML → CSS class).
  - M16 OpenAI-compatible `/v1/chat/completions` (for Open WebUI).
  - M18 "agentic ask": the LLM calls tools in a loop (max 5 rounds) before it answers.
- Model bench: 3 chat models (glm-4.7-flash, gpt-oss-20b, qwen3-coder) on 7 questions. The `expected` field is null, so nothing was scored.

**Evidence it produced.**
- **The pilot was planned but never assessed.** The roadmap's M17 was "Pilot value assessment: measure outcomes with and without knotenpunkt". The M17 that shipped was "client-server split" instead.
- **A real oracle was drafted (2026-03-05).** `Knotenpunkt.xlsx` holds 13 domain questions with reference answers (e.g. "When can a participant start an online exam?"). It also holds 6 frontend questions with model answers and four criteria: correctness, completeness, clarity, consistency. Only 2 answers were scored, and both got 0 on every criterion. The 13 reference questions were never run.
- The pilot answers (`testruns/`) show the failure modes: the agent loops on `find "*"`, prints raw `<tools>` JSON as its answer, and says "I cannot answer" to "what is the most painful part of the codebase?".
- **LoRA experiment (Feb 2026), verdict FAILURE.** Question: does a 1.5B model with project-trained LoRA adapters write better code summaries? Training: 1,374 pairs over 11 domains. Result on 34 compared pairs: BERTScore −0.01, structural accuracy +0.0, output format lost in 20 cases, +3.8 s latency.

**Why it failed.**
- The vision grew while the evidence shrank. WUs 33–45 added node kinds, edge types, "concept" nodes and an artifact-ingest API. The graph got larger; the answers did not get better.
- The one real oracle (the xlsx) was written and then not used. The pilot, the only real user, was never measured.

**What it did differently.** The only version with a real external user and a real question set. It also produced the first honest negative results.

---

## delta — design only (Mar 2026)

**Goal.** Design gamma cleanly before writing any code.

**What it contains.** Two docs: `architecture.md` (813 lines, "Knotenpunkt Gamma — Intent, Architecture & Roadmap") and `architecture-evolution.md` (392 lines, an adversarial review). Gamma's `project.md` and `architecture.md` come from these.

The review found real problems:
- "analysis/ is not core": it imports tree-sitter and goldmark.
- "6 of 19 edge types are only planned" — and those 6 are the edges the value proposition depends on.
- "The enrichment pipeline is open-loop. Nothing measures whether any stage produced good output." It proposed quality signals (e.g. a concept with 1 link is noise, one with more than 50 is too broad).
- "The agentic loop is the intelligence ceiling": answer quality depends on the loop more than on the graph.

**Why it failed.** It corrected the *design* and kept the *premise*. Nobody asked whether a graph beats plain retrieval, and the review did not demand an answer-quality test.

**What it did differently.** No code at all — a deliberate pause to think.

---

## gamma — the planned system (Mar 30 → Apr 15, 2026)

**Goal.** "Make project knowledge durable, queryable, and compounding" in three layers:
1. Structure: a deterministic graph with no LLM.
2. Understanding: LLM "purpose" sentences per node, concept extraction, embeddings.
3. Compounding: the graph learns from query logs and background routines.

**How it was built.**
- Fresh repo. Go 1.26, CGO (tree-sitter, sqlite-vec), SQLite only, one database per project context.
- Packages: `core/{graph,domain,port,traverse,compute,briefing}` (stdlib only, checked by a boundary test), `analysis` (parsers: Go, TS, PHP, Python, Rust, Bash, Twig, HTML, CSS, Markdown), `ingest`, `storage`, `query` (hybrid FTS5 + vector with RRF fusion), `llm/{enrich,embed,synthesize}`, `tools`, `adapter/{mcp,http}`.
- Graph model fixed up front: 10 node kinds (Symbol, Document, Section, File, Tag, Contributor, Concept, Decision, Conversation, Artifact) and 19 edge types. IDs are kind-prefixed qualified names (`symbol:pkg.Func`, `doc:docs/README.md`).
- MCP tools: `info`, `find`, `search`, `analyze`, `graph`, `impact`, `architecture`, `dead_code`, `experts`, `knowledge`, `read_code`, `ask`.
- Two modes: "structural" (no LLM) and "full" (LLM via wire). It runs as a systemd user service.
- Process: a frozen roadmap of 44 WUs in 8 phases, then Horizon 2 (H2-01 … H2-46), all run by the forge runner. `project.md` even specifies a 7-agent team (lead, core, parser, storage, ingest, intelligence, adapter).
- WU 12 was a validation spike that gated the rest.
- `cmd/bench` is a model-selection bench: Bayesian scores for purpose, concept, chat and tool-call fixtures. It picks an LLM. It does not test answers to real questions.

**Evidence it produced.**
- WU 12 checked *the data model*: IDs stay stable, JSON round-trips, concepts span 2 artifact types, FTS5 returns results for 3 queries. It did not check *answer quality*. It passed, so the build continued.
- Data audit at Horizon 2 (2026-04-02):
  - The query log was empty: "no queries have been made yet".
  - 27% of concept names were polluted by a bug, and 0% had purposes.
  - There was 1 `implements` edge in total.
- Its own `docs/progress/architectural-reflection.md`: the graph is **descriptive, not interpretive**. It can say "X has 47 dependents", not "why X exists". The most valuable knowledge (rationale, gotchas, tacit knowledge) has no node.

**Why it failed.** The most disciplined build — aimed at an unvalidated premise. All 10 node kinds and 19 edge types existed before one real query. The gates measured internal consistency, never usefulness.

**What it did differently.** Strict boundaries, Pike's rules, a frozen plan, a spike gate, and an evidence gate for Horizon 2 ("≥100 logged queries before we build learning routines"). That evidence gate was the right idea; it was never reached because nobody used the tool.

**What is reusable.** The parsers, the SQLite schema, the hybrid search, and the MCP adapter are the most mature code of all versions.

---

## After gamma — the premise was finally tested (June 2026)

- **exp-002** (2026-06-22): 15 frozen German questions over a Symfony PHP repo (a different repo from beta's pilot), with exact code identifiers as answers. Single model (Qwen3.6-27B via wire), substring scorer.
  - Semantic retrieval with `nomic-embed-text-v2-moe`: **86%** (multi-hop 87%).
  - ripgrep: 40%. ctags structure: 40%. Cheap ctags graph: **26%, the worst and most expensive**.
  - The embedder choice was the strongest lever, bigger than the choice of method.
  - Caveat: a cheap ctags graph, not gamma's tree-sitter graph. One repo, one model, 15 questions.
- **exp-003:** an agent loop (semantic search + file reads) scored 87%, a tie with single-shot (86%), at about 10× the tokens. Grep + read in the same loop scored 73%. The index earns its place; the loop does not.

The first real oracle took about one day and decided the question that five versions had not asked.

## The pattern across all five

1. **No oracle was ever run.** Beta drafted one (the xlsx, 13 reference answers) and did not use it. Every other "evaluation" was the tool or an LLM grading itself.
2. **The premise was never at risk.** Each version re-planned *how* to build the graph, never *whether* a graph helps. Alpha's own tool ratings already showed the graph tools scoring worst.
3. **The frame grew each time:** code → project → knowledge → compounding knowledge. A vision that justifies any feature cannot fail a test.
4. **Infrastructure before use.** Postgres, federation, process management, a 7-agent team plan and learning routines all came before real queries. Gamma's query log was empty.
5. **The real user was dropped.** The Symfony pilot was the only outside user. Its value assessment (M17) was replaced by an infrastructure milestone.
6. **The good lessons came from small failures:** intelligence at query time (beta M13/M14), LoRA does not help (beta), the pipeline is open-loop (delta), descriptive ≠ interpretive (gamma). All of them came late.

## For the next version

- Start from exp-002's oracle and its winner (semantic search + `nomic-embed-text-v2-moe`). The question set is the spec.
- Add one thing at a time — a graph, docs, git history. Each addition must raise the score on the frozen questions, or it goes.
- If you want the graph back, test gamma's tree-sitter graph against exp-002 first (the untested "V4").
- Pick one user and one repo: the owner, on a repo they work in this week. Count the real questions asked.
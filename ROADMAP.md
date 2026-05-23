# Roadmap
 
This document outlines the planned development of `inframap-simulacrum` across
four phases, from core seeding infrastructure through to full environment
simulation and LLM-assisted tooling.
 
---
 
## Phase 1 — Core Seeder
 
The foundation. Establishes data generation, encryption, and the basic TUI shell.
 
- [ ] Schema and dictionary definitions (open, versionable)
- [ ] Seed generation with per-environment salt
- [ ] Support for database records, config, and environment state
- [ ] Encrypted output storage (generated seeds never exposed)
- [ ] Basic TUI — launch, status, teardown
- [ ] `simulacrum up` / `simulacrum down` commands
- [ ] MIT license, contributing guide, CI setup
---
 
## Phase 2 — Canary Layer
 
Introduces honeypot infrastructure for passive intrusion detection.
 
- [ ] Canary record generation alongside real seed data
- [ ] Append-only access log for canary records
- [ ] Alert triggers on canary record access
- [ ] Canary config (density, record types, placement strategy)
- [ ] Separation of canary state from general seed output
- [ ] Documentation on canary usage in staging vs prod-adjacent environments
---
 
## Phase 3 — Scenario Engine
 
Turns the seeder into a full environment simulator with opt-in behavioural scenarios.
 
- [ ] Default scenario — realistic, passive, always-on
- [ ] YAML-based scenario definitions (`scenarios/`)
- [ ] `simulacrum run --scenario <name>` command
- [ ] Built-in scenarios: spike traffic, new user onboarding, payment failure, error injection
- [ ] Scenario composition (chain or combine scenarios)
- [ ] Scenario diffing — what changed between runs
- [ ] TUI scenario browser and selector
---
 
## Phase 4 — Walker + LLM Behaviour Generation
 
The walker statically analyses the project to build context for the behaviour
engine. The LLM uses that context to generate realistic, scenario-specific
behaviour tests.
 
- [ ] Walker core — scans project source to extract DB calls and API endpoints
- [ ] `simulacrum walk` command
- [ ] Saves walker output as structured context (schema, call graph, endpoint map)
- [ ] Context passed to behaviour engine as grounding for simulation
- [ ] LLM integration — uses walker context + system prompt to generate behaviour tests
- [ ] Scenario generation via natural language ("describe what you want to test")
- [ ] TUI inline help (`?`) — context-aware, screen-sensitive
- [ ] Onboarding mode — walker output surfaced to new developers as environment overview
---
 
## Future Considerations
 
- GitHub Actions integration for scenario runs in CI
- inframap native integration (topology-aware seeding)
- Multi-environment support (seed profiles per environment)
- Seed versioning and rollback
 

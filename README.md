# inframap-simulacrum

An addition to the inframap family, decoupled in order to ensure portability
across services, executable on demand or tailored to specific applications you build.

## Overview

`inframap-simulacrum` is a developer environment runtime — it seeds, simulates,
and narrates. It goes beyond static fixture tooling by combining data generation
with scenario-driven behavioural simulation, making it useful for onboarding,
integration testing, and staging validation.

## Features

- **Seeder** — generates and populates data across database records, config, and
  environment state. Output is salted and never exposed; seed schemas remain
  open and versionable.
- **Scenario engine** — drives behaviour on top of seeded data. Default behaviour
  is realistic but passive; specific scenarios are opt-in and scriptable via config.
- **Canary layer** — honeypot records are seeded alongside real data. Any access
  to canary records triggers an alert, enabling passive intrusion detection in
  staging and production-adjacent environments.
- **Walker** — an LLM-assisted context engine that inspects environment state,
  narrates DB content in plain English, and assists with seed and scenario
  generation via a structured system prompt.

## Usage

Simulacrum is operated via a TUI. On launch, the default scenario is active —
a realistic but inert environment ready for exploration.

```bash
simulacrum up
```

To run a specific scenario:

```bash
simulacrum run --scenario payment-failure
```

To inspect current environment state via the walker:

```bash
simulacrum walk
```

## Scenarios

Scenarios live in `scenarios/` as YAML files and are fully version-controllable.

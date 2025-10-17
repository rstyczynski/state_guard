# Finite State Machine

## Goal

Go based FMS that process state change following definition

FMS is defined in yaml file covering:

1. states

2. state transitions

This is simplistic FSM implementation.

Execute only phase 1. Other phases will be done on request, following the plan:

## Phase 1. CLI

## Phase 2. Interactive mode

## Exemplary FSM definition

```yaml
version: 1
name: generic_lifecycle
initial: CREATED
final:
  - TERMINATED

states:
  - CREATED
  - STARTING
  - RUNNING
  - STOPPING
  - STOPPED
  - TERMINATING
  - TERMINATED
  - MAINTENANCE
  - FAILED

transitions:
  # Main sequence
  - from: CREATED
    to: STARTING

  - from: STARTING
    to: RUNNING

  - from: RUNNING
    to: STOPPING

  - from: STOPPING
    to: STOPPED

  - from: STOPPED
    to: TERMINATING

  - from: TERMINATING
    to: TERMINATED

  # Maintenance bidirectional transitions
  - from: RUNNING
    to: MAINTENANCE

  - from: MAINTENANCE
    to: RUNNING

  # Dotted backward transitions
  - from: STOPPED
    to: STARTING

  - from: FAILED
    to: STARTING

  # ANY → FAILED
  - from: "*"
    to: FAILED
```

# Finite State Machine

## Goal

Ansible based FMS that process state change following definition

FMS is defined in yaml file covering:
1. states
2. state transitions

This is simplistic FSM implementation.

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

  - from: TERMINATED
    to: STARTING

  # ANY → FAILED
  - from: "*"
    to: FAILED
```

## Ansible hints

### Generic

Always use FQDN module names

Presenting JSON data via debug.msg always use human readable json formatter



### Roles

In roles always use prefixed variables and facts with role name

Do not use 'roles' syntax

Do not use 'import role' syntax

Use only 'include_role' syntax

## Do not

1. do not use tags

2. do not add anything that you were not directly asked to.

3. NEVER update this READMe file.

Any information presented to operator like final status must be yaml. Use ANSIBLE_STDOUT_CALLBACK=debug

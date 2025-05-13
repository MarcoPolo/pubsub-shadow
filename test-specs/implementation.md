# Specification for running an Implementation in this GossipSub interop tester

This document specifies the requirements that a GossipSub implementation must
fulfill in order to be testable.

## Input

### ExperimentParams

todo

### ScriptAction

see `script_action.py` for the kinds of actions you should support.

## Output

Implementations MUST reserve STDOUT as their output channel and use STDERR for
diagnostics and errors.


Implementations MUST log their STDOUT events using a structured JSON logging format.

All STDOUT logs must include the following fields:
- time: The RFC3339 timestamp of the log entry.
- msg: The message being logged.

Implementations MUST log at least the following events:

- Received message. This event should be logged with the following additional fields
  - from: The peer ID of the sender (not the original publisher).
  - topic: The topic of the message.
  - id: The message id of the message.

  Example:

  New lines added for readability, implementations MUST NOT add new new lines within a JSON object.
  ```json
  {
    "time": "1999-12-31T16:08:12.4030048-08:00",
    "level": "INFO",
    "msg": "Received Message",
    "service": "gossipsub",
    "from": "12D3KooWJTwhQuDG8K7z5rXpB2n9VtFY5wevFWEj9s4HUWt5nvgj",
    "topic": "foobar",
    "id": "31"
  }
  ```

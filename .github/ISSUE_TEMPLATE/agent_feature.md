---
name: Propose AI Agent Feature or Tool
about: Propose new agentic capabilities, LLM integrations, or custom tools for the lazyinfra AI copilot.
title: 'feat(agent): '
labels: 'enhancement, ai-agent'
assignees: ''

---

## Describe the Proposed Feature
Provide a clear and concise description of the new AI agent capability you want to introduce (e.g. "Auto-diagnosing SQS message processing queues").

## Problem / Use Case
Describe the friction point or daily manual AWS infrastructure task this is solving. How will this help reduce developer effort?

## Proposed TUI Flow & Keyboard Shortcuts
Describe how the user will interact with this feature within the lazyinfra Bubble Tea interface:
- Which screen does this live in? (e.g., Lambda view, API Gateway view, CloudWatch logs tail)
- What keyboard shortcut triggers it? (e.g. pressing `a` on an error line, `p` for payload generation)
- How should the result be rendered? (e.g. split view, modal overlay, inline status)

## Required Agent Tools & AWS APIs
What underlying AWS SDK methods or workspace directories does the AI Agent need access to?
- Example: `sqs.ReceiveMessage`, `sqs.GetQueueAttributes`, reading `.go` files in current dir.

## Additional Context
Add any other screenshots, mockups, or suggestions about configuration keys (`.lazyinfra.yaml`).

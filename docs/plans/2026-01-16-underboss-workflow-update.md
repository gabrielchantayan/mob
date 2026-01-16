# Underboss Workflow Update Design
Date: 2026-01-16
Topic: underboss-workflow-update

## Overview

Update the Underboss agent's mental model and workflow to reflect system constraints regarding agent communication. Specifically, the Underboss must understand that subordinate agents (Soldati and Associates) cannot report back directly to him, and he must actively monitor work via Bead status.

## Requirements

1.  **No Direct Reporting**: Underboss must know agents cannot report back.
2.  **Active Monitoring**: Underboss must check beads for completion.
3.  **Underboss Plans**: Underboss must perform planning and exploration himself.
4.  **No Delegation of Planning**: Underboss cannot dispatch agents to plan/explore as they cannot report findings.

## Design

### Prompt Modifications (`internal/underboss/prompts.go`)

The `DefaultSystemPrompt` for the Underboss will be updated to include a "CRITICAL WORKFLOW CONSTRAINTS" section.

**Key Additions:**
*   Explicit statement that Soldati/Associates cannot report back.
*   Instruction to use `get_bead` or `list_beads` to check status.
*   Prohibition on delegating research tasks.
*   Mandate for the Underboss to use `grep`, `glob`, `read` tools personally for planning.

### Workflow

1.  **User Request**: User asks Underboss to implement a feature.
2.  **Exploration**: Underboss uses tools to research codebase.
3.  **Planning**: Underboss creates a plan and breaks it into Beads.
4.  **Delegation**: Underboss spawns agents and assigns Beads (for implementation only).
5.  **Monitoring**: Underboss periodically checks bead status to confirm completion.

## Implementation

*   Modify `internal/underboss/prompts.go`.
*   Update `DefaultSystemPrompt` string.

## Verification

*   Build the `internal/underboss` package.
*   (Manual) Observe Underboss behavior in future interactions to ensure he explores before delegating and checks bead status.

---
name: "have-a-meeting"
description: "Coordinate multi-agent meetings using the huddle-cli tool"
version: "1.0.0"
---

# Agent Huddle (Meeting Room) CLI Skills

This document defines the operational specifications and workflows for conducting multi-agent collaborative meetings using the `huddle-cli` command-line tool.

## 0. Prerequisites (Installation)

Before using this skill, ensure that `huddle-cli` is installed in your PATH. You can install it quickly using the following command:

```bash
curl -fsSL https://raw.githubusercontent.com/radiumce/agent-huddle/main/install-cli.sh | bash
```

After installation, you **MUST** run the `huddle-cli` command by itself to verify the server connection status:
```bash
huddle-cli
```
If the status shows "Unreachable" or if it fails to connect to the default local address, **you must ask the user for the correct server URL**. Once you obtain the correct URL from the user, configure it by running:
```bash
huddle-cli --server <server_url>
```
Do not proceed to use any other `huddle-cli` commands until the server is confirmed to be "Running".

## 1. Role Definition and Initialization (Initialization)

Before using the tool, clearly define whether the current Agent's role is **Host** or **Participant**.

### Role A: Host
Responsible for creating the meeting room and setting the tone for the discussion.

* **Workflow**: Run the `create` command.
* **Command Syntax**: 
  `huddle-cli create --room-id "<unique_room_id>" --host "<host_alias>" --init-message "<init_message>"`
* **Required Parameters**:
    * `--room-id`: (String) A unique ID used to identify the room.
    * `--host`: (String) The display nickname of the host.
    * `--init-message`: (String) **Critical Field**. Must contain the work instructions to be discussed, output requirements, or the complete initial discussion topic.
* **Subsequent Behavior**: After successful creation, choose to enter **Standard Discussion** or **Concurrent Posting** mode based on the meeting flow.

### Role B: Participant
Responsible for participating in reviews or discussions.

* **Workflow**:
    1.  **Discovery & Join**: Run `huddle-cli list` to see active rooms, or use a known ID.
    2.  **Get Context**: **Must** call `huddle-cli context --room-id "<room_id>" --member "<participant_alias>"` to pull history records.
    3.  **Identity Setup**: Set your `--member` or `--sender` alias consistently in subsequent commands.

## 2. Interaction Modes (Interaction Modes)

Choose one of the following two modes for interaction based on the meeting stage.

### Mode A: Concurrent Brainstorming (Independent Viewpoint Posting)
**Applicable Scenarios**: Initial round of speaking at the start of a meeting, voting, or when independent expert opinions are needed from all parties (without referring to others) before aggregation.

* **Core Logic**: Forcefully submit a viewpoint. Even if others speak during the process, do not interrupt, but immediately retrieve others' viewpoints after submission.
* **Command Syntax**: 
  `huddle-cli post --force --room-id "<room_id>" --sender "<your_alias>" --content "<your_viewpoint>" --last-id <last_seen_id>`
* **Required Parameters**:
    * `--last-id`: (Number) The last message ID you saw before thinking.
    * `--content`: (String) Your independent viewpoint.
* **Execution Flow**:
    1.  **Think**: Generate a viewpoint based on the old `last-id`.
    2.  **Submit**: Execute the `post --force` command.
    3.  **Result Handling (Critical)**:
        * **If Pre-existing Messages are returned**: This indicates that other Agents also posted viewpoints while you were thinking/submitting. **Immediately read these messages** from the console output as reference material and prepare for the next round of discussion.
        * **If entered wait/timeout**: This indicates you are the current unique speaker; continue to wait for subsequent replies.

### Mode B: Standard Sequential Discussion
**Applicable Scenarios**: Mutual debate, replying based on the previous message, consensus reaching stage.

* **Core Logic**: Strict linear consistency. If someone interrupts while I am thinking, I must discard my reply and listen to them first.
* **Command Syntax**: 
  `huddle-cli post --room-id "<room_id>" --sender "<your_alias>" --content "<your_viewpoint>" --last-id <last_seen_id>`
* **Execution Flow (State Machine)**:
    1.  **Lock**: Based on the latest `message_id`.
    2.  **Attempt**: Run the `post` command without `--force`.
    3.  **Conflict Detection**:
        * If the tool errors/prompts an update conflict: **STOP & RE-READ** using `huddle-cli context`. Read the new message, regenerate the reply, and try again.
    4.  **Wait**: Call `huddle-cli wait --room-id "<room_id>" --member "<your_alias>" --last-id <new_id>` after sending successfully if you want to explicitly wait to hear back.

## 3. Meeting Termination (Termination)

* **Trigger Condition**: Only when the Host and Participant reach a consensus (Consensus Reached). 
  * **CRITICAL RULE**: Participants MUST NOT leave or close the meeting autonomously. Participants must explicitly obtain the Host's confirmation before terminating their participation. Only the Host has the authority to declare the meeting concluded.
* **Workflow**: The **Host** runs `huddle-cli close --room-id "<room_id>"`.



## 4. Best Practices Summary

> **When to use Force Post (`post --force`)**: When you need to "speak first, then listen to what others say" (blind betting/independent review).
> **When to use Standard Post (`post`)**: When you need to "listen to others finish, then I reply" (linear dialogue).
> **Context Management**: Regardless of which mode is used, pay close attention to `--last-id` to ensure context coherence.
> **Leaving a Room (Important)**: When you are no longer participating in the meeting discussion, you MUST run `huddle-cli leave --room-id "<room_id>" --member "<your_alias>"` to ensure the room's active member list is accurately updated.

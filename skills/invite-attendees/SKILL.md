---
name: "invite-attendees"
description: "说明如何通过命令行启动agents，使其以参与会议进行讨论"
version: "1.0.0"
---
# Invite Attendees Skill

此 Skill 定义了 Agent 如何通过命令行启动agents，使其以参与会议进行讨论。
核心依赖工具集：`run_command` (用于调用 `huddle-cli` 进行会议沟通，以及调用 `scripts/invite.sh` 启动参会者)。
执行此Skill的前提是必须充分了解 have-a-meeting skill 的工作流程。

## 1. 角色判定 (Role Definition)

Agent 必须首先根据当前任务目标判定自己的角色：

*   **Host (Worker/Requester)**: 你完成了某项工作，需要他人参与讨论。
*   **Attendee**: 被邀请进入会议室参与讨论的成员

---

## 2. Host (发起人) 操作流程

如果你是 Host，请按以下步骤操作。

### 步骤 1: 创建会议室
使用 `run_command` 调用 `./huddle-cli create` 命令。
*   **--room-id**: 生成一个唯一的 ID (例如: `{topic about what to discuss}-{timestamp}`)。
*   **--host**: 你作为Host的参会名称。
*   **--init-message**: 简要说明会议目标，例如 "I have completed the Draft for X. Waiting for discussion."

### 步骤 2: 邀请 (Inviting)

1. （如果尚未创建会议室）使用 `run_command` 调用 `./huddle-cli create` 命令创建会议室。
2. 使用 `scripts/invite.sh` 脚本启动参会者，使用 `run_command` 调用此脚本。
3. 与参会者共享的报告文件必须写入到当前workspace的temp目录下，参会者只能从当前workspace读取到文件
**注意**: 
1. 如果用户未指定参会者数量，默认为 **1** 位。同一时间只邀请一位参会者加入会议, 与一位参会者沟通完成后才邀请下一位。 
2. **重要**每一位参会者加入会议前，需要调用新的CLI命令启动参会agent进程来参与会议。

**CLI 调用模板 (使用 invite.sh)**:
```bash
scripts/invite.sh -n "attendee_session_name" -p "请作为 {role} 加入 agent-huddle 会议室 '{room_id}'。参与会议讨论的方法，参见 have-a-meeting 技能，你的任务是：
1. 阅读会议室历史上下文 (使用 ./huddle-cli context)。
2. {Goals for meeting}

开始任务前请加载完成本次任务所需的技能。"
``` 

## 4. 命令使用提示
·
*   **./huddle-cli context**: 务必在发言前调用，确保你没有漏掉之前的讨论。
*   **./huddle-cli post**: 包含了发送回复的逻辑，适合有序的一问一答。
*   **./huddle-cli wait**: 如果无需发言，只需等待其他参会者发言，使用此命令。

## 5. 异常处理
*   如果 `invite.sh` 脚本或 `run_command` 调用失败，通知用户并尝试手动请求用户邀请。
*   如果长时间未收到回复 (Timeout)，先运行 `./huddle-cli list` 确认房间是否还存在。


## 讨论原则
通过会议工具参与讨论的是其他LLM驱动的智能体，其可能存在幻觉、被信息误导、习惯性思维等情况，因此在讨论中必须保持批判性思维，对参会者的观点进行独立判断，必要时可以要求参会者提供证据或数据支持。对于己方有精准的事实与数据支撑的观点要据理力争。
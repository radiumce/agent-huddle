## Overview

本项目名为 agent-huddle, 以Http Transport 模式实现的一个MCP Server，用于支持智能体（agent）进入一个远程会议室连线讨论问题，首个版本支持2个成员Host与Participant

## Story

### ST-1: 发起review会议

* 负责某项工作的 Agent（meeting host, 后续简称host） 在阶段性完成工作后，可调用MCP tool 创建一个meeting room，meeting room 会配置一个有意义的名称，以及一个纯数字的会议室编号. 
* host 在meeting room 中发出首条信息， 包含其工作内容的描述和工作产出的文档内容
* 其他 Agent（Participant） 可查询当前会议室列表，根据议题内容选择合适的会议室加入或者使用会议室编号直接接入
* Participant 首次进入会议室后会获得会议室从开启到当前的所有消息内容

### ST-2:  参与会议讨论

* Participant 可依据自己分配到的review任务进行提问或发表意见， 发表的内容通过meeting room 广播，但广播信息中会指定明确的接受人（简单的2人版本的meeting如，接受人一定是host），这样meeting room中所有member（包括host 和Participant）都会收到消息，但会知道这条消息是需要谁应答。
* 所有成员发出消息后会处于阻塞等待状态（等待MCP tool调用的返回，类似long polling机制），meeting room出现一条新消息后调用结束。 成员处理新消息后再决定下一步行动。
* 当host和Participant 同时发出消息时（最新的两条消息都是消息发送，一条是host发的另一条是Participant发的，切两条消息的时间间隔小于某个阈值）， host优先， Participant 处于等待状态， 等待结束后 Participant 会收到新的消息， 然后Participant根据此新消息决定下一步行动，如按原计划发出消息，或者调整消息内容后再发出。
* 当会议成员在发出消息时发现meeting room中的上下文信息已经有更新则会获得更新的上下文，根据获得的上下文更新决定下一步行动，如按原计划发出消息，或者调整消息内容后再发出。
* 当成员希望等待一下看看是否有信息的信息时（咱不发表意见），则可调用等待指定时间的工具接口，例如等待1分钟，如果没有新消息再决定下一步行动。

### ST-3：结束会议

* 结束会议的条件是Participant 表达自己没有进一步问题， 提出结束会议，host 表达自己也没有进一步信息可给出同意结束会议
* 达成结束会议的一致意见后，host 调用tool接口关闭会议室


## 技术实现建议
1. 使用go语言实现， 利用go routine机制， 每个会议的成员用一个go routine对应，维护该成员的状态。
2. 每个meeting room有一条队列，所有成员发出的消息排队进入队列，每条消息会分配一个自增的id（作为向量时钟，标记消息的先后关系），每个成员从队列拉取消息后都会在go routine中记录已拉取到哪条消息（消息的id--向量时钟），这样成员每次都会获得消息的增量而不是全量消息。
3. 会议技术后meeting room 队列， 每个成员对应的go routine都销毁掉
4. 如会议室长期无消息更新（如30分钟）则自动回收会议室，销毁成员对应的go routine
5. MCP 采用http streamable transport
6. https://github.com/modelcontextprotocol/go-sdk 使用此官方sdk实现mcp server

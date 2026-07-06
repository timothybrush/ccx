package scheduler

// SelectionTrace 记录一次渠道选择的关键阶段与候选跳过原因。
//
// 它只描述调度器已经做出的判断，不参与选择决策；调用方可用于日志、
// 诊断接口或测试断言。
type SelectionTrace struct {
	Kind        ChannelKind               `json:"kind"`
	Model       string                    `json:"model,omitempty"`
	RoutePrefix string                    `json:"routePrefix,omitempty"`
	ChannelName string                    `json:"channelName,omitempty"`
	AgentRole   string                    `json:"agentRole,omitempty"`
	Stages      []SelectionTraceStage     `json:"stages,omitempty"`
	Candidates  []SelectionTraceCandidate `json:"candidates,omitempty"`
	Selected    *SelectionTraceSelection  `json:"selected,omitempty"`
}

// SelectionTraceStage 记录某个过滤阶段后的候选数量。
type SelectionTraceStage struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// SelectionTraceCandidate 记录单个候选渠道在某阶段被跳过的原因。
type SelectionTraceCandidate struct {
	ChannelIndex int    `json:"channelIndex"`
	ChannelName  string `json:"channelName"`
	Stage        string `json:"stage"`
	Reason       string `json:"reason"`
	Details      string `json:"details,omitempty"`
}

// SelectionTraceSelection 记录最终选中的渠道。
type SelectionTraceSelection struct {
	ChannelIndex int    `json:"channelIndex"`
	ChannelName  string `json:"channelName"`
	Reason       string `json:"reason"`
}

func newSelectionTrace(opts SelectionOptions) *SelectionTrace {
	return &SelectionTrace{
		Kind:        opts.Kind,
		Model:       opts.Model,
		RoutePrefix: opts.RoutePrefix,
		ChannelName: opts.ChannelName,
		AgentRole:   opts.AgentRole,
	}
}

func (t *SelectionTrace) setStage(name string, count int) {
	if t == nil {
		return
	}
	t.Stages = append(t.Stages, SelectionTraceStage{Name: name, Count: count})
}

func (t *SelectionTrace) skipChannel(ch ChannelInfo, stage, reason, details string) {
	if t == nil {
		return
	}
	t.Candidates = append(t.Candidates, SelectionTraceCandidate{
		ChannelIndex: ch.Index,
		ChannelName:  ch.Name,
		Stage:        stage,
		Reason:       reason,
		Details:      details,
	})
}

func (t *SelectionTrace) selectChannel(channelIndex int, channelName, reason string) {
	if t == nil {
		return
	}
	t.Selected = &SelectionTraceSelection{
		ChannelIndex: channelIndex,
		ChannelName:  channelName,
		Reason:       reason,
	}
}

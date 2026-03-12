package handlers

type statusOption struct {
	Value string
	Label string
}

func processingStatusOptions() []statusOption {
	return []statusOption{
		{Value: "pending", Label: "待处理"},
		{Value: "processing", Label: "处理中"},
		{Value: "completed", Label: "已完成"},
	}
}

func faultStatusOptions() []statusOption {
	return []statusOption{
		{Value: "normal", Label: "正常"},
		{Value: "warning", Label: "告警"},
		{Value: "critical", Label: "严重"},
	}
}

func processingStatusLabel(value string) string {
	for _, item := range processingStatusOptions() {
		if item.Value == value {
			return item.Label
		}
	}
	return value
}

func faultStatusLabel(value string) string {
	for _, item := range faultStatusOptions() {
		if item.Value == value {
			return item.Label
		}
	}
	return value
}

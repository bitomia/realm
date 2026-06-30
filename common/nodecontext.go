package common

var (
	nodeContextBuilder NodeContexBuilder
)

type NodeContext struct {
	Repository NodesRepository
	NodeName   string
	RunMode    RunMode
}

type NodeContexBuilder func(nodeName string) NodeContext

func SetNodeContextBuilder(builder NodeContexBuilder) {
	nodeContextBuilder = builder
}

func NewNodeContext(nodeName string) NodeContext {
	if nodeContextBuilder == nil {
		panic("SetNodeContextBuilder not initialized")
	}
	return nodeContextBuilder(nodeName)
}

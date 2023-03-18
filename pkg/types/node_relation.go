package types

type NodeRel struct {
	NodeID  int64
	NodeRel string
}

func NewNodeRel(id int64, rel string) NodeRel {
	return NodeRel{
		NodeID:  id,
		NodeRel: rel,
	}
}

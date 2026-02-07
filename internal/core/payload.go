package core

import "time"

type Payload struct {
	rootNode  *Node
	createdAt time.Time
	nodes     []Node
}

func NewPayload(nodes []Node, root *Node) *Payload {
	return &Payload{
		rootNode:  root,
		nodes:     nodes,
		createdAt: time.Now(),
	}
}

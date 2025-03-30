package e2e

import (
	"fmt"
	"time"
)

// Cluster impl
type Cluster struct {
	nodes []*Node
}

func (c *Cluster) findNode(addr string) (*Node, error) {
	for _, v := range c.nodes {
		if v.RaftAddr == addr {
			return v, nil
		}
	}
	return nil, fmt.Errorf("node not found")
}

func (c *Cluster) Leader() (*Node, error) {
	l, err := c.nodes[0].WaitForLeader()
	if err != nil {
		return nil, err
	}
	return c.findNode(l)
}

func (c *Cluster) WaitForNewLeader(oldLeader *Node) (*Node, error) {
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timer.C:
			return nil, fmt.Errorf("timeout expired")
		case <-ticker.C:
			leader, err := c.Leader()
			if err != nil {
				continue
			}
			if !leader.sameAs(oldLeader) {
				return leader, nil
			}
		}
	}
}

func (c *Cluster) Followers() ([]*Node, error) {
	leaderAddr, err := c.nodes[0].WaitForLeader()
	if err != nil {
		return nil, err
	}
	leader, err := c.findNode(leaderAddr)
	if err != nil {
		return nil, err
	}

	var followers []*Node
	for _, v := range c.nodes {
		if v != leader {
			followers = append(followers, v)
		}
	}

	return followers, nil
}

func (c *Cluster) RemoveNode(n *Node) {
	newNodes := make([]*Node, 0)
	for _, v := range c.nodes {
		if v.RaftAddr != n.RaftAddr {
			newNodes = append(newNodes, v)
		}
	}
	c.nodes = newNodes
}

// Shutdown ... Shutdown each Node in Cluster
func (c *Cluster) Shutdown() {
	for _, v := range c.nodes {
		v.Shutdown()
	}
}

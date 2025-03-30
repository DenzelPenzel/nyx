package e2e

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_MultiNodes(t *testing.T) {
	node1 := CreateLeaderNode()
	defer node1.Shutdown()

	node2 := CreateNewNode(false)
	defer node1.Shutdown()

	err := node2.Join(node1)
	require.NoError(t, err)

	lAddr, err := node2.WaitForLeader()
	require.NoError(t, err)
	require.Equal(t, node1.RaftAddr, lAddr)

	c := &Cluster{
		nodes: []*Node{node1, node2},
	}

	l, err := c.Leader()
	require.NoError(t, err)

	node3 := CreateNewNode(false)
	defer node3.Shutdown()
	err = node3.Join(l)
	require.NoError(t, err)

	lAddr, err = node3.WaitForLeader()
	require.NoError(t, err)
	require.Equal(t, node1.RaftAddr, lAddr)

	c = &Cluster{
		nodes: []*Node{node1, node2, node3},
	}
	l, err = c.Leader()
	require.NoError(t, err)
}

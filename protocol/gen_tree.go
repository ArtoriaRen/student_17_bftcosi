package protocol

import (
	"gopkg.in/dedis/onet.v1"
	"errors"
	"fmt"
)

// GenTree will create a tree of n servers with a localRouter, and returns the
// list of servers and the associated roster / tree.
// NOTE: register being not implementable with the current API could hurt the scalability tests
func GenTrees(servers []*onet.Server, rosterGenerator func(...*onet.Server) *onet.Roster, nNodes, nSubtrees int) ([]*onet.Tree, error) {

	//parameter verification
	if servers == nil {
		return nil, errors.New("the roster is nil")
	}
	if nNodes < 1 {
		return nil, fmt.Errorf("the number of nodes in the global tree " +
			"cannot be less than one, but is %d", nNodes)
	}
	if len(servers) < nNodes {
		return nil, fmt.Errorf("the global tree should have %d nodes, " +
			"but there is only %d servers", nNodes, len(servers))
	}
	if nSubtrees < 1 {
		return nil, fmt.Errorf("the number of shards in the global tree " +
			"cannot be less than one, but is %d", nSubtrees)
	}

	if nNodes <= nSubtrees {
		nSubtrees = nNodes -1
	}

	trees := make([]*onet.Tree, nSubtrees)

	if nSubtrees == 0 {
		roster := rosterGenerator(servers[0])
		rootNode := onet.NewTreeNode(0, roster.List[0])
		trees = append(trees, onet.NewTree(roster, rootNode))
		return trees, nil
	}


	//generate each shard
	nodesPerShard := (nNodes - 1) / nSubtrees
	surplusNodes := (nNodes - 1) % nSubtrees

	start := 1
	for i := 0 ; i< nSubtrees; i++ {

		end := start + nodesPerShard
		if i < surplusNodes { //to handle surplus nodes
			end++
		}

		//generate tree roster
		treeServers := []*onet.Server{servers[0]}
		treeServers = append(treeServers, servers[start:end]...)
		treeRoster := rosterGenerator(treeServers...)

		//generate leader and subleader
		rootNode := onet.NewTreeNode(0, treeRoster.List[0])
		subleader := onet.NewTreeNode(1, treeRoster.List[1])
		subleader.Parent = rootNode
		rootNode.Children = []*onet.TreeNode{subleader}

		//generate leaves
		for j := 2 ; j < end-start+1 ; j++ {
			node := onet.NewTreeNode(j, treeRoster.List[j])
			node.Parent = subleader
			subleader.Children = append(subleader.Children, node)
		}

		start = end
		trees[i] = onet.NewTree(treeRoster, rootNode)
	}


	//l.Trees[tree.ID] = tree
	//if registerOLD {
	//	servers[0].overlay.RegisterRoster(list)
	//	servers[0].overlay.RegisterTree(tree)
	//}

	return trees, nil
}

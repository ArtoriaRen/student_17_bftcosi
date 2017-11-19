package protocol

import (
	"gopkg.in/dedis/onet.v1"
	"errors"
	"fmt"
	"gopkg.in/dedis/onet.v1/network"
)

// GenTree will create a tree of n servers with a localRouter, and returns the
// list of servers and the associated roster / tree.
// NOTE: register being not implementable with the current API could hurt the scalability tests
func GenTrees(roster *onet.Roster, nNodes, nSubtrees int) ([]*onet.Tree, error) {

	//parameter verification
	if roster == nil {
		return nil, errors.New("the roster is nil")
	}
	if nNodes < 1 {
		return nil, fmt.Errorf("the number of nodes in the trees " +
			"cannot be less than one, but is %d", nNodes)
	}
	if len(roster.List) < nNodes {
		return nil, fmt.Errorf("the trees should have %d nodes, " +
			"but there is only %d servers in the roster", nNodes, len(roster.List))
	}
	if nSubtrees < 1 {
		return nil, fmt.Errorf("the number of shards in the trees " +
			"cannot be less than one, but is %d", nSubtrees)
	}

	if nNodes <= nSubtrees {
		nSubtrees = nNodes -1
	}

	trees := make([]*onet.Tree, nSubtrees)

	if nSubtrees == 0 {
		localRoster := onet.NewRoster(roster.List[0:1])
		rootNode := onet.NewTreeNode(0, localRoster.List[0])
		trees = append(trees, onet.NewTree(localRoster, rootNode))
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
		servers := []*network.ServerIdentity{roster.List[0]}
		servers = append(servers, roster.List[start:end]...)
		treeRoster := onet.NewRoster(servers)

		var err error
		trees[i], err = genSubtree(treeRoster, 1)
		if err != nil {
			return nil, err
		}

		start = end
	}


	//l.Trees[tree.ID] = tree
	//if registerOLD {
	//	servers[0].overlay.RegisterRoster(list)
	//	servers[0].overlay.RegisterTree(tree)
	//}

	return trees, nil
}

func genSubtree(roster *onet.Roster, subleaderID int) (*onet.Tree, error) {

	//generate leader and subleader
	rootNode := onet.NewTreeNode(0, roster.List[0])
	subleader := onet.NewTreeNode(subleaderID, roster.List[subleaderID])
	subleader.Parent = rootNode
	rootNode.Children = []*onet.TreeNode{subleader}

	//generate leaves
	for j := 1 ; j < len(roster.List) ; j++ {
		if j != subleaderID {
			node := onet.NewTreeNode(j, roster.List[j])
			node.Parent = subleader
			subleader.Children = append(subleader.Children, node)
		}
	}

	return onet.NewTree(roster, rootNode), nil
}

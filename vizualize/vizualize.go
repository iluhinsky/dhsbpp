package vizualize

import (
	"fmt"
	"os"
	"os/exec"
	"sort"

	"github.com/iluhinsky/dhsbpp/packing"
	"github.com/iluhinsky/dhsbpp/tree"
)

type DotNode struct {
	BinIndex int

	Children []*DotNode
	Parent   *DotNode

	NumOfNodes int64
	pRoot      *tree.PartitionNode
	pNodes     map[*tree.PartitionNode]bool
}

func MakeVisualizationPicture(bins []*packing.Bin, pngFileName string, dirToSave string) error {
	var treeDistribution = newDotTree(bins)

	var dotFile, err = os.Create("treeDistribution.dot")
	if err != nil {
		return err
	}
	if err := writeTreeToDotFile(treeDistribution, dotFile); err != nil {
		return err
	}

	if err = makePngFile(dotFile, pngFileName, dirToSave); err != nil {
		return err
	}

	if err = os.Remove(dotFile.Name()); err != nil {
		return err
	}

	return nil
}

func makePngFile(dotFile *os.File, pngFileName string, dirToSave string) error {
	var cmd = exec.Command("dot", "-Tpng", dotFile.Name(), "-o", dirToSave+pngFileName)
	var err = cmd.Run()

	return err
}

func newDotTree(bins []*packing.Bin) *DotNode {
	var unconnectedDotNodes = make(map[*DotNode]bool)
	for idx := range bins {
		var rootNodes = bins[idx].MakeMapRootNodesOfBin()

		for rootNode := range rootNodes {
			var pNodes = findPartitionNodesOfDotNode(bins[idx].PartNodes, rootNode)
			var dotNode = DotNode{BinIndex: bins[idx].Index, pRoot: rootNode, pNodes: pNodes}
			unconnectedDotNodes[&dotNode] = true
		}
	}

	var rootDotNode = connectBinNodes(unconnectedDotNodes)

	return rootDotNode
}

func connectBinNodes(unconnectedDotNodes map[*DotNode]bool) *DotNode {
	var rootDotNode *DotNode

	for dotNode := range unconnectedDotNodes {
		var parent = findParent(dotNode, unconnectedDotNodes)

		if parent != nil {
			dotNode.Parent = parent
			parent.Children = append(parent.Children, dotNode)
		} else {
			rootDotNode = dotNode
		}
	}

	return rootDotNode
}

func findParent(child *DotNode, dotNodes map[*DotNode]bool) *DotNode {
	for dotNode := range dotNodes {
		if ok := dotNode.pNodes[child.pRoot.Parent]; ok {
			return dotNode
		}
	}

	return nil
}

func findPartitionNodesOfDotNode(allPartNodes map[*tree.PartitionNode]bool,
	pRoot *tree.PartitionNode) map[*tree.PartitionNode]bool {

	var pNodesOfDotNode = make(map[*tree.PartitionNode]bool)

	findPartitionNodesOfDotNodeFunc(pRoot, allPartNodes, pNodesOfDotNode)

	return pNodesOfDotNode
}

func findPartitionNodesOfDotNodeFunc(pNode *tree.PartitionNode, allPartNodes map[*tree.PartitionNode]bool,
	pNodesOfDotNode map[*tree.PartitionNode]bool) {
	if ok, _ := allPartNodes[pNode]; !ok {
		return
	}

	pNodesOfDotNode[pNode] = true

	for _, children := range pNode.Children {
		findPartitionNodesOfDotNodeFunc(children, allPartNodes, pNodesOfDotNode)
	}
}

func writeTreeToDotFile(binTree *DotNode, dotFile *os.File) error {

	_, err := fmt.Fprintf(dotFile,
		"digraph G                                                          \n"+
			"{                                                                     \n"+
			"  node[shape = \"box\", color=\"black\",fontsize=14,      	           \n"+
			"  style=\"filled\"];                                                  \n"+
			"  edge[color=\"black\",style=\"bold\"];\n")

	if err != nil {
		return err
	}

	err = drawNodeDot(binTree, dotFile)
	if err != nil {
		return err
	}

	if _, err = fmt.Fprintf(dotFile, "}"); err != nil {
		return err
	}
	if err = dotFile.Close(); err != nil {
		return err
	}

	return nil
}

var m = map[int]string{
	1: "cadetblue1", 2: "darkorchid1", 3: "gold1", 4: "olivedrab2", 5: "plum1", 6: "antiquewhite1",
	7: "coral", 8: "gray40", 9: "violetred1", 10: "aquamarine4",
}

func drawNodeDot(binNode *DotNode, dotFile *os.File) error {
	var _, err = fmt.Fprintf(dotFile, "  ptr%p[label=\""+
		"bin index = %v\n"+
		"number of nodes = %v\n"+
		"size = %v\","+
		"fillcolor=\"%v\"];\n", binNode, binNode.BinIndex, len(binNode.pNodes),
		calculateSizeOfDotNode(binNode), m[binNode.BinIndex])

	if err != nil {
		return err
	}

	sort.Slice(binNode.Children, func(i, j int) bool {
		if binNode.Children[i].BinIndex != binNode.Children[j].BinIndex {
			return binNode.Children[i].BinIndex < binNode.Children[j].BinIndex
		} else {
			return calculateSizeOfDotNode(binNode.Children[i]) < calculateSizeOfDotNode(binNode.Children[j])
		}
	})

	for _, child := range binNode.Children {
		if _, err = fmt.Fprintf(dotFile, "  ptr%p->ptr%p;\n", binNode, child); err != nil {
			return err
		}
		if err = drawNodeDot(child, dotFile); err != nil {
			return err
		}
	}

	return nil
}

func calculateSizeOfDotNode(binNode *DotNode) int64 {
	var size int64 = 0
	for pNode := range binNode.pNodes {
		size += pNode.NodeSize
	}
	return size
}

package packing

import (
	"fmt"
	"sort"

	"github.com/iluhinsky/dhsbpp/tree"
)

var AlgorithmPackingFunc func(*tree.PartitionNode, []*Bin) []*Bin
var SeparateFunc func(pNode *tree.PartitionNode) ([]*tree.PartitionNode, []*tree.PartitionNode)

var AllocationFactor int64 = 60  // default value
var ReallocationDelta int64 = 20 // default value
var MaxCapacity int64
var InitEpochs int

var Volume int64
var OverloadThreshold int64
var UnderloadThreshold int64

func UpdParams() {
	Volume = MaxCapacity * AllocationFactor / 100
	OverloadThreshold = MaxCapacity * (AllocationFactor + ReallocationDelta) / 100
	UnderloadThreshold = MaxCapacity * (AllocationFactor - ReallocationDelta) / 100
}

type Bin struct {
	Index     int
	Size      int64
	PartNodes map[*tree.PartitionNode]bool
}

func NewBin(idx int) *Bin {
	var bin Bin

	bin.Index = idx
	bin.Size = 0
	bin.PartNodes = make(map[*tree.PartitionNode]bool)

	return &bin
}

func (bin *Bin) AddToBinSize(pNode *tree.PartitionNode, tasks int64) {
	pNode.AddToNodeSize(tasks)

	bin.Size += tasks
}

func (bin *Bin) MakeMapRootNodesOfBin() map[*tree.PartitionNode]bool {
	var tmpPartNodes = make(map[*tree.PartitionNode]bool)
	for node := range bin.PartNodes {
		tmpPartNodes[node] = true
	}

	for node := range tmpPartNodes {
		if _, ok := bin.PartNodes[node.Parent]; ok {
			delete(tmpPartNodes, node)
		}
	}

	return tmpPartNodes
}

func (bin *Bin) MakeSliceRootNodesOfBin() []*tree.PartitionNode {
	var sliceRootNodesOfBin = make([]*tree.PartitionNode, 0)
	for rootNode := range bin.MakeMapRootNodesOfBin() {
		sliceRootNodesOfBin = append(sliceRootNodesOfBin, rootNode)
	}

	return sliceRootNodesOfBin
}

func (bin *Bin) AddSubTree(pNode *tree.PartitionNode) {
	bin.Size += pNode.SubTreeSize
	bin.addNodes(pNode)
}

func (bin *Bin) addNodes(pNode *tree.PartitionNode) {
	bin.PartNodes[pNode] = true

	for _, child := range pNode.Children {
		bin.addNodes(child)
	}
}

func (bin *Bin) freeBin() {
	bin.PartNodes = make(map[*tree.PartitionNode]bool) // garbage collector?
	bin.Size = 0
}

func PreprocessPartitionTree(pRoot *tree.PartitionNode) {
	if pRoot.NodeSize > Volume {

		rootChunk := tree.PartitionNode{Name: pRoot.Name + "#", Parent: pRoot, Children: pRoot.Children,
			NodeSize: pRoot.NodeSize - Volume, SubTreeSize: pRoot.SubTreeSize - Volume}

		pRoot.NodeSize = Volume
		pRoot.Children = nil
		pRoot.Children = append(pRoot.Children, &rootChunk)
	}

	for _, child := range pRoot.Children {
		PreprocessPartitionTree(child)
	}
}

func HierarchicalFirstFitDecreasing(pNode *tree.PartitionNode, bins []*Bin) []*Bin {
	if pNode.SubTreeSize <= Volume {
		var bin = findBinForFit(bins, pNode)

		if bin != nil {
			bin.AddSubTree(pNode)
		} else {
			var bin = NewBin(len(bins) + 1)
			bin.AddSubTree(pNode)
			bins = append(bins, bin)
		}
	} else {
		//	var separate, forUnite = pNode.SeparateMaxChild()
		var separate, forUnite = SeparateFunc(pNode)

		for _, node := range separate {
			bins = HierarchicalFirstFitDecreasing(node, bins)
		}

		Unite(pNode, forUnite)
	}

	return bins
}

func HierarchicalGreedyDecreasing(pNode *tree.PartitionNode, bins []*Bin) []*Bin {
	if pNode.SubTreeSize <= Volume {
		var bin = NewBin(len(bins) + 1)
		bin.AddSubTree(pNode)
		bins = append(bins, bin)
	} else {
		var separate, forUnite = SeparateFunc(pNode)

		for _, node := range separate {
			bins = HierarchicalGreedyDecreasing(node, bins)
		}

		Unite(pNode, forUnite)
	}

	return bins
}

func findBinForFit(bins []*Bin, pNode *tree.PartitionNode) *Bin {
	for _, bin := range bins {
		if pNode.SubTreeSize <= Volume-bin.Size {
			return bin
		}
	}

	return nil
}

func FindBinForRebalancing(bins []*Bin, tasksPerEpoch []map[string]int64,
	nameToPartNode map[string]*tree.PartitionNode) *Bin {

	var initiallyUnderloadedBins = make(map[*Bin]bool)
	for _, bin := range bins {
		initiallyUnderloadedBins[bin] = bin.Size <= UnderloadThreshold
	}

	var loadedBin *Bin
	for i := InitEpochs; i < len(tasksPerEpoch) && loadedBin == nil; i++ {

		updateSizeInOneTimeInterval(bins, tasksPerEpoch[i], nameToPartNode, true)             //Add
		updateSizeInOneTimeInterval(bins, tasksPerEpoch[i-InitEpochs], nameToPartNode, false) //Sub

		loadedBin = findOverOrUnderloadedBin(bins, initiallyUnderloadedBins)
	}

	return loadedBin
}

func DynamicalAlgorithmPackingFunc(loadedBin *Bin, bins []*Bin) ([]*Bin, int64) {
	var oldSize = loadedBin.Size
	var untiedChildren = untieChildNodesOfBinFromOtherBins(loadedBin)
	var sliceRootNodesOfBin = loadedBin.MakeSliceRootNodesOfBin()
	loadedBin.freeBin()

	sort.Slice(sliceRootNodesOfBin, func(i, j int) bool {
		return sliceRootNodesOfBin[i].SubTreeSize > sliceRootNodesOfBin[j].SubTreeSize
	})

	for _, rootNode := range sliceRootNodesOfBin {
		PreprocessPartitionTree(rootNode) // think later
		bins = AlgorithmPackingFunc(rootNode, bins)
	}

	tieChildNodesToOtherBins(untiedChildren)

	//fmt.Println(oldSize, loadedBin.Size)
	var migrationSize = oldSize - loadedBin.Size

	return bins, migrationSize
}

func untieChildNodesOfBinFromOtherBins(bin *Bin) map[*tree.PartitionNode][]*tree.PartitionNode {
	var untiedChildren = make(map[*tree.PartitionNode][]*tree.PartitionNode)

	for pNode := range bin.PartNodes {
		for _, child := range pNode.Children {
			if ok := bin.PartNodes[child]; !ok {
				untiedChildren[pNode] = append(untiedChildren[pNode], child)
			}
		}
	}
	var sum int64
	for pNode, children := range untiedChildren {
		for _, child := range children {
			pNode.RemoveChild(child) // mistake!!!!!
			sum += child.SubTreeSize
		}
	}
	fmt.Println(sum)
	return untiedChildren
}

func tieChildNodesToOtherBins(untiedChildren map[*tree.PartitionNode][]*tree.PartitionNode) {
	for pNode, children := range untiedChildren {
		for _, child := range children {
			pNode.AppendChild(child)
		}
	}
}

func updateSizeInOneTimeInterval(bins []*Bin, tasksPerOneEpoch map[string]int64,
	nameToPartNode map[string]*tree.PartitionNode, isPlus bool) {

	for name, tasks := range tasksPerOneEpoch { //Add
		var pNode = nameToPartNode[name]

		if !isPlus {
			tasks = -tasks
		}

		for _, bin := range bins {
			if ok := bin.PartNodes[pNode]; ok {
				bin.AddToBinSize(pNode, tasks)
				break
			}
		}
	}
}

func findOverOrUnderloadedBin(bins []*Bin, initiallyUnderloadedBins map[*Bin]bool) *Bin {
	for _, bin := range bins {
		if (bin.Size >= OverloadThreshold) ||
			(bin.Size <= UnderloadThreshold && !initiallyUnderloadedBins[bin]) {
			return bin
		}
	}

	return nil
}

func SeparateRoot(pNode *tree.PartitionNode) ([]*tree.PartitionNode, []*tree.PartitionNode) {
	var separate = make([]*tree.PartitionNode, 0, len(pNode.Children)+1)
	for _, ch := range pNode.Children {
		separate = append(separate, ch)
	}
	var forUnite = pNode.Children
	pNode.Children = nil
	pNode.SubTreeSize = pNode.NodeSize
	separate = append(separate, pNode)

	sort.Slice(separate, func(i, j int) bool {
		return separate[i].SubTreeSize > separate[j].SubTreeSize
	})

	return separate, forUnite
}

func SeparateMaxChild(pNode *tree.PartitionNode) ([]*tree.PartitionNode, []*tree.PartitionNode) {
	var maxChild *tree.PartitionNode
	var maxSize int64 = 0
	for _, child := range pNode.Children {
		if child.SubTreeSize >= maxSize {
			maxSize = child.SubTreeSize
			maxChild = child
		}
	}

	if maxChild == nil {
		fmt.Println("something wrong")
		return nil, nil
	}

	// pNode.RemoveChild(maxChild)  ?? Debug
	for idx := range pNode.Children { // Debug
		if pNode.Children[idx] == maxChild {
			pNode.Children = append(pNode.Children[:idx], pNode.Children[idx+1:]...) // mistake
			break
		}
	}
	pNode.SubTreeSize -= maxChild.SubTreeSize

	var separate = make([]*tree.PartitionNode, 0)
	if maxChild.SubTreeSize > pNode.SubTreeSize {
		separate = append(separate, maxChild, pNode)
	} else {
		separate = append(separate, pNode, maxChild)
	}
	var forUnite = make([]*tree.PartitionNode, 0, 1)
	forUnite = append(forUnite, maxChild)

	return separate, forUnite
}

func Unite(pNode *tree.PartitionNode, children []*tree.PartitionNode) {
	for _, child := range children {
		pNode.Children = append(pNode.Children, child)
		pNode.SubTreeSize += child.SubTreeSize
	}
}

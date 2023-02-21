package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/iluhinsky/dhsbpp/hierarchy"
	"github.com/iluhinsky/dhsbpp/packing"
	"github.com/iluhinsky/dhsbpp/tree"
	"github.com/iluhinsky/dhsbpp/vizualize"
)

var datasetName string

func parseParameters() error {
	var isDataset, isAlgorithm, isSeparate, isMaxCapacity, isInitEpochs bool

	for idx := 1; idx < len(os.Args); idx++ {
		var arg = os.Args[idx]
		if arg[0] != '-' {
			return errors.New("error: unknown argument '" + arg + "'")
		}
		var slice = strings.Split(arg[1:], "=")
		if len(slice) != 2 {
			return errors.New("error: unknown argument '" + arg + "'")
		}

		var parameter, value = slice[0], slice[1]

		switch parameter {
		case "dataset":
			isDataset = true
			datasetName = value
			// check dir

		case "algorithm":
			isAlgorithm = true
			if value == "first_fit" {
				packing.AlgorithmPackingFunc = packing.HierarchicalFirstFitDecreasing
			} else if value == "greedy" {
				packing.AlgorithmPackingFunc = packing.HierarchicalGreedyDecreasing
			} else {
				isAlgorithm = false
				return errors.New("error: unknown argument '" + arg + "'")
			}

		case "separate":
			isSeparate = true
			if value == "root" {
				packing.SeparateFunc = packing.SeparateRoot
			} else if value == "max_child" {
				packing.SeparateFunc = packing.SeparateMaxChild
			} else {
				isSeparate = false
				return errors.New("error: unknown argument '" + arg + "'")
			}
		case "max_capacity":
			isMaxCapacity = true
			var n, err = strconv.ParseInt(value, 10, 64)
			if err != nil || n <= 0 {
				isMaxCapacity = false
				return errors.New("error: unknown argument '" + arg + "'")
			}
			packing.MaxCapacity = n

		case "init_epochs":
			isInitEpochs = true
			var n, err = strconv.Atoi(value)
			if err != nil || n <= 0 {
				isInitEpochs = false
				return errors.New("error: unknown argument '" + arg + "'")
			}
			packing.InitEpochs = n

		case "AF":
			var n, err = strconv.ParseInt(value, 10, 64)
			if err != nil || n <= 0 {
				return errors.New("error: unknown argument '" + arg + "'")
			}
			packing.AllocationFactor = n
		case "RD":
			var n, err = strconv.ParseInt(value, 10, 64)
			if err != nil || n <= 0 {
				return errors.New("error: unknown argument '" + arg + "'")
			}
			packing.ReallocationDelta = n
		}
	}

	if !isDataset {
		return errors.New("error: datasetName not specified")
	}
	if !isAlgorithm {
		return errors.New("error: algorithm not specified")
	}
	if !isSeparate {
		return errors.New("error: separate not specified")
	}
	if !isMaxCapacity {
		return errors.New("error: max_capacity not specified")
	}
	if !isInitEpochs {
		return errors.New("error: init_epochs not specified")
	}

	return nil
}

func main() {
	var err = parseParameters()
	packing.UpdParams()
	if err != nil {
		fmt.Println(err)
		return
	}

	var datasetPath = os.Getenv("HOME") + "/go/src/github.com/dati-mipt/dhsbpp/datasets/" + datasetName
	fmt.Println(datasetPath)
	var newHierarchy = hierarchy.NewHierarchy(datasetPath+"/ChildParent.csv",
		datasetPath+"/WeightsPerEpoch.csv")

	root, err := tree.NewTree(newHierarchy.ChildToParent)
	if err != nil {
		fmt.Println(err)
		return
	}

	var pRoot = tree.NewPartitionTree(root)

	err = pRoot.SetInitialSize(newHierarchy.WeightsPerEpoch, packing.InitEpochs)
	if err != nil {
		fmt.Println(err)
		return
	}

	packing.PreprocessPartitionTree(pRoot)
	var bins = make([]*packing.Bin, 0)
	bins = packing.AlgorithmPackingFunc(pRoot, bins)

	var picsPath = os.Getenv("HOME") + "/go/src/github.com/dati-mipt/dhsbpp/outputPics/"
	err = vizualize.MakeVisualizationPicture(bins, "1distribution.png", picsPath)
	if err != nil {
		fmt.Println(err)
		return
	}

	nameToPartitionNode, _ := pRoot.MapNameToPartitionNode()
	var loadedBin = packing.FindBinForRebalancing(bins, newHierarchy.WeightsPerEpoch, nameToPartitionNode)

	err = vizualize.MakeVisualizationPicture(bins, "2distribution.png", picsPath)
	if err != nil {
		fmt.Println(err)
		return
	}
	var sum int64
	for _, bin := range bins {
		sum += bin.Size
	}
	fmt.Println(sum, "1")
	if loadedBin != nil {
		var migrationSize int64
		bins, migrationSize = packing.DynamicalAlgorithmPackingFunc(loadedBin, bins)
		fmt.Println("Number of bins", len(bins))
		fmt.Println("Bin Index:", loadedBin.Index)
		fmt.Println("Migration Size:", migrationSize)

		err = vizualize.MakeVisualizationPicture(bins, "3distribution.png", picsPath)
		if err != nil {
			fmt.Println(err)
			return
		}
	}
	sum = 0
	for _, bin := range bins {
		sum += bin.Size
	}
	fmt.Println(sum, "1")
}

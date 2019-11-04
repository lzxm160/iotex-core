// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package trie

import (
	"fmt"

	"github.com/pkg/errors"
)

// ErrEndOfIterator defines an error which will be returned
var ErrEndOfIterator = errors.New("hit the end of the iterator, no more item")

// Iterator iterates a trie
type Iterator interface {
	Next() ([]byte, []byte, error)
}

// LeafIterator defines an iterator to go through all the leaves under given node
type LeafIterator struct {
	tr      Trie
	stack   []Node
	allNode [][]byte
}

// NewLeafIterator returns a new leaf iterator
func NewLeafIterator(tr Trie) (Iterator, error) {
	rootHash := tr.RootHash()
	root, err := tr.loadNodeFromDB(rootHash)
	if err != nil {
		return nil, err
	}
	stack := []Node{root}
	return &LeafIterator{tr: tr, stack: stack}, nil
}

// Next moves iterator to next node
func (li *LeafIterator) Next() ([]byte, []byte, error) {
	for len(li.stack) > 0 {
		size := len(li.stack)
		node := li.stack[size-1]
		li.stack = li.stack[:size-1]
		if node.Type() == LEAF {
			key := node.Key()
			value := node.Value()

			return append(key[:0:0], key...), value, nil
		}
		children, err := node.children(li.tr)
		if err != nil {
			return nil, nil, err
		}
		li.stack = append(li.stack, children...)
	}

	return nil, nil, ErrEndOfIterator
}

// Next moves iterator to next node
func (li *LeafIterator) All() (ret [][]byte, err error) {
	for len(li.stack) > 0 {
		size := len(li.stack)
		node := li.stack[size-1]
		li.stack = li.stack[:size-1]
		if node.Type() == LEAF {
			//	copyNode := make([]byte, len(node.Key()))
			//	copy(copyNode, node.Key())
			//	li.allNode = append(li.allNode, copyNode)
			return
		}
		switch node.Type() {
		case EXTENSION:
			fmt.Println("extension:", node.Key(), ":", node.Value())
			ret = append(ret, node.Value())
		case BRANCH:
			branch, _ := node.(*branchNode)
			for k, v := range branch.hashes {
				fmt.Println("branch:", k, ":", v)
				ret = append(ret, v)
			}
		}
		children, errs := node.children(li.tr)
		if errs != nil {
			err = errs
			return
		}
		li.stack = append(li.stack, children...)
		//for _, v := range children {
		//	copyNode := make([]byte, len(v.Key()))
		//	copy(copyNode, v.Key())
		//	li.allNode = append(li.allNode, copyNode)
		//}
	}

	return nil, ErrEndOfIterator
}

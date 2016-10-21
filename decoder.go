package mafsa

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"unicode/utf8"
)

// Decoder is a type which can decode a byte slice into a MinTree.
type Decoder struct {
	fileVer int
	ptrLen  int
	nodeMap map[int]*MinTreeNode
	tree    *MinTree
}

// Decode transforms the binary serialization of a MA-FSA into a
// new MinTree (a read-only MA-FSA).
func (d *Decoder) Decode(data []byte) (*MinTree, error) {
	tree := newMinTree()
	return tree, d.decodeMinTree(tree, data)
}

// ReadFrom reads the binary serialization of a MA-FSA into a
// new MinTree (a read-only MA-FSA) from a io.Reader.
func (d *Decoder) ReadFrom(r io.Reader) (*MinTree, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	tree := newMinTree()
	return tree, d.decodeMinTree(tree, data)
}

// decodeMinTree transforms the binary serialization of a MA-FSA into a
// read-only MA-FSA pointed to by t.
func (d *Decoder) decodeMinTree(t *MinTree, data []byte) error {
	if len(data) < 2 {
		return errors.New("Not enough bytes")
	}

	// First word contains some file format information
	d.fileVer = int(data[0])
	d.ptrLen = int(data[1])
	if d.ptrLen != 2 && d.ptrLen != 4 && d.ptrLen != 8 {
		return fmt.Errorf("Only 2, 4 and 8 are valid pointer sizes but we got: %d", d.ptrLen)
	}

	// The node map translates from byte slice offsets to
	// actual node pointers in the resulting tree
	d.nodeMap = make(map[int]*MinTreeNode)

	// The node map is only needed during decoding
	defer func() {
		d.nodeMap = make(map[int]*MinTreeNode)
	}()

	// We need access to the tree so we can implement
	// minimal perfect hashing in the recursive function later
	d.tree = t

	// Begin decoding at the root node, which starts
	// at ptrLen+flags+1(min char len) in the byte slice
	err := d.decodeEdge(data, t.Root, d.ptrLen+1+1, []rune{})
	if err != nil {
		return err
	}

	// Traverse the tree once it is built so that each node
	// has the right number. The number represents the number
	// of words attainable by STARTING at that node.
	d.doNumbers(t.Root)

	return nil
}

// decodeEdge decodes the edge described by the word of the byte slice
// starting at offset, and adds each subsequent edge on this same
// node to parent, which is already in the tree. After adding the
// immediate child nodes to parent, it recursively follows the
// pointer at the end of the word to subsequent child nodes.
func (d *Decoder) decodeEdge(data []byte, parent *MinTreeNode, offset int, entry []rune) error {
	for i := offset; i < len(data); {
		// Break the word apart into the pieces we need
		// First we get the flags which also contains the
		// charLen in bytes (in the three bits before the least
		// significant two)
		flags := data[i]
		charLen := int(flags >> 2)
		charBytes := data[i+1 : i+charLen+1]
		ptrBytes := data[i+charLen+1 : i+charLen+d.ptrLen+1]

		final := flags&endOfWord == endOfWord
		lastChild := flags&endOfNode == endOfNode

		r, _ := utf8.DecodeRune(charBytes)
		if r == utf8.RuneError {
			return fmt.Errorf("Found invalid UTF8 sequence: %x\n", charBytes)
		}

		ptr, err := d.decodePointer(ptrBytes)
		if err != nil {
			return err
		}

		// If this word/edge points to a node we haven't
		// seen before, add it to the node map
		if _, ok := d.nodeMap[ptr]; !ok {
			d.nodeMap[ptr] = &MinTreeNode{
				Edges: make(map[rune]*MinTreeNode),
				Final: final,
			}
		}

		// Add edge to node
		parent.Edges[r] = d.nodeMap[ptr]
		entry := append(entry, r) // TODO: Ugh, redeclaring entry seems weird here, but it's necessary, no?

		i += charLen + d.ptrLen + 1
		// If there are edges to other nodes, decode them
		if ptr > 0 {
			d.decodeEdge(data, d.nodeMap[ptr], ptr, entry)
		}

		// If this word represents the last outgoing edge
		// for this node, stop iterating the file at this level
		if lastChild {
			break
		}
	}

	return nil
}

// doNumbers sets the number on this node to the number
// of entries accessible by starting at this node.
func (d *Decoder) doNumbers(node *MinTreeNode) {
	if node.Number > 0 {
		// We've already visited this node
		return
	}
	for _, child := range node.Edges {
		// A node's number is the sum of the
		// numbers of its immediate child nodes.
		// (The paper did not explicitly state
		// this, but as it turns out, that's the
		// rule.)
		d.doNumbers(child)
		if child.Final {
			node.Number++
		}
		node.Number += child.Number
	}
}

// decodePointer converts a byte slice containing a number to
// the offset in the byte array where the next child is to
// an int that can be used to index into the byte slice.
func (d *Decoder) decodePointer(ptrBytes []byte) (int, error) {
	switch d.ptrLen {
	case 2:
		return int(binary.BigEndian.Uint16(ptrBytes)), nil
	case 4:
		return int(binary.BigEndian.Uint32(ptrBytes)), nil
	case 8:
		return int(binary.BigEndian.Uint64(ptrBytes)), nil
	default:
		return 0, errors.New("Child offset pointer must be only be 2, 4, or 8 bytes long")
	}
}

const (
	endOfWord = 1 << iota
	endOfNode
)

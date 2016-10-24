package mafsa

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"sort"
	"unicode/utf8"
)

// Encoder is a type which can encode a BuildTree into a byte slice
// which can be written to a file.
type Encoder struct {
	queue     []*BuildTreeNode
	curOffset int
}

// Encode serializes a BuildTree t into a byte slice.
func (e *Encoder) Encode(t *BuildTree) ([]byte, error) {
	e.queue = []*BuildTreeNode{}

	// First entry (== fixed-length entry to which a node can be
	// serialized) is a null entry that specifies the file format:
	// First byte indicates the flag scheme (basically a file format verison number)
	// Second byte is the pointer length in bytes
	//   Note: entry length can be calculated as follows:
	//         Second byte + 1 (flags) + char length in bytes (1-4, encoded in the higher flag bits)
	// Any leftover bytes in this first entry are zero
	data := []byte{0x02, 0x04}
	for i := len(data); i < int(data[1]+1+1); i++ {
		data = append(data, 0x00)
	}
	e.curOffset = e.getChildNodesSizeTotal(data, t.Root.Edges) + int(data[1]+1+1)

	data = e.encodeEdges(t.Root, data)

	for len(e.queue) > 0 {
		// Pop first item off the queue
		top := e.queue[0]
		e.queue = e.queue[1:]

		// Recursively marshal child nodes
		data = e.encodeEdges(top, data)
	}

	return data, nil
}

// WriteTo encodes and saves the BuildTree to a io.Writer.
func (e *Encoder) WriteTo(wr io.Writer, t *BuildTree) error {
	bs, err := e.Encode(t)
	if err != nil {
		return err
	}

	_, err = io.Copy(wr, bytes.NewReader(bs))
	if err != nil {
		return err
	}

	return nil
}

// getChildNodesSizeTotal returns the size in bytes of all encoded child nodes.
func (e *Encoder) getChildNodesSizeTotal(data []byte, childNodes map[rune]*BuildTreeNode) int {
	// get rune size in bytes first
	var runelentotal int
	for r := range childNodes {
		runelentotal += utf8.RuneLen(r)
	}
	// get the non-variable size of the entries which is:
	// child node count * (pointer size + flag byte)
	statictotal := len(childNodes) * (int(data[1]) + 1)

	return statictotal + runelentotal
}

// encodeEdges encodes the edges going out of node into bytes which are appended
// to data. The modified byte slice is returned.
func (e *Encoder) encodeEdges(node *BuildTreeNode, data []byte) []byte {
	// We want deterministic output for testing purposes,
	// so we need to order the keys of the edges map.
	edgeKeys := sortEdgeKeys(node)

	for i := 0; i < len(edgeKeys); i++ {
		currune := edgeKeys[i]
		child := node.Edges[currune]
		runelen := utf8.RuneLen(currune)

		var flags byte
		if child.final {
			flags |= endOfWord
		}
		if i == len(edgeKeys)-1 {
			flags |= endOfNode // end of node (last child outgoing from this node)
		}
		// encode rune length in the bits after the first two least significant ones
		lenmask := byte(runelen) << 2
		flags |= lenmask

		// the whole entry fits in here. The size is: length of rune + flag + pointer size
		entryBytes := make([]byte, runelen+1+int(data[1]))
		ret := copy(entryBytes, append([]byte{flags}, []byte(string(currune))...))
		if ret != runelen+1 {
			fmt.Printf("Could not copy UTF-8 bytes + flag. Wanted to copy %d, but only copied %d\n", runelen, ret)
			return nil
		}

		// If bytePos is 0, we haven't encoded this edge yet
		if child.bytePos == 0 {
			if len(child.Edges) > 0 {
				child.bytePos = e.curOffset
				childrenlength := e.getChildNodesSizeTotal(data, child.Edges)
				e.curOffset += childrenlength
			}
			e.queue = append(e.queue, child)
		}

		pointer := child.bytePos

		switch int(data[1]) {
		case 2:
			binary.BigEndian.PutUint16(entryBytes[runelen+1:], uint16(pointer))
		case 4:
			binary.BigEndian.PutUint32(entryBytes[runelen+1:], uint32(pointer))
		case 8:
			binary.BigEndian.PutUint64(entryBytes[runelen+1:], uint64(pointer))
		}

		data = append(data, entryBytes...)
	}

	return data
}

// sortEdgeKeys returns a sorted list of the keys
// of the map containing outgoing edges.
func sortEdgeKeys(node *BuildTreeNode) []rune {
	edgeKeys := make(runeSlice, 0, len(node.Edges))
	for char := range node.Edges {
		edgeKeys = append(edgeKeys, char)
	}
	sort.Sort(edgeKeys)
	return []rune(edgeKeys)
}

type runeSlice []rune

func (s runeSlice) Len() int           { return len(s) }
func (s runeSlice) Less(i, j int) bool { return s[i] < s[j] }
func (s runeSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

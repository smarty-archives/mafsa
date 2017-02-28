package mafsa

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"sort"
)

// Encoder is a type which can encode a BuildTree into a byte slice
// which can be written to a file.
type Encoder struct {
	queue   []*BuildTreeNode
	counter int
	wordBuf []byte
}

// Encode serializes a BuildTree t into a byte slice.
func (e *Encoder) Encode(t *BuildTree) ([]byte, error) {
	var buffer bytes.Buffer
	err := e.WriteTo(&buffer, t)
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// WriteTo encodes and saves the BuildTree to a io.Writer.
func (e *Encoder) WriteTo(wr io.Writer, t *BuildTree) error {
	bwr := bufio.NewWriter(wr)
	defer bwr.Flush()

	e.queue = []*BuildTreeNode{}
	e.counter = len(t.Root.Edges) + 1

	// First "word" (fixed-length entry) is a null entry
	// that specifies the file format:
	// First byte indicates the flag scheme (basically a file format verison number)
	// Second byte is word length in bytes (at least 4)
	// Third byte is char length in bytes
	// Fourth byte is pointer length in bytes
	//   Note: Word length (the first byte)
	//   must be exactly Second byte + 1 (flags) + Fourth byte

	pointerLen := 4
	maxRuneLen := characterLen(t.maxRune)
	wordLen := 1 + maxRuneLen + pointerLen

	// preallocate a buffer we can reuse while building words
	e.wordBuf = make([]byte, wordLen)

	e.wordBuf[0] = 0x1
	e.wordBuf[1] = byte(wordLen)
	e.wordBuf[2] = byte(maxRuneLen)
	e.wordBuf[3] = byte(pointerLen)

	// Any leftover bytes in this first word are zero
	for i := 4; i < wordLen; i++ {
		e.wordBuf[i] = 0x00
	}
	_, err := bwr.Write(e.wordBuf)
	if err != nil {
		return err
	}

	err = e.encodeEdges(t.Root, bwr, pointerLen, maxRuneLen)
	if err != nil {
		return err
	}

	for len(e.queue) > 0 {
		// Pop first item off the queue
		top := e.queue[0]
		e.queue = e.queue[1:]

		// Recursively marshal child nodes
		err = e.encodeEdges(top, bwr, pointerLen, maxRuneLen)
		if err != nil {
			return err
		}
	}

	return nil
}

// encodeEdges encodes the edges going out of node into bytes which are appended
// to data. The modified byte slice is returned.
func (e *Encoder) encodeEdges(node *BuildTreeNode, bw *bufio.Writer, pointerLen, runeLen int) error {
	// We want deterministic output for testing purposes,
	// so we need to order the keys of the edges map.
	edgeKeys := sortEdgeKeys(node)

	for i := 0; i < len(edgeKeys); i++ {
		child := node.Edges[edgeKeys[i]]
		encodeCharacter(edgeKeys[i], runeLen, e.wordBuf)

		var flags byte
		if child.final {
			flags |= 0x01 // end of word
		}
		if i == len(edgeKeys)-1 {
			flags |= 0x02 // end of node (last child outgoing from this node)
		}
		e.wordBuf[runeLen] = flags

		// If bytePos is 0, we haven't encoded this edge yet
		if child.bytePos == 0 {
			if len(child.Edges) > 0 {
				child.bytePos = e.counter
				e.counter += len(child.Edges)
			}
			e.queue = append(e.queue, child)
		}

		pointer := child.bytePos
		switch pointerLen {
		case 2:
			binary.BigEndian.PutUint16(e.wordBuf[runeLen+1:], uint16(pointer))
		case 4:
			binary.BigEndian.PutUint32(e.wordBuf[runeLen+1:], uint32(pointer))
		case 8:
			binary.BigEndian.PutUint64(e.wordBuf[runeLen+1:], uint64(pointer))
		}

		_, err := bw.Write(e.wordBuf)
		if err != nil {
			return err
		}
	}

	return nil
}

func characterLen(r rune) int {
	if r <= math.MaxUint8 {
		return 1
	} else if r <= math.MaxUint16 {
		return 2
	}
	return 4
}

func encodeCharacter(r rune, runeLen int, buf []byte) {
	if runeLen == 1 {
		buf[0] = byte(r)
	} else if runeLen == 2 {
		binary.BigEndian.PutUint16(buf, uint16(r))
	} else {
		binary.BigEndian.PutUint32(buf, uint32(r))
	}
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

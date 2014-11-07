MA-FSA for Go
=============

Package mafsa implements Minimal Acyclic Finite State Automata (MA-FSA) with Minimal Perfect Hashing (MPH). Basically, it's a set of strings that lets you test for membership, do spelling correction (fuzzy matching) and autocomplete, but with higher memory efficiency than a regular trie. With MPH, you can associate each entry in the tree with data from your own application.

In this package, MA-FSA is implemented by two types:

- BuildTree
- MinTree

A BuildTree is used to build data from scratch. Once all the elements have been inserted, the BuildTree can be serialized into a byte slice or written to a file directly. It can then be decoded into a MinTree, which uses significantly less memory. MinTrees are read-only, but this greatly improves space efficiency.


## Tutorial

Create a BuildTree and insert your items in lexicographical order. Be sure to call `Finish()` when you're done.

```go
bt := mafsa.New()
bt.Insert("cities") // an error will be returned if input is < last input
bt.Insert("city")
bt.Insert("pities")
bt.Insert("pity")
bt.Finish()
```

The tree is now compressed to a minimum number of nodes and is ready to be saved.

```go
err := bt.Save("filename")
if err != nil {
    log.Fatal("Could not save data to file:", err)
}
```

In your production application, then, you can read the file into a MinTree directly:

```go
mt, err := mafsa.Load("filename")
if err != nil {
    log.Fatal("Could not load data from file:", err)
}
```

The `mt` variable is a `*MinTree` which has the same data as the original BuildTree, but without all the extra "scaffolding" that was required for adding new elements. In our testing, we were able to store over 8 million phrases (average length 24, much longer than words in a typical dictionary) in as little as 2 GB on a 64-bit system.

The package provides some basic read mechanisms.

```go
// See if a string is a member of the set
fmt.Println("Does tree contain 'cities'?", mt.Contains("cities"))
fmt.Println("Does tree contain 'pitiful'?", mt.Contains("pitiful"))

// You can traverse down to a certain node, if it exists
fmt.Printf("'y' node is at: %p", mt.Traverse([]rune("city")))

// To traverse the tree and get the number of elements inserted
// before the prefix specified
node, idx := mt.IndexedTraverse([]rune("pit"))
fmt.Println("Index number for 'pit': %d", idx)
```

To associate entries in the tree with data from your own application, create a slice with the data in the same order as the elements were inserted into the tree:

```go
myData := []string{
    "The plural of city",
    "Noun; a large town",
    "The state of pitying",
    "A feeling of sorrow and compassion",
}
```

The index number returned with `IndexedTraverse()` (usually minus 1) will get you to the element in your slice if you traverse directly to a final node:

```go
node, i := mt.IndexedTraverse([]rune("pities"))
if node != nil && node.Final {
    fmt.Println(myData[i-1])
}
```

If you do `IndexedTraverse()` directly to a word in the tree, you must -1 because that method returns the number of elements in the tree before those that *start* with the specified prefix, which is non-inclusive with the node the method landed on.

There are many ways to apply MA-FSA with minimal perfect hashing, so the package only provides the basic utilities. Along with `Traverse()` and `IndexedTraverse()`, the edges of each node are exported so you may conduct your own traversals according to your needs.
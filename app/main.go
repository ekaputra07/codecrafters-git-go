package main

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
)

// Usage: your_program.sh <command> <arg1> <arg2> ...
func main() {
	// You can use print statements as follows for debugging, they'll be visible when running tests.
	// fmt.Fprintf(os.Stderr, "Logs from your program will appear here!\n")

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: mygit <command> [<args>...]\n")
		os.Exit(1)
	}

	switch command := os.Args[1]; command {
	case "init":
		cmdInit()

	case "cat-file":
		data := cmdCatFile(os.Args[3])
		fmt.Printf("%s", data)

	case "hash-object":
		hash := cmdHashObject(os.Args[3])
		fmt.Printf("%x", hash)

	case "ls-tree":
		// support optional --name-only flag
		args := os.Args[2:]
		if len(args) == 1 {
			cmdLsTree(args[0], false)
		} else {
			nameOnlyIndex := slices.Index(args, "--name-only")
			if nameOnlyIndex == 0 {
				cmdLsTree(args[1], true)
			} else {
				cmdLsTree(args[0], true)
			}
		}

	case "write-tree":
		hash := writeTree(".")
		fmt.Printf("%x", hash)

	case "commit-tree":
		treeSHA := os.Args[2]
		parentSHA := os.Args[4]
		message := os.Args[6]
		hash := cmdCommitTree(treeSHA, parentSHA, message)
		fmt.Printf("%x", hash)

	default:
		fmt.Fprintf(os.Stderr, "Unknown command %s\n", command)
		os.Exit(1)
	}
}

func cmdInit() {
	for _, dir := range []string{".git", ".git/objects", ".git/refs"} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating directory: %s\n", err)
		}
	}

	headFileContents := []byte("ref: refs/heads/main\n")
	if err := os.WriteFile(".git/HEAD", headFileContents, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file: %s\n", err)
	}

	fmt.Println("Initialized git directory")
}

func cmdCatFile(hash string) []byte {
	path := ".git/objects/" + hash[:2] + "/" + hash[2:]
	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	z, err := zlib.NewReader(f)
	if err != nil {
		panic(err)
	}
	defer z.Close()

	data, err := io.ReadAll(z)
	if err != nil {
		panic(err)
	}

	// print data after nullByteIndex
	nullByteIndex := bytes.IndexByte(data, 0)
	return data[nullByteIndex+1:]
}

func cmdHashObject(path string) [20]byte {
	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}

	data, err := io.ReadAll(f)
	if err != nil {
		panic(err)
	}

	// blob <size>\x00<data>
	bytes := fmt.Appendf([]byte("blob "), "%v\x00", len(data))
	bytes = append(bytes, data...)

	// the hash
	hash := sha1.Sum(bytes)
	hex := fmt.Sprintf("%x", hash) // to HEX

	// compress and write to file
	odir := ".git/objects/" + hex[:2]
	opath := odir + "/" + hex[2:]

	// - create dir
	if err := os.MkdirAll(odir, 0755); err != nil {
		panic(err)
	}

	// - create file
	ofile, err := os.Create(opath)
	if err != nil {
		fmt.Printf("failed to create file: %s", opath)
		panic(err)
	}
	defer ofile.Close()

	// - compress
	w := zlib.NewWriter(ofile)
	defer w.Close()

	_, err = w.Write(bytes)
	if err != nil {
		panic(err)
	}
	return hash
}

type treeEntry struct {
	mode string
	kind string
	name string
	hash [20]byte
}

func (tr treeEntry) String() string {
	return fmt.Sprintf("%s %s %x\t%s", tr.mode, tr.kind, tr.hash, tr.name)
}

func (tr treeEntry) NameOnly() string {
	return tr.name
}

// cmdLsTree lists the contents of a tree object given its hash.
// it supports the --name-only flag to print only the names of the entries.
func cmdLsTree(hash string, nameOnly bool) {
	path := ".git/objects/" + hash[:2] + "/" + hash[2:]

	// open object file
	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	// decompress object file
	z, err := zlib.NewReader(f)
	if err != nil {
		panic(err)
	}
	defer z.Close()

	// read all data
	data, err := io.ReadAll(z)
	if err != nil {
		panic(err)
	}

	// parse tree entries
	// data format:
	// tree <size>\0<mode1> <name1>\0<20_byte_sha><mode2> <name2>\0<20_byte_sha>...
	nullByteIndex := bytes.IndexByte(data, 0)
	// header := data[:nullByteIndex] // header unsed for now
	content := data[nullByteIndex+1:]

	// loop through content to extract mode/name and sha, create treeEntry and print
	cursor := 0
	for {
		// find next null byte
		nextNullByteIndex := bytes.IndexByte(content[cursor:], 0)
		if nextNullByteIndex == -1 {
			// no more null bytes
			break
		}

		// extract mode and name part
		modeNamePart := content[cursor : cursor+nextNullByteIndex]
		modeName := bytes.Split(modeNamePart, []byte(" "))
		mode := string(modeName[0])
		name := string(modeName[1])

		// extract next 20 bytes (sha)
		shaStart := cursor + nextNullByteIndex + 1
		shaEnd := shaStart + 20
		shaPart := content[shaStart:shaEnd]

		// move cursor past null byte
		cursor = shaEnd

		// create tree entry
		e := treeEntry{
			mode: mode,
			name: name,
			hash: [20]byte(shaPart),
		}
		if mode == "40000" {
			e.kind = "tree"
		} else {
			e.kind = "blob"
		}

		if nameOnly {
			fmt.Println(e.NameOnly())
		} else {
			fmt.Println(e.String())
		}
	}
}

func writeTree(baseDir string) [20]byte {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		panic(err)
	}

	treeEntries := []treeEntry{}

	for _, entry := range entries {
		// skip .git directory
		if entry.IsDir() {
			if entry.Name() == ".git" {
				continue
			}
			// recursively write tree for subdirectory
			hash := writeTree(baseDir + "/" + entry.Name())
			treeEntries = append(treeEntries, treeEntry{
				mode: "40000",
				kind: "tree",
				name: entry.Name(),
				hash: hash,
			})
		} else {
			hash := cmdHashObject(baseDir + "/" + entry.Name())
			treeEntries = append(treeEntries, treeEntry{
				mode: "100644",
				kind: "blob",
				name: entry.Name(),
				hash: hash,
			})
		}
	}

	// order entries by name
	slices.SortFunc(treeEntries, func(a, b treeEntry) int {
		return strings.Compare(a.name, b.name)
	})

	// build tree object content
	var body bytes.Buffer
	for _, entry := range treeEntries {
		// format: <mode1> <name1>\0<20_byte_sha><mode2> <name2>\0<20_byte_sha>
		body.WriteString(fmt.Sprintf("%s %s", entry.mode, entry.name))
		body.WriteByte(0)
		body.Write(entry.hash[:])
	}
	// tree <size>\0<body>
	var header bytes.Buffer
	header.WriteString("tree ")
	header.WriteString(fmt.Sprintf("%d", body.Len()))
	header.WriteByte(0)

	fullObject := append(header.Bytes(), body.Bytes()...)

	// hash the tree object
	hash := sha1.Sum(fullObject)
	hex := fmt.Sprintf("%x", hash) // to HEX

	// compress and write to file
	odir := ".git/objects/" + hex[:2]
	opath := odir + "/" + hex[2:]

	// - create dir
	if err := os.MkdirAll(odir, 0755); err != nil {
		panic(err)
	}

	// - create file
	ofile, err := os.Create(opath)
	if err != nil {
		fmt.Printf("failed to create file: %s", opath)
		panic(err)
	}
	defer ofile.Close()

	// - compress
	w := zlib.NewWriter(ofile)
	defer w.Close()

	_, err = w.Write(fullObject)
	if err != nil {
		panic(err)
	}
	return hash
}

func cmdCommitTree(treeSHA, parentSHA, message string) [20]byte {
	// build commit object content
	var body bytes.Buffer
	body.WriteString(fmt.Sprintf("tree %s\n", treeSHA))
	body.WriteString(fmt.Sprintf("parent %s\n", parentSHA))
	body.WriteString("author eka <eka@example.com> 946684800 +0000\n")
	body.WriteString("commiter eka <eka@example.com> 946684800 +0000\n")
	body.WriteString("\n" + message)

	// commit <size>\0<body>
	var header bytes.Buffer
	header.WriteString("commit ")
	header.WriteString(fmt.Sprintf("%d", body.Len()))
	header.WriteByte(0)

	fullObject := append(header.Bytes(), body.Bytes()...)

	// hash the commit object
	hash := sha1.Sum(fullObject)
	hex := fmt.Sprintf("%x", hash) // to HEX

	// compress and write to file
	odir := ".git/objects/" + hex[:2]
	opath := odir + "/" + hex[2:]

	// - create dir
	if err := os.MkdirAll(odir, 0755); err != nil {
		panic(err)
	}

	// - create file
	ofile, err := os.Create(opath)
	if err != nil {
		fmt.Printf("failed to create file: %s", opath)
		panic(err)
	}
	defer ofile.Close()

	// - compress
	w := zlib.NewWriter(ofile)
	defer w.Close()

	_, err = w.Write(fullObject)
	if err != nil {
		panic(err)
	}
	return hash
}

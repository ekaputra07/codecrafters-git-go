package main

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"slices"
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
		cmdCatFile(os.Args[3])
	case "hash-object":
		cmdHashObject(os.Args[3])
	case "ls-tree":
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

func cmdCatFile(hash string) {
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
	fmt.Printf("%s", data[nullByteIndex+1:])
}

func cmdHashObject(path string) {
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
	fmt.Printf("%x", hash)
}

type treeEntry struct {
	mode string
	kind string
	name string
	hash string
}

func (tr treeEntry) String() string {
	return fmt.Sprintf("%s %s %s\t%s", tr.mode, tr.kind, tr.hash, tr.name)
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

		// append part before null byte
		modeNamePart := content[cursor : cursor+nextNullByteIndex]
		modeName := bytes.Split(modeNamePart, []byte(" "))
		mode := string(modeName[0])
		name := string(modeName[1])

		// append next 20 bytes (sha)
		shaStart := cursor + nextNullByteIndex + 1
		shaEnd := shaStart + 20
		shaPart := content[shaStart:shaEnd]

		// move cursor past null byte
		cursor = shaEnd

		// create tree entry
		e := treeEntry{
			mode: mode,
			name: name,
			hash: fmt.Sprintf("%x", shaPart),
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

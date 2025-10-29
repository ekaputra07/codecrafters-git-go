package main

import (
	"bytes"
	"compress/zlib"
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
	compressed, err := os.ReadFile(path)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	// decompress
	b := bytes.NewReader(compressed)
	r, err := zlib.NewReader(b)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	defer r.Close()

	uncompressed, err := io.ReadAll(r)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	// The object file format is:
	// <type> <size>\0<content>
	// blob 14\0Hello, World!
	nullByte := slices.Index(uncompressed, 0)
	// size, _ := strconv.Atoi(string(uncompressed[5:nullByte]))
	// fmt.Println("size bytes:", size)

	data := uncompressed[nullByte+1:]
	fmt.Printf("%s", data)
}

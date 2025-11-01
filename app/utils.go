package main

import (
	"compress/zlib"
	"fmt"
	"os"
)

func writeObject(hash string, data []byte) error {
	// compress and write to file
	odir := ".git/objects/" + hash[:2]
	opath := odir + "/" + hash[2:]

	// - create dir
	if err := os.MkdirAll(odir, 0755); err != nil {
		return err
	}

	// - create file
	ofile, err := os.Create(opath)
	if err != nil {
		fmt.Printf("failed to create file: %s", opath)
		return err
	}
	defer ofile.Close()

	// - compress
	w := zlib.NewWriter(ofile)
	defer w.Close()

	_, err = w.Write(data)
	if err != nil {
		return err
	}
	return nil
}

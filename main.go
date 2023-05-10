package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/linuxhenhao/qnap_decrypt/lib"
)

func main() {
	var in, out, key string
	flag.StringVar(&in, "i", "", "input file path")
	flag.StringVar(&out, "o", "", "output file path")
	flag.StringVar(&key, "k", "", "password")
	flag.Parse()
	if len(in) == 0 || len(out) == 0 || len(key) == 0 {
		fmt.Printf("Usage: %s -i input_file -o output_path -k password\n", os.Args[0])
		return
	}
	is, err := lib.IsOpensslFile(in)
	if err != nil {
		fmt.Printf("err=%v\n", err)
		return
	}
	if !is {
		fmt.Printf("%s is not openssl encrypted file\n", in)
		return
	}
	if err = lib.DecipherOpensslFile(in, out, key); err != nil {
		fmt.Printf("decryption err: %v\n", err)
	}
	return
}

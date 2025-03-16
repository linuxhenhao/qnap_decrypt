package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/linuxhenhao/qnap_decrypt/lib"
)

func main() {
	var in, out, key string
	var bufferSize int
	flag.StringVar(&in, "i", "", "input file path")
	flag.StringVar(&out, "o", "", "output file path")
	flag.StringVar(&key, "k", "", "password")
	flag.IntVar(&bufferSize, "b", lib.DefaultBufferSize, "buffer size in bytes (default: 64KB)")
	flag.Parse()
	if len(in) == 0 || len(out) == 0 || len(key) == 0 {
		fmt.Printf("Usage: %s -i input_file -o output_path -k password [-b buffer_size]\n", os.Args[0])
		return
	}
	f, err := os.Open(in)
	if err != nil {
		fmt.Printf("open file error: %v\n", err)
		return
	}
	defer f.Close()

	// 创建一个BufferedReader实例，避免在多个函数中重复创建
	bufferedReader := lib.CreateBufferedReader(f, bufferSize)

	// 使用BufferedReader检查文件是否为OpenSSL加密文件
	is, err := lib.IsOpensslReader(bufferedReader)
	if err != nil {
		fmt.Printf("err=%v\n", err)
		return
	}
	if !is {
		fmt.Printf("%s is not openssl encrypted file\n", in)
		return
	}

	// 使用BufferedReader的Seek方法重置位置，避免重新创建实例
	// 这样可以复用已创建的BufferedReader实例
	bufferedReader.Seek(0, 0)

	if err = lib.DecipherOpensslReader(f, bufferedReader, out, key, bufferSize); err != nil {
		fmt.Printf("decryption err: %v\n", err)
	}
}

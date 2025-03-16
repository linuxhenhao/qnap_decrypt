package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/linuxhenhao/qnap_decrypt/lib"
)

func main() {
	var in, out, key string
	var bufferSize, workerCount int
	flag.StringVar(&in, "i", "", "input file path or directory")
	flag.StringVar(&out, "o", "", "output file path or directory")
	flag.StringVar(&key, "k", "", "password")
	flag.IntVar(&bufferSize, "b", lib.DefaultBufferSize, "buffer size in bytes (default: 64KB)")
	flag.IntVar(&workerCount, "w", 4, "worker count for concurrent processing (default: 4)")
	flag.Parse()
	if len(in) == 0 || len(out) == 0 || len(key) == 0 {
		fmt.Printf("Usage: %s -i input_path -o output_path -k password [-b buffer_size] [-w worker_count]\n", os.Args[0])
		return
	}

	// 使用worker池处理文件或目录
	err := lib.ProcessPath(in, out, key, workerCount, bufferSize)
	if err != nil {
		fmt.Printf("处理失败: %v\n", err)
	}
}

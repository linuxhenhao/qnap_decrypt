package lib

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"time"
)

// 这里移除了WorkerPool相关的结构体和方法，改为直接使用顺序处理方式

// ProcessPath 处理指定路径（文件或目录）
func ProcessPath(srcPath, destPath, password string, workerCount, bufferSize int) error {
	// 获取源路径的文件信息
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("获取源路径信息失败: %w", err)
	}

	// 声明状态变量
	var state *ProcessState

	// 根据源路径类型处理
	if srcInfo.IsDir() {
		// 只有处理目录时才加载或创建处理状态
		state, err = LoadState(destPath)
		if err != nil {
			return fmt.Errorf("加载状态失败: %w", err)
		}

		// 对于目录，使用顺序处理方式
		err = processDirectorySequential(srcPath, destPath, password, bufferSize, state)
		if err != nil {
			fmt.Printf("处理目录时发生错误: %v\n", err)
			return err
		}
	} else {
		// 对于单个文件，直接使用顺序处理方式，不再使用工作池
		// 确保目标目录存在
		destDir := filepath.Dir(destPath)
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return fmt.Errorf("创建目录失败 %s: %w", destDir, err)
		}
		// 处理单个文件
		processFileSequential(srcPath, destPath, password, bufferSize)
		return nil
	}

	return nil
}

// processDirectory函数已移除，因为它依赖于已删除的WorkerPool结构体

// PipelineContext 流水线处理上下文
type PipelineContext struct {
	wg         sync.WaitGroup
	InputChan  chan *FileData
	OutputChan chan *FileData
	ErrorChan  chan error
	Password   string
	BufferSize int
	State      *ProcessState
	DestDir    string
}

// FileData 文件数据块结构
type FileData struct {
	Path     string
	DestDir  string
	RelPath  string
	DataChan chan []byte
}

// processDirectoryPipelined 流水线方式处理目录
func processDirectorySequential(srcDir, destDir, password string, bufferSize int, state *ProcessState) error {
	ctx := &PipelineContext{
		InputChan:  make(chan *FileData, 4),   // 最多有四个文件在等待处理，不需要提前读取太多
		OutputChan: make(chan *FileData, 100), //  这里可以大一点，因为是由输入决定的
		ErrorChan:  make(chan error, 100),     // 这里无所谓
		Password:   password,
		BufferSize: bufferSize,
		State:      state,
		DestDir:    destDir,
	}

	// 启动三个阶段的工作协程
	ctx.wg.Add(3)
	go fileReader(ctx, srcDir)
	go dataDecryptor(ctx)
	go fileWriter(ctx)

	// 错误收集协程
	go func() {
		for err := range ctx.ErrorChan {
			fmt.Printf("Error: %s\n", err.Error())
		}
	}()

	ctx.wg.Wait()
	close(ctx.ErrorChan)
	return nil
}

func readFile(ctx *PipelineContext, f *os.File, path, relPath string) {
	isFirst := true
	fileData := &FileData{
		Path:    path,
		DestDir: ctx.DestDir,
		RelPath: relPath,
	}
	start := time.Now()
	// readFile 中的修改
	defer func() {
	    fmt.Printf("file %s: started at %s, end at %s, cost %s\n", path, 
	        start.Format("2006-01-02 15:04:05.000"),
	        time.Now().Format("2006-01-02 15:04:05.000"),
	        time.Since(start))
	    }()
	
	// dataDecryptor 中的修改
	        fmt.Printf("decrypt %s: started at %s, end at %s, cost %s\n", 
	            fileData.Path, 
	            start.Format("2006-01-02 15:04:05.000"),
	            time.Now().Format("2006-01-02 15:04:05.000"),
	            time.Since(start))
	
	// fileWriter 中的修改
	        fmt.Printf("write %s: started at %s, end at %s, cost %s\n", 
	            fileData.Path,
	            start.Format("2006-01-02 15:04:05.000"),
	            time.Now().Format("2006-01-02 15:04:05.000"),
	            time.Since(start))
	for {
		buf := make([]byte, ctx.BufferSize)
		isFinal := false
		n, err := f.Read(buf)
		if errors.Is(err, io.EOF) {
			isFinal = true
		}
		if err != nil && !errors.Is(err, io.EOF) {
			ctx.ErrorChan <- fmt.Errorf("read file %s failed: %w", path, err)
			return
		}
		buf = buf[:n]

		if isFirst {
			isFirst = false
			if !IsOpensslEncrypted(buf) {
				ctx.ErrorChan <- fmt.Errorf("file %s is not encrypted", path)
				return
			}
			fileData.DataChan = make(chan []byte, 100)
			ctx.InputChan <- fileData
		}
		if len(buf) > 0 {
			// 最后一次读取，EOF，n=0，如果发一次数据后再 Close，会导致接收方无法正确处理
			fileData.DataChan <- buf
		}
		if isFinal && fileData.DataChan != nil {
			close(fileData.DataChan)
			return
		}
	}
}

// sortFilesByInode 按inode排序文件
// 不会包含文件夹
func sortFilesByInode(srcDir string) []string {
	// 实现inode排序逻辑
	type fileLoc struct {
		Path string
		Ino  uint64
	}
	files := []fileLoc{}
	filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			stat := info.Sys().(*syscall.Stat_t)
			files = append(files, fileLoc{
				Path: path,
				Ino:  stat.Ino,
			})
		}
		return nil
	})
	// 按inode升序排序
	sort.Slice(files, func(i, j int) bool {
		return files[i].Ino < files[j].Ino
	})
	out := make([]string, len(files))
	for _, f := range files {
		out = append(out, f.Path)
	}
	return out
}

// fileReader 文件读取阶段
func fileReader(ctx *PipelineContext, srcDir string) {
	defer func() {
		close(ctx.InputChan)
		ctx.wg.Done()
	}()
	sortedFiles := sortFilesByInode(srcDir)
	for _, path := range sortedFiles {
		// 不会有文件夹
		rel, _ := filepath.Rel(srcDir, path)
		if ctx.State.IsProcessed(rel) {
			continue
		}
		f, err := os.Open(path)
		if err != nil {
			ctx.ErrorChan <- fmt.Errorf("open file %s failed: %w", path, err)
			continue
		}
		defer f.Close()
		readFile(ctx, f, path, rel)
	}

}

func singleFileReader(ctx *PipelineContext, srcPath string, relPath string) {
	defer func() {
		close(ctx.InputChan)
		ctx.wg.Done()
	}()
	f, err := os.Open(srcPath)
	if err != nil {
		ctx.ErrorChan <- fmt.Errorf("open file %s failed: %w", srcPath, err)
		return
	}
	defer f.Close()
	readFile(ctx, f, srcPath, relPath)
}

// dataDecryptor 数据解密阶段
func dataDecryptor(ctx *PipelineContext) {
	defer func() {
		close(ctx.OutputChan)
		ctx.wg.Done()
	}()
	for fileData := range ctx.InputChan {
		start := time.Now()
		// 流式解密数据
		outData := &FileData{
			Path:     fileData.Path,
			DestDir:  fileData.DestDir,
			RelPath:  fileData.RelPath,
			DataChan: make(chan []byte, 100),
		}
		ctx.OutputChan <- outData
		if err := DecipherOpensslReader(fileData.DataChan, outData.DataChan, ctx.Password, ctx.BufferSize); err != nil {
			ctx.ErrorChan <- err
			close(outData.DataChan)
			continue
		}
		// 正常处理
		close(outData.DataChan)
		fmt.Printf("decrypt %s: started at %s, end at %s, cost %s\n", 
			fileData.Path, 
			start.Format("2006-01-02 15:04:05"),
			time.Now().Format("2006-01-02 15:04:05"),
			time.Since(start))
	}
}

// fileWriter 文件写入阶段
func fileWriter(ctx *PipelineContext) {
	defer ctx.wg.Done()
	for fileData := range ctx.OutputChan {
		start := time.Now()
		destPath := filepath.Join(ctx.DestDir, fileData.RelPath)
		destDir := filepath.Dir(destPath)
		if err := os.MkdirAll(destDir, 0755); err != nil {
			ctx.ErrorChan <- err
			continue
		}
		isFailed := false
		for data := range fileData.DataChan {
			if err := os.WriteFile(destPath, data, 0644); err != nil {
				ctx.ErrorChan <- fmt.Errorf("file %s write decrypted file failed, %w", fileData.Path, err)
				isFailed = true
				break
			}
		}
		if isFailed {
			continue
		}
		if ctx.State != nil {
			ctx.State.MarkProcessed(fileData.RelPath)
			if err := ctx.State.SaveState(ctx.DestDir); err != nil {
				ctx.ErrorChan <- fmt.Errorf("file %s SaveState failed, %w", fileData.Path, err)
			}
		}
		fmt.Printf("write %s: started at %s, end at %s, cost %s\n", 
			fileData.Path,
			start.Format("2006-01-02 15:04:05"),
			time.Now().Format("2006-01-02 15:04:05"),
			time.Since(start))
	}
}

// processFileSequential 顺序处理单个文件
// 保留原顺序处理函数用于单个文件处理
func processFileSequential(srcPath, destPath, password string, bufferSize int) {
	ctx := &PipelineContext{
		InputChan:  make(chan *FileData, 100),
		OutputChan: make(chan *FileData, 100),
		ErrorChan:  make(chan error, 100),
		Password:   password,
		BufferSize: bufferSize,
		State:      nil,
		DestDir:    filepath.Dir(destPath),
	}
	ctx.wg.Add(3)
	go singleFileReader(ctx, srcPath, filepath.Base(destPath))
	go dataDecryptor(ctx)
	go fileWriter(ctx)

	go func() {
		for err := range ctx.ErrorChan {
			fmt.Printf("Error: %s\n", err.Error())
		}
	}()

	ctx.wg.Wait()
	close(ctx.ErrorChan)
}

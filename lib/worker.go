package lib

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// WorkerPool 表示一个工作池，用于并发处理文件解密任务
type WorkerPool struct {
	WorkerCount int            // 工作协程数量
	Password    string         // 解密密码
	BufferSize  int            // 缓冲区大小
	jobs        chan Job       // 任务通道
	wg          sync.WaitGroup // 等待组，用于等待所有工作协程完成
	ErrorChan   chan error     // 错误通道，用于收集错误信息
	State       *ProcessState  // 处理状态，用于记录已处理的文件
	DestDir     string         // 目标目录，用于保存状态文件
}

// Job 表示一个解密任务
type Job struct {
	SrcPath  string // 源文件路径
	DestPath string // 目标文件路径
	RelPath  string // 相对路径，用于状态记录
}

// NewWorkerPool 创建一个新的工作池
func NewWorkerPool(workerCount int, password string, bufferSize int, destDir string, state *ProcessState) *WorkerPool {
	return &WorkerPool{
		WorkerCount: workerCount,
		Password:    password,
		BufferSize:  bufferSize,
		jobs:        make(chan Job),
		ErrorChan:   make(chan error, 100), // 缓冲通道，避免阻塞
		State:       state,
		DestDir:     destDir,
	}
}

// Start 启动工作池
func (wp *WorkerPool) Start() {
	// 启动指定数量的工作协程
	for i := 0; i < wp.WorkerCount; i++ {
		wp.wg.Add(1)
		go wp.worker()
	}
}

// worker 工作协程，不断从任务通道获取任务并处理
func (wp *WorkerPool) worker() {
	defer wp.wg.Done()

	for job := range wp.jobs {
		// 确保目标目录存在
		destDir := filepath.Dir(job.DestPath)
		if err := os.MkdirAll(destDir, 0755); err != nil {
			wp.ErrorChan <- fmt.Errorf("创建目录失败 %s: %w", destDir, err)
			continue
		}

		// 打开源文件
		f, err := os.Open(job.SrcPath)
		if err != nil {
			wp.ErrorChan <- fmt.Errorf("打开文件失败 %s: %w", job.SrcPath, err)
			continue
		}

		// 创建BufferedReader
		bufferedReader := CreateBufferedReader(f, wp.BufferSize)

		// 检查是否为OpenSSL加密文件
		is, err := IsOpensslReader(bufferedReader)
		if err != nil {
			f.Close()
			wp.ErrorChan <- fmt.Errorf("检查文件类型失败 %s: %w", job.SrcPath, err)
			continue
		}

		// 如果不是OpenSSL加密文件，跳过处理
		if !is {
			f.Close()
			continue
		}

		// 重置BufferedReader位置
		bufferedReader.Seek(0, 0)

		// 解密文件
		if err = DecipherOpensslReader(f, bufferedReader, job.DestPath, wp.Password, wp.BufferSize); err != nil {
			wp.ErrorChan <- fmt.Errorf("解密文件失败 %s: %w", job.SrcPath, err)
		} else {
			// 解密成功，只有在有状态对象且有相对路径时才标记为已处理
			if wp.State != nil && job.RelPath != "" {
				wp.State.MarkProcessed(job.RelPath)
				// 每成功处理一个文件就保存一次状态
				if err := wp.State.SaveState(wp.DestDir); err != nil {
					wp.ErrorChan <- fmt.Errorf("保存状态失败: %w", err)
				}
			}
		}

		// 关闭文件
		f.Close()
	}
}

// AddJob 添加一个解密任务到工作池
func (wp *WorkerPool) AddJob(srcPath, destPath string, relPath string) {
	wp.jobs <- Job{SrcPath: srcPath, DestPath: destPath, RelPath: relPath}
}

// Wait 等待所有任务完成并关闭通道
func (wp *WorkerPool) Wait() {
	// 关闭任务通道，表示不再添加新任务
	close(wp.jobs)
	// 等待所有工作协程完成
	wp.wg.Wait()
	// 关闭错误通道
	close(wp.ErrorChan)
}

// ProcessPath 处理指定路径（文件或目录）
func ProcessPath(srcPath, destPath, password string, workerCount, bufferSize int) error {
	// 获取源路径的文件信息
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("获取源路径信息失败: %w", err)
	}

	// 声明状态变量
	var state *ProcessState

	// 只有处理目录时才加载或创建处理状态
	if srcInfo.IsDir() {
		state, err = LoadState(destPath)
		if err != nil {
			return fmt.Errorf("加载状态失败: %w", err)
		}
	}

	// 创建工作池
	pool := NewWorkerPool(workerCount, password, bufferSize, destPath, state)
	// 启动工作池
	pool.Start()

	// 根据源路径类型处理
	if srcInfo.IsDir() {
		// 处理目录
		err = processDirectory(srcPath, destPath, pool)
		if err != nil {
			fmt.Printf("处理目录时发生错误: %v\n", err)
		}
	} else {
		// 处理单个文件，无需关心重复处理问题
		pool.AddJob(srcPath, destPath, "")
	}

	// 等待所有任务完成
	pool.Wait()

	// 收集并返回错误信息
	var errs []error
	for err := range pool.ErrorChan {
		errs = append(errs, err)
	}

	// 如果有错误，返回第一个错误
	if len(errs) > 0 {
		return errs[0]
	}

	return nil
}

// processDirectory 递归处理目录
func processDirectory(srcDir, destDir string, pool *WorkerPool) error {
	// 确保目标目录存在
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("创建目标目录失败 %s: %w", destDir, err)
	}

	// 遍历源目录
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 计算相对路径
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return fmt.Errorf("计算相对路径失败: %w", err)
		}

		// 构建目标路径
		destPath := filepath.Join(destDir, rel)

		// 如果是目录，创建对应的目标目录
		if info.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		// 检查文件是否已处理
		if !pool.State.IsProcessed(rel) {
			// 如果是文件且未处理，添加到任务队列
			pool.AddJob(path, destPath, rel)
		} else {
			fmt.Printf("跳过已处理的文件: %s\n", path)
		}
		return nil
	})
}

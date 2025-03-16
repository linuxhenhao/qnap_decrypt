package lib

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// 状态文件名
const StateFileName = ".qnap_decrypt_state.txt"

// ProcessState 表示处理状态
type ProcessState struct {
	ProcessedFiles map[string]bool // 已处理文件的相对路径映射
	mutex          sync.RWMutex    // 用于并发安全的读写锁
	file           *os.File        // 状态文件句柄
	writer         *bufio.Writer   // 缓冲写入器
}

// NewProcessState 创建一个新的处理状态对象
func NewProcessState() *ProcessState {
	return &ProcessState{
		ProcessedFiles: make(map[string]bool),
	}
}

// LoadState 从状态文件加载处理状态
func LoadState(destDir string) (*ProcessState, error) {
	statePath := filepath.Join(destDir, StateFileName)
	state := NewProcessState()

	// 如果状态文件不存在，创建新文件
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		file, err := os.OpenFile(statePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("创建状态文件失败: %w", err)
		}
		state.file = file
		state.writer = bufio.NewWriter(file)
		return state, nil
	}

	// 读取现有状态文件
	file, err := os.Open(statePath)
	if err != nil {
		return nil, fmt.Errorf("打开状态文件失败: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		state.ProcessedFiles[scanner.Text()] = true
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取状态文件失败: %w", err)
	}

	// 打开文件用于追加写入
	file, err = os.OpenFile(statePath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("打开状态文件失败: %w", err)
	}
	state.file = file
	state.writer = bufio.NewWriter(file)

	return state, nil
}

// IsProcessed 检查文件是否已处理
func (ps *ProcessState) IsProcessed(relPath string) bool {
	ps.mutex.RLock()
	defer ps.mutex.RUnlock()

	_, exists := ps.ProcessedFiles[relPath]
	return exists
}

// MarkProcessed 标记文件为已处理
func (ps *ProcessState) MarkProcessed(relPath string) {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	if !ps.ProcessedFiles[relPath] {
		ps.ProcessedFiles[relPath] = true
		if _, err := ps.writer.WriteString(relPath + "\n"); err != nil {
			fmt.Printf("写入状态文件失败: %v\n", err)
			return
		}
		if err := ps.writer.Flush(); err != nil {
			fmt.Printf("刷新状态文件失败: %v\n", err)
			return
		}
	}
}

// Close 关闭状态文件
func (ps *ProcessState) Close() error {
	if ps.file != nil {
		return ps.file.Close()
	}
	return nil
}

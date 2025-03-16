package lib

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// 状态文件名
const StateFileName = ".qnap_decrypt_state.json"

// ProcessState 表示处理状态
type ProcessState struct {
	ProcessedFiles map[string]bool `json:"processed_files"` // 已处理文件的相对路径映射
	mutex          sync.RWMutex    // 用于并发安全的读写锁
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

	// 如果状态文件不存在，返回一个新的状态对象
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		return NewProcessState(), nil
	}

	// 读取状态文件
	data, err := os.ReadFile(statePath)
	if err != nil {
		return nil, fmt.Errorf("读取状态文件失败: %w", err)
	}

	// 解析JSON
	state := NewProcessState()
	if err := json.Unmarshal(data, &state.ProcessedFiles); err != nil {
		return nil, fmt.Errorf("解析状态文件失败: %w", err)
	}

	return state, nil
}

// SaveState 保存处理状态到状态文件
func (ps *ProcessState) SaveState(destDir string) error {
	ps.mutex.RLock()
	defer ps.mutex.RUnlock()

	statePath := filepath.Join(destDir, StateFileName)

	// 将状态对象序列化为JSON
	data, err := json.MarshalIndent(ps.ProcessedFiles, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化状态失败: %w", err)
	}

	// 写入状态文件
	if err := os.WriteFile(statePath, data, 0644); err != nil {
		return fmt.Errorf("写入状态文件失败: %w", err)
	}

	return nil
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

	ps.ProcessedFiles[relPath] = true
}

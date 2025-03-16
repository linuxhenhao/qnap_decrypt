package lib

import (
	"io"
)

// BufferedReader 是一个实现io.Reader接口的缓冲读取器
// 它封装了底层的io.Reader并提供缓冲功能，减少系统调用次数
type BufferedReader struct {
	reader     io.Reader // 底层读取器
	buffer     []byte    // 缓冲区
	bufferSize int       // 缓冲区大小
	rPos       int       // 当前读取位置
	bufferEnd  int       // 缓冲区有效数据结束位置
}

// ChannelReader 实现从channel读取数据的缓冲读取器
// 封装数据通道并提供缓冲读取功能
type ChannelReader struct {
	dataChan      <-chan []byte // 数据输入通道
	currentBuffer []byte        // 当前数据缓冲区
	currentPos    int           // 当前读取位置
}

// NewChannelReader 创建新的通道读取器
func NewChannelReader(dataChan <-chan []byte) *ChannelReader {
	return &ChannelReader{
		dataChan: dataChan,
	}
}

// Read 实现io.Reader接口
// 当缓冲区数据不足时自动从channel获取新数据块
func (cr *ChannelReader) Read(p []byte) (n int, err error) {
	// 当前缓冲区已读完
	if cr.currentPos >= len(cr.currentBuffer) {
		select {
		case data, ok := <-cr.dataChan:
			if !ok {
				return 0, io.EOF // 通道已关闭
			}
			cr.currentBuffer = data
			cr.currentPos = 0
		}
	}

	// 计算可读取字节数
	available := len(cr.currentBuffer) - cr.currentPos
	toCopy := len(p)
	if toCopy > available {
		toCopy = available
	}

	// 复制数据到目标缓冲区
	copy(p, cr.currentBuffer[cr.currentPos:cr.currentPos+toCopy])
	cr.currentPos += toCopy
	return toCopy, nil
}

// ChannelBufferedWriter 是一个实现io.Writer接口的缓冲写入器
// 封装数据通道并提供缓冲功能，减少通道写入次数
type ChannelBufferedWriter struct {
	dataChan   chan<- []byte // 数据输出通道
	buffer     []byte        // 缓冲区
	bufferSize int           // 缓冲区大小
	wPos       int           // 当前写入位置
}

// NewChannelBufferedWriter 创建新的通道缓冲写入器
func NewChannelBufferedWriter(dataChan chan<- []byte, bufferSize int) *ChannelBufferedWriter {
	return &ChannelBufferedWriter{
		dataChan:   dataChan,
		buffer:     make([]byte, bufferSize),
		bufferSize: bufferSize,
		wPos:       0,
	}
}

// Write 实现io.Writer接口
// 将数据写入缓冲区，当缓冲区满时，将数据发送到数据通道
func (bw *ChannelBufferedWriter) Write(p []byte) (n int, err error) {
	total := len(p)
	remaining := total

	for remaining > 0 {
		availableSpace := bw.bufferSize - bw.wPos

		if availableSpace == 0 {
			if err = bw.Flush(); err != nil {
				return total - remaining, err
			}
			availableSpace = bw.bufferSize
		}

		toWrite := remaining
		if toWrite > availableSpace {
			toWrite = availableSpace
		}

		copy(bw.buffer[bw.wPos:], p[total-remaining:total-remaining+toWrite])
		bw.wPos += toWrite
		remaining -= toWrite
	}

	return total, nil
}

// Flush 将缓冲区中的数据发送到数据通道
func (bw *ChannelBufferedWriter) Flush() error {
	if bw.wPos == 0 {
		return nil
	}

	bw.dataChan <- append([]byte{}, bw.buffer[:bw.wPos]...)
	bw.wPos = 0
	return nil
}

// Close 关闭缓冲写入器，刷新所有缓冲数据
func (bw *ChannelBufferedWriter) Close() error {
	return bw.Flush()
}

package lib

import (
	"fmt"
	"io"
	"os"
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

// BufferedWriter 是一个实现io.Writer接口的缓冲写入器
// 它封装了底层的io.Writer并提供缓冲功能，减少系统调用次数
type BufferedWriter struct {
	writer     io.Writer // 底层写入器
	buffer     []byte    // 缓冲区
	bufferSize int       // 缓冲区大小
	wPos       int       // 当前写入位置
}

// NewBufferedReader 创建一个新的缓冲读取器
// bufferSize 指定缓冲区大小
func NewBufferedReader(reader io.Reader, bufferSize int) *BufferedReader {
	return &BufferedReader{
		reader:     reader,
		buffer:     make([]byte, bufferSize),
		bufferSize: bufferSize,
		rPos:       0,
		bufferEnd:  0,
	}
}

// NewBufferedWriter 创建一个新的缓冲写入器
// bufferSize 指定缓冲区大小
func NewBufferedWriter(writer io.Writer, bufferSize int) *BufferedWriter {
	return &BufferedWriter{
		writer:     writer,
		buffer:     make([]byte, bufferSize),
		bufferSize: bufferSize,
		wPos:       0,
	}
}

// 为了向后兼容，保留原有的BufferedReadWriter结构体和构造函数
// BufferedReadWriter 是一个同时实现io.Reader和io.Writer接口的缓冲读写器
// 它封装了底层的io.Reader和io.Writer并提供缓冲功能，减少系统调用次数
type BufferedReadWriter struct {
	reader     io.Reader // 底层读取器
	writer     io.Writer // 底层写入器
	buffer     []byte    // 缓冲区
	bufferSize int       // 缓冲区大小
	// 读取相关字段
	rPos      int // 当前读取位置
	bufferEnd int // 缓冲区有效数据结束位置
	// 写入相关字段
	wPos int // 当前写入位置
}

// NewBufferedReadWriter 创建一个新的缓冲读写器
// bufferSize 指定缓冲区大小，读写共用同一个bufferSize参数
// 注意：此函数仅为向后兼容保留，建议使用NewBufferedReader或NewBufferedWriter
func NewBufferedReadWriter(reader io.Reader, writer io.Writer, bufferSize int) *BufferedReadWriter {
	return &BufferedReadWriter{
		reader:     reader,
		writer:     writer,
		buffer:     make([]byte, bufferSize),
		bufferSize: bufferSize,
		rPos:       0,
		bufferEnd:  0,
		wPos:       0,
	}
}

// Read 实现io.Reader接口
// 从缓冲区读取数据，如果缓冲区数据不足，则从底层reader读取更多数据到缓冲区
func (br *BufferedReader) Read(p []byte) (n int, err error) {
	// 如果缓冲区已经读完，从底层reader填充缓冲区
	if br.rPos >= br.bufferEnd {
		br.rPos = 0
		br.bufferEnd, err = br.reader.Read(br.buffer)
		if err != nil {
			return 0, err
		}
		// 如果读不到数据，返回EOF
		if br.bufferEnd == 0 {
			return 0, io.EOF
		}
	}

	// 计算可以读取的字节数
	available := br.bufferEnd - br.rPos
	toRead := len(p)
	if toRead > available {
		toRead = available
	}

	// 复制数据到目标切片
	copy(p, br.buffer[br.rPos:br.rPos+toRead])
	br.rPos += toRead

	return toRead, nil
}

// Read 实现io.Reader接口 (为了向后兼容)
// 从缓冲区读取数据，如果缓冲区数据不足，则从底层reader读取更多数据到缓冲区
func (brw *BufferedReadWriter) Read(p []byte) (n int, err error) {
	// 如果缓冲区已经读完，从底层reader填充缓冲区
	if brw.rPos >= brw.bufferEnd {
		brw.rPos = 0
		brw.bufferEnd, err = brw.reader.Read(brw.buffer)
		if err != nil {
			return 0, err
		}
		// 如果读不到数据，返回EOF
		if brw.bufferEnd == 0 {
			return 0, io.EOF
		}
	}

	// 计算可以读取的字节数
	available := brw.bufferEnd - brw.rPos
	toRead := len(p)
	if toRead > available {
		toRead = available
	}

	// 复制数据到目标切片
	copy(p, brw.buffer[brw.rPos:brw.rPos+toRead])
	brw.rPos += toRead

	return toRead, nil
}

// Write 实现io.Writer接口
// 将数据写入缓冲区，当缓冲区满时，将数据刷新到底层writer
func (bw *BufferedWriter) Write(p []byte) (n int, err error) {
	total := len(p)
	remaining := total

	for remaining > 0 {
		// 计算当前可以写入缓冲区的字节数
		availableSpace := bw.bufferSize - bw.wPos

		// 如果缓冲区已满，刷新缓冲区
		if availableSpace == 0 {
			if err = bw.Flush(); err != nil {
				return total - remaining, err
			}
			availableSpace = bw.bufferSize
		}

		// 计算本次写入的字节数
		toWrite := remaining
		if toWrite > availableSpace {
			toWrite = availableSpace
		}

		// 复制数据到缓冲区
		copy(bw.buffer[bw.wPos:], p[total-remaining:total-remaining+toWrite])
		bw.wPos += toWrite
		remaining -= toWrite
	}

	return total, nil
}

// Write 实现io.Writer接口 (为了向后兼容)
// 将数据写入缓冲区，当缓冲区满时，将数据刷新到底层writer
func (brw *BufferedReadWriter) Write(p []byte) (n int, err error) {
	total := len(p)
	remaining := total

	for remaining > 0 {
		// 计算当前可以写入缓冲区的字节数
		availableSpace := brw.bufferSize - brw.wPos

		// 如果缓冲区已满，刷新缓冲区
		if availableSpace == 0 {
			if err = brw.Flush(); err != nil {
				return total - remaining, err
			}
			availableSpace = brw.bufferSize
		}

		// 计算本次写入的字节数
		toWrite := remaining
		if toWrite > availableSpace {
			toWrite = availableSpace
		}

		// 复制数据到缓冲区
		copy(brw.buffer[brw.wPos:], p[total-remaining:total-remaining+toWrite])
		brw.wPos += toWrite
		remaining -= toWrite
	}

	return total, nil
}

// Flush 将缓冲区中的数据写入底层writer
func (bw *BufferedWriter) Flush() error {
	if bw.wPos == 0 {
		return nil
	}

	_, err := bw.writer.Write(bw.buffer[:bw.wPos])
	if err != nil {
		return err
	}

	bw.wPos = 0
	return nil
}

// Close 关闭缓冲写入器，刷新所有缓冲数据
func (bw *BufferedWriter) Close() error {
	return bw.Flush()
}

// Flush 将缓冲区中的数据写入底层writer (为了向后兼容)
func (brw *BufferedReadWriter) Flush() error {
	if brw.wPos == 0 {
		return nil
	}

	_, err := brw.writer.Write(brw.buffer[:brw.wPos])
	if err != nil {
		return err
	}

	brw.wPos = 0
	return nil
}

// Close 关闭缓冲读写器，刷新所有缓冲数据 (为了向后兼容)
func (brw *BufferedReadWriter) Close() error {
	return brw.Flush()
}

// Seek 实现类似io.Seeker接口的功能，重置BufferedReader的内部状态
// 并将底层文件指针定位到指定位置
// 注意：这个方法要求底层reader是*os.File类型
func (br *BufferedReader) Seek(offset int64, whence int) (int64, error) {
	// 检查底层reader是否为*os.File类型
	file, ok := br.reader.(*os.File)
	if !ok {
		return 0, fmt.Errorf("底层reader不是*os.File类型，无法执行Seek操作")
	}

	// 重置BufferedReader的内部状态
	br.rPos = 0
	br.bufferEnd = 0

	// 调用底层文件的Seek方法
	return file.Seek(offset, whence)
}

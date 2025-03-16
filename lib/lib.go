package lib

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"fmt"
	"io"
	"os"

	"github.com/pkg/errors"
	"github.com/scritch007/go-tools/crypto"
)

const (
	AES_KEY_LENGTH = 256
	BYTE_SIZE      = 8
	BLOCK_SIZE     = 16
	ITERATIONS     = 1
)

// CreateBufferedReader 创建一个BufferedReader，用于读取文件
// 这个函数可以在多个地方复用同一个BufferedReader，避免重复创建
func CreateBufferedReader(f *os.File, bufferSize ...int) *BufferedReader {
	bufSize := DefaultBufferSize
	if len(bufferSize) > 0 && bufferSize[0] > 0 {
		bufSize = bufferSize[0]
	}
	return NewBufferedReader(f, bufSize)
}

func IsOpensslFile(f *os.File) (bool, error) {
	// 创建一个BufferedReader来读取文件头部
	bufferedReader := CreateBufferedReader(f)
	return IsOpensslReader(bufferedReader)
}

// IsOpensslReader 检查给定的BufferedReader是否指向一个OpenSSL加密文件
// 这个函数不会改变底层文件的位置，因为它只读取前8个字节
func IsOpensslReader(reader *BufferedReader) (bool, error) {
	buf := make([]byte, SALT_SIZE)
	_, err := reader.Read(buf)
	if err != nil {
		return false, err
	}
	return string(buf) == SALT_STR, nil
}

// DecipherOpensslFile 使用文件指针解密OpenSSL加密文件
// 这是为了向后兼容保留的函数
func DecipherOpensslFile(f *os.File, dstPath, password string, bufferSize ...int) error {
	// 复用CreateBufferedReader函数创建BufferedReader
	bufferedReader := CreateBufferedReader(f, bufferSize...)
	return DecipherOpensslReader(f, bufferedReader, dstPath, password, bufferSize...)
}

// DecipherOpensslReader 使用已创建的BufferedReader解密OpenSSL加密文件
// 这个函数可以避免重复创建BufferedReader，提高性能
func DecipherOpensslReader(f *os.File, reader *BufferedReader, dstPath, password string, bufferSize ...int) error {
	fname := "DecipherOpensslReader"
	salt := make([]byte, SALT_SIZE)
	f.Seek(SALT_SIZE, 0)
	f.Read(salt)
	out, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("%s: %w", fname, err)
	}
	defer out.Close()
	hash := md5.New()
	// IV Size is equal to blockSize, blockSize can be gotten by ase.NewCipher(key) block.BlockSize()
	key, iv := crypto.EVP_BytesToKey(AES_KEY_LENGTH/BYTE_SIZE, BLOCK_SIZE, hash, salt, []byte(password), ITERATIONS)
	// 使用已创建的BufferedReader和新创建的BufferedWriter
	bufSize := DefaultBufferSize
	if len(bufferSize) > 0 && bufferSize[0] > 0 {
		bufSize = bufferSize[0]
	}
	bufferedWriter := NewBufferedWriter(out, bufSize)
	defer bufferedWriter.Close() // 确保所有缓冲数据都被写入
	err = DecryptStreamToStream(reader, bufferedWriter, key, iv)
	return errors.Wrap(err, fname)
}

func DecryptStreamToStream(input io.Reader, out io.Writer, key, iv []byte) error {
	fname := "DecryptStreamToStream"
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("%s: %w", fname, err)
	}
	// blockSize need be multiple times of block.BlockSize()
	blockSize := block.BlockSize()
	inBuffer, outBuffer := make([]byte, blockSize), make([]byte, blockSize)
	ecb := cipher.NewCBCDecrypter(block, iv)
	first := true
	for {
		if _, err := input.Read(inBuffer); err == nil {
			// write last decrypted block data after next read
			if first {
				first = false
			} else {
				_, err = out.Write(outBuffer)
			}
			ecb.CryptBlocks(outBuffer, inBuffer)
			if err != nil {
				return errors.Wrap(err, fname+": out.Write")
			}
		} else {
			if err == io.EOF {
				out.Write(PKCS5Trimming(outBuffer))
				return nil
			}
			return errors.Wrap(err, fname+": input.Read")
		}
	}
}

func PKCS5Trimming(encrypt []byte) []byte {
	padding := encrypt[len(encrypt)-1]
	return encrypt[:len(encrypt)-int(padding)]
}

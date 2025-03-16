package lib

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"fmt"
	"io"

	"github.com/pkg/errors"
	"github.com/scritch007/go-tools/crypto"
)

const (
	AES_KEY_LENGTH = 256
	BYTE_SIZE      = 8
	BLOCK_SIZE     = 16
	ITERATIONS     = 1
)

func IsOpensslEncrypted(buf []byte) bool {
	// header 是 Salted__ + 8byte 的 salt，还有文件数据，所以长度大于 2*SALT_SIZE
	if len(buf) <= 2*SALT_SIZE {
		return false
	}
	return string(buf[:SALT_SIZE]) == SALT_STR
}

// DecipherOpensslReader 使用已创建的BufferedReader解密OpenSSL加密文件
// 这个函数可以避免重复创建BufferedReader，提高性能
func DecipherOpensslReader(dataChan <-chan []byte, outChan chan<- []byte, password string, bufferSize int) error {
	fname := "DecipherOpensslReader"
	reader := NewChannelReader(dataChan)
	saltMagic := make([]byte, SALT_SIZE)
	salt := make([]byte, SALT_SIZE)
	_, err := reader.Read(saltMagic)
	if err != nil {
		return fmt.Errorf("%s ReadSaltMagic: %w", fname, err)
	}
	if string(saltMagic) != SALT_STR {
		return fmt.Errorf("%s: not a openssl encrypted file", fname)
	}
	_, err = reader.Read(salt)
	if err != nil {
		return fmt.Errorf("%s ReadSalt: %w", fname, err)
	}
	hash := md5.New()
	// IV Size is equal to blockSize, blockSize can be gotten by ase.NewCipher(key) block.BlockSize()
	key, iv := crypto.EVP_BytesToKey(AES_KEY_LENGTH/BYTE_SIZE, BLOCK_SIZE, hash, salt, []byte(password), ITERATIONS)
	// 使用已创建的ChannelReader和新创建的BufferedWriter
	bufferedWriter := NewChannelBufferedWriter(outChan, bufferSize)
	defer bufferedWriter.Close()
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

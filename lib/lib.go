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

func IsOpensslFile(filepath string) (bool, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return false, fmt.Errorf("IsOpensslFile: %w", err)
	}
	defer f.Close()
	buf := make([]byte, SALT_SIZE)
	f.Read(buf)
	return string(buf) == SALT_STR, nil
}

func DecipherOpensslFile(filepath, dstPath, password string) error {
	fname := "DecipherOpensslFile"
	f, err := os.Open(filepath)
	if err != nil {
		return fmt.Errorf("%s: %w", fname, err)
	}
	defer f.Close()
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
	fmt.Println(key, len(key))
	fmt.Println(iv, len(iv))
	err = DecryptStreamToStream(f, out, key, iv)
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
	for {
		if _, err := input.Read(inBuffer); err == nil {
			// write last decrypted block data after next read
			_, err = out.Write(outBuffer)
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

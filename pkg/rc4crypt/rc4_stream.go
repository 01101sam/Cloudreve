package rc4crypt

import (
	"crypto/md5"
	"crypto/rc4"
	"errors"
	"io"

	"github.com/cloudreve/Cloudreve/v4/pkg/conf"
)

// saltKey generates the effective RC4 key using the base key and file-specific salt.
func saltKey(baseUserKey []byte, filePath string) []byte {
	// It's crucial that filePath is canonical and consistent for the same file.
	hasher := md5.New()
	hasher.Write(baseUserKey)      // Add base key first
	hasher.Write([]byte(filePath)) // Then add file path
	// Using the MD5 hash directly as the RC4 key. RC4 keys can be variable length.
	return hasher.Sum(nil)
}

// RC4StreamSeekReader provides seeking capabilities for RC4 encrypted streams
type RC4StreamSeekReader struct {
	underlyingFile io.ReadSeekCloser
	cipher         *rc4.Cipher
	baseKey        []byte
	filePath       string
	currentOffset  int64 // Current logical offset in the decrypted stream
	fileSize       int64 // Total size of the (encrypted) file
}

// NewRC4StreamSeekReader creates a new RC4 stream reader with seeking capabilities
func NewRC4StreamSeekReader(underlyingFile io.ReadSeekCloser, baseKey []byte, filePath string, fileSize int64) (*RC4StreamSeekReader, error) {
	if baseKey == nil || len(baseKey) == 0 { // No encryption
		return &RC4StreamSeekReader{ // Passthrough
			underlyingFile: underlyingFile,
			baseKey:        nil,
			fileSize:       fileSize,
		}, nil
	}

	// Initial seek to beginning and setup cipher for that
	if _, err := underlyingFile.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	effectiveKey := saltKey(baseKey, filePath)
	cipher, err := rc4.NewCipher(effectiveKey)
	if err != nil {
		return nil, err
	}

	return &RC4StreamSeekReader{
		underlyingFile: underlyingFile,
		cipher:         cipher,
		baseKey:        baseKey,
		filePath:       filePath,
		currentOffset:  0,
		fileSize:       fileSize,
	}, nil
}

// Read reads decrypted data from the stream
func (r *RC4StreamSeekReader) Read(p []byte) (n int, err error) {
	if r.cipher == nil { // Passthrough mode
		return r.underlyingFile.Read(p)
	}

	// Underlying file is already at the correct physical position due to Seek or prior Reads
	n, err = r.underlyingFile.Read(p)
	if n > 0 {
		r.cipher.XORKeyStream(p[:n], p[:n])
		r.currentOffset += int64(n)
	}
	return n, err
}

// Seek seeks to a position in the decrypted stream
func (r *RC4StreamSeekReader) Seek(offset int64, whence int) (int64, error) {
	if r.baseKey == nil || len(r.baseKey) == 0 { // Passthrough mode
		newOffset, err := r.underlyingFile.Seek(offset, whence)
		if err == nil {
			r.currentOffset = newOffset
		}
		return newOffset, err
	}

	var newAbsOffset int64
	switch whence {
	case io.SeekStart:
		newAbsOffset = offset
	case io.SeekCurrent:
		newAbsOffset = r.currentOffset + offset
	case io.SeekEnd:
		newAbsOffset = r.fileSize + offset
	default:
		return r.currentOffset, io.ErrUnexpectedEOF
	}

	if newAbsOffset < 0 {
		return r.currentOffset, errors.New("rc4crypt.Seek: invalid offset")
	}

	// If seeking backwards or to the same position we're already at
	if newAbsOffset <= r.currentOffset {
		// Need to restart from beginning
		if _, err := r.underlyingFile.Seek(0, io.SeekStart); err != nil {
			return r.currentOffset, err
		}

		// Re-initialize cipher for the beginning of the file
		effectiveKey := saltKey(r.baseKey, r.filePath)
		var err error
		r.cipher, err = rc4.NewCipher(effectiveKey)
		if err != nil {
			return r.currentOffset, err
		}
		r.currentOffset = 0
	}

	// "Fast-forward" the cipher and the underlying file reader by reading and discarding bytes
	if newAbsOffset > r.currentOffset {
		toDiscard := newAbsOffset - r.currentOffset
		discarded, err := io.CopyN(io.Discard, r, toDiscard)
		if err != nil && err != io.EOF {
			return r.currentOffset, err
		}
		// r.currentOffset is updated by Read method during io.CopyN
		if discarded < toDiscard && err == io.EOF {
			// We hit EOF before reaching the target offset
			return r.currentOffset, io.EOF
		}
	}

	return r.currentOffset, nil
}

// Close closes the underlying file
func (r *RC4StreamSeekReader) Close() error {
	return r.underlyingFile.Close()
}

// RC4StreamWriter provides RC4 encryption for sequential writes
type RC4StreamWriter struct {
	underlyingWriter io.WriteCloser
	cipher           *rc4.Cipher
}

// NewRC4StreamWriter creates a new RC4 stream writer
func NewRC4StreamWriter(underlyingWriter io.WriteCloser, baseKey []byte, filePath string) (*RC4StreamWriter, error) {
	if baseKey == nil || len(baseKey) == 0 { // No encryption
		return &RC4StreamWriter{underlyingWriter: underlyingWriter, cipher: nil}, nil
	}

	effectiveKey := saltKey(baseKey, filePath)
	cipher, err := rc4.NewCipher(effectiveKey)
	if err != nil {
		return nil, err
	}
	return &RC4StreamWriter{underlyingWriter: underlyingWriter, cipher: cipher}, nil
}

// Write encrypts and writes data
func (w *RC4StreamWriter) Write(p []byte) (n int, err error) {
	if w.cipher == nil { // Passthrough
		return w.underlyingWriter.Write(p)
	}

	// Create a temporary buffer for encrypted data
	encryptedP := make([]byte, len(p))
	w.cipher.XORKeyStream(encryptedP, p)
	return w.underlyingWriter.Write(encryptedP)
}

// Close closes the underlying writer
func (w *RC4StreamWriter) Close() error {
	return w.underlyingWriter.Close()
}

// Discard advances the cipher state by n bytes without producing any output.
// This is useful when appending to an already encrypted file at an offset
// other than zero so that the RC4 keystream remains aligned with the
// existing ciphertext.
func (w *RC4StreamWriter) Discard(n int64) {
	if w.cipher == nil || n <= 0 {
		return // Passthrough mode or nothing to discard
	}

	// Reuse a small fixed buffer to fast-forward the cipher.
	buf := make([]byte, 4096)
	remaining := n
	for remaining > 0 {
		chunk := int64(len(buf))
		if remaining < chunk {
			chunk = remaining
		}

		// XORKeyStream advances the cipher state even when src == dst.
		w.cipher.XORKeyStream(buf[:chunk], buf[:chunk])
		remaining -= chunk
	}
}

// rc4Reader wraps an io.Reader with RC4 decryption
type rc4Reader struct {
	cipher *rc4.Cipher
	reader io.Reader
}

func (r *rc4Reader) Read(p []byte) (n int, err error) {
	n, err = r.reader.Read(p)
	if n > 0 {
		r.cipher.XORKeyStream(p[:n], p[:n])
	}
	return n, err
}

// rc4Writer wraps an io.Writer with RC4 encryption
type rc4Writer struct {
	cipher *rc4.Cipher
	writer io.Writer
}

func (w *rc4Writer) Write(p []byte) (n int, err error) {
	encrypted := make([]byte, len(p))
	w.cipher.XORKeyStream(encrypted, p)
	return w.writer.Write(encrypted)
}

// NewRC4Reader creates a reader that decrypts data on the fly
// This is a convenience function for when you don't need seeking
func NewRC4Reader(source io.Reader, filePath string) (io.Reader, error) {
	baseKey := conf.DecodedFileEncryptionKey
	if baseKey == nil || len(baseKey) == 0 {
		return source, nil // Passthrough
	}

	effectiveKey := saltKey(baseKey, filePath)
	cipher, err := rc4.NewCipher(effectiveKey)
	if err != nil {
		return nil, err
	}

	return &rc4Reader{cipher: cipher, reader: source}, nil
}

// NewRC4Writer creates a writer that encrypts data on the fly
// This is a convenience function for when you don't need close functionality
func NewRC4Writer(dest io.Writer, filePath string) (io.Writer, error) {
	baseKey := conf.DecodedFileEncryptionKey
	if baseKey == nil || len(baseKey) == 0 {
		return dest, nil // Passthrough
	}

	effectiveKey := saltKey(baseKey, filePath)
	cipher, err := rc4.NewCipher(effectiveKey)
	if err != nil {
		return nil, err
	}

	return &rc4Writer{cipher: cipher, writer: dest}, nil
}

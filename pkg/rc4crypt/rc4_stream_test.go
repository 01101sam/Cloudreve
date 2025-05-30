package rc4crypt

import (
	"bytes"
	"io"
	"testing"

	"github.com/cloudreve/Cloudreve/v4/pkg/conf"
)

func TestSaltKey(t *testing.T) {
	baseKey := []byte("test-key")
	filePath1 := "/path/to/file1.txt"
	filePath2 := "/path/to/file2.txt"

	key1 := saltKey(baseKey, filePath1)
	key2 := saltKey(baseKey, filePath2)

	// Keys should be different for different file paths
	if bytes.Equal(key1, key2) {
		t.Error("saltKey should generate different keys for different file paths")
	}

	// Same path should generate the same key
	key1Again := saltKey(baseKey, filePath1)
	if !bytes.Equal(key1, key1Again) {
		t.Error("saltKey should generate the same key for the same file path")
	}
}

func TestRC4StreamWriterAndReader(t *testing.T) {
	// Set up test encryption key
	testKey := []byte("test-encryption-key-12345")
	originalConf := conf.DecodedFileEncryptionKey
	defer func() {
		conf.DecodedFileEncryptionKey = originalConf
	}()
	conf.DecodedFileEncryptionKey = testKey

	testData := []byte("Hello, this is a test message for RC4 encryption!")
	filePath := "/test/file.txt"

	// Test encryption
	var encryptedBuf bytes.Buffer
	writer, err := NewRC4StreamWriter(&nopCloser{Writer: &encryptedBuf}, testKey, filePath)
	if err != nil {
		t.Fatalf("Failed to create RC4 writer: %v", err)
	}

	n, err := writer.Write(testData)
	if err != nil {
		t.Fatalf("Failed to write encrypted data: %v", err)
	}
	if n != len(testData) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(testData), n)
	}
	writer.Close()

	encryptedData := encryptedBuf.Bytes()

	// Verify data was encrypted (not equal to original)
	if bytes.Equal(encryptedData, testData) {
		t.Error("Data was not encrypted")
	}

	// Test decryption
	reader, err := NewRC4StreamSeekReader(
		&nopSeekCloser{ReadSeeker: bytes.NewReader(encryptedData)},
		testKey,
		filePath,
		int64(len(encryptedData)),
	)
	if err != nil {
		t.Fatalf("Failed to create RC4 reader: %v", err)
	}
	defer reader.Close()

	decryptedData, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("Failed to read decrypted data: %v", err)
	}

	// Verify decrypted data matches original
	if !bytes.Equal(decryptedData, testData) {
		t.Errorf("Decrypted data does not match original.\nOriginal: %s\nDecrypted: %s",
			string(testData), string(decryptedData))
	}
}

func TestRC4StreamSeekReader(t *testing.T) {
	testKey := []byte("test-encryption-key-12345")
	filePath := "/test/seekable.txt"
	testData := []byte("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz")

	// Encrypt the data first
	var encryptedBuf bytes.Buffer
	writer, _ := NewRC4StreamWriter(&nopCloser{Writer: &encryptedBuf}, testKey, filePath)
	writer.Write(testData)
	writer.Close()
	encryptedData := encryptedBuf.Bytes()

	// Create seek reader
	reader, err := NewRC4StreamSeekReader(
		&nopSeekCloser{ReadSeeker: bytes.NewReader(encryptedData)},
		testKey,
		filePath,
		int64(len(encryptedData)),
	)
	if err != nil {
		t.Fatalf("Failed to create RC4 seek reader: %v", err)
	}
	defer reader.Close()

	// Test seeking to different positions
	tests := []struct {
		name     string
		offset   int64
		whence   int
		expected string
		readLen  int
	}{
		{"Seek to start", 0, io.SeekStart, "0123456789", 10},
		{"Seek to middle", 10, io.SeekStart, "ABCDEFGHIJ", 10},
		{"Seek relative forward", 10, io.SeekCurrent, "UVWXYZabcd", 10},       // Current position is 20 after previous read
		{"Seek backward from current", -20, io.SeekCurrent, "KLMNOPQRST", 10}, // Current position is 30 after previous read, -20 = position 10
		{"Seek from end", -10, io.SeekEnd, "qrstuvwxyz", 10},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pos, err := reader.Seek(tc.offset, tc.whence)
			if err != nil {
				t.Fatalf("Seek failed: %v", err)
			}

			buf := make([]byte, tc.readLen)
			n, err := reader.Read(buf)
			if err != nil && err != io.EOF {
				t.Fatalf("Read failed: %v", err)
			}

			got := string(buf[:n])
			if n > 0 && got != tc.expected {
				t.Errorf("Expected %q, got %q at position %d", tc.expected, got, pos)
			}
		})
	}
}

func TestPassthroughMode(t *testing.T) {
	testData := []byte("This should pass through unchanged")
	filePath := "/test/passthrough.txt"

	// Test writer with nil key (passthrough mode)
	var buf bytes.Buffer
	writer, err := NewRC4StreamWriter(&nopCloser{Writer: &buf}, nil, filePath)
	if err != nil {
		t.Fatalf("Failed to create passthrough writer: %v", err)
	}

	writer.Write(testData)
	writer.Close()

	if !bytes.Equal(buf.Bytes(), testData) {
		t.Error("Passthrough writer should not modify data")
	}

	// Test reader with nil key (passthrough mode)
	reader, err := NewRC4StreamSeekReader(
		&nopSeekCloser{ReadSeeker: bytes.NewReader(testData)},
		nil,
		filePath,
		int64(len(testData)),
	)
	if err != nil {
		t.Fatalf("Failed to create passthrough reader: %v", err)
	}
	defer reader.Close()

	output, _ := io.ReadAll(reader)
	if !bytes.Equal(output, testData) {
		t.Error("Passthrough reader should not modify data")
	}
}

func TestConvenienceFunctions(t *testing.T) {
	// Set up test encryption key
	testKey := []byte("test-encryption-key-12345")
	originalConf := conf.DecodedFileEncryptionKey
	defer func() {
		conf.DecodedFileEncryptionKey = originalConf
	}()
	conf.DecodedFileEncryptionKey = testKey

	testData := []byte("Testing convenience functions")
	filePath := "/test/convenience.txt"

	// Test NewRC4Writer
	var encBuf bytes.Buffer
	encWriter, err := NewRC4Writer(&encBuf, filePath)
	if err != nil {
		t.Fatalf("NewRC4Writer failed: %v", err)
	}
	encWriter.Write(testData)

	// Test NewRC4Reader
	decReader, err := NewRC4Reader(bytes.NewReader(encBuf.Bytes()), filePath)
	if err != nil {
		t.Fatalf("NewRC4Reader failed: %v", err)
	}

	decrypted, _ := io.ReadAll(decReader)
	if !bytes.Equal(decrypted, testData) {
		t.Error("Convenience functions failed to encrypt/decrypt correctly")
	}
}

// Helper types for testing

type nopCloser struct {
	io.Writer
}

func (n *nopCloser) Close() error {
	return nil
}

type nopSeekCloser struct {
	io.ReadSeeker
}

func (n *nopSeekCloser) Close() error {
	return nil
}

package helpers_test

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"

	"github.com/fmartingr/games-screenshot-mananger/pkg/helpers"
)

const testfileContents = "testfile"

// TestCopyFileOk
func TestCopyFileOk(t *testing.T) {
	tmpfile, err := ioutil.TempFile("", "testfile")
	if err != nil {
		t.Fatal(err)
	}

	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(testfileContents)); err != nil {
		tmpfile.Close()
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	// Create and remove the file since the CopyFile does not overwrite
	tmpfileDest, err := ioutil.TempFile("", "testfile_dest")
	if err != nil {
		t.Fatal(err)
	}
	os.Remove(tmpfileDest.Name())

	bytesCopied, err := helpers.CopyFile(tmpfile.Name(), tmpfileDest.Name())
	if err != nil {
		t.Fatal(err)
	}
	tmpfileDest, err = os.Open(tmpfileDest.Name())
	if err != nil {
		t.Fatal(err)
	}

	tmpfileDestContents := make([]byte, 8)
	if _, err := tmpfileDest.Read(tmpfileDestContents); err != nil {
		t.Fatal(err)
	}

	if int64(len(testfileContents)) != bytesCopied {
		t.Errorf("Bytes copied not correct: %d (should be %d)", bytesCopied, int64(len(testfileContents)))
	}
}

// TestMD5FileOk
// Tests that the MD5File function works ok using a string as path
// and result is correct
func TestMD5FileOk(t *testing.T) {
	testfileMD5 := []byte("8bc944dbd052ef51652e70a5104492e3")
	tmpfile, err := ioutil.TempFile("", "testfile")
	if err != nil {
		t.Fatal(err)
	}

	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(testfileContents)); err != nil {
		tmpfile.Close()
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	resultMD5, err := helpers.Md5File(tmpfile.Name())
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Compare(resultMD5, testfileMD5) > 1 {
		t.Errorf("MD5 Missmatch: %s (should be %s)", string(resultMD5), testfileMD5)
	}
}

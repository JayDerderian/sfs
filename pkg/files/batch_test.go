package files

import (
	"path/filepath"
	"testing"

	"github.com/alecthomas/assert/v2"
)

func TestBatchLimit(t *testing.T) {
	d, err := MakeTmpDir(t, filepath.Join(GetTestingDir(), "tmp"))
	if err != nil {
		Fatal(t, err)
	}

	testFiles, err := MakeABunchOfTxtFiles(20)
	if err != nil {
		Fatal(t, err)
	}
	d.AddFiles(testFiles)

	// get total size of testFiles in testing dir
	b := NewBatch()
	b.Cap = 25000 // 25 kb batch capacity for testing

	var totalSize int64
	for _, testFile := range testFiles {
		totalSize += testFile.Size()
	}
	// make sure our test file content size is greater than our
	// established capacity. we want to check for left over files
	assert.True(t, totalSize > b.Cap)

	// add test files
	remTestFiles, _ := b.AddFiles(testFiles)
	assert.True(t, len(remTestFiles) < len(testFiles))

	if err := Clean(t, GetTestingDir()); err != nil {
		t.Fatal(err)
	}
}

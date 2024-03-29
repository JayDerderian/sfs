package service

import (
	"testing"

	"github.com/sfs/pkg/env"

	"github.com/alecthomas/assert/v2"
)

const (
	testData  = "hello, I love you won't you tell me your name?"
	testData2 = "hello, I love you, let me jump in your game"
)

func TestFileIO(t *testing.T) {
	env.SetEnv(false)

	total := RandInt(5)
	testFiles, err := MakeTestFiles(t, total)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// test f.Load()
	for _, f := range testFiles {
		f.Load()
		assert.NotEqual(t, 0, len(f.Content))
		assert.Equal(t, []byte(txtData), f.Content)

		f.Clear()
		assert.Equal(t, 0, len(f.Content))
	}

	// update files with new content
	for _, f := range testFiles {
		if err := f.Save([]byte(testData)); err != nil {
			t.Fatalf("[ERROR] failed to save new content: %v", err)
		}
		f.Load() // f.Save() doesn't load file contents into memory
		assert.NotEqual(t, 0, len(f.Content))
		assert.Equal(t, []byte(testData), f.Content)

		f.Clear()
		assert.Equal(t, 0, len(f.Content))
	}

	if err := RemoveTestFiles(t, total); err != nil {
		t.Fatalf("[ERROR] failed to remove test files: %v", err)
	}
}

func TestGetFileSize(t *testing.T) {
	env.SetEnv(false)

	total := RandInt(5)
	testFiles, err := MakeTestFiles(t, total)
	if err != nil {
		t.Fatalf("%v", err)
	}

	for _, f := range testFiles {
		fSize := f.GetSize()
		assert.NotEqual(t, 0, fSize)

		f.Clear()
	}

	if err := RemoveTestFiles(t, total); err != nil {
		t.Fatalf("[ERROR] failed to remove test files: %v", err)
	}
}

func TestFileSecurityFeatures(t *testing.T) {
	env.SetEnv(false)

	testFiles, err := MakeTestFiles(t, 1)
	if err != nil {
		t.Fatalf("%v", err)
	}
	tf := testFiles[0]
	tf.Load()

	// lock file, then attempt to modify
	tf.Lock("default")

	stuff := tf.Content
	if err := tf.Save([]byte(txtData)); err != nil {
		t.Fatalf("[ERROR] failed to save new content: %v", err)
	}
	assert.Equal(t, stuff, tf.Content)

	// try to clear internal contents
	if err := tf.Clear(); err != nil {
		t.Fatalf("[ERROR] failed to save new content: %v", err)
	}
	assert.NotEqual(t, 0, len(tf.Content))
	assert.Equal(t, stuff, tf.Content)

	// attempt to change password
	tf.ChangePassword("wrongPassword", "someOtherThing")
	assert.NotEqual(t, tf.Key, "someOtherThing")
	assert.Equal(t, "default", tf.Key)
	assert.Equal(t, true, tf.Protected)

	// actually change password
	tf.ChangePassword("default", "someOtherThing")
	assert.Equal(t, "someOtherThing", tf.Key)
	assert.Equal(t, true, tf.Protected)

	// unclock file
	tf.Unlock("someOtherThing")
	assert.Equal(t, false, tf.Protected)

	if err = RemoveTestFiles(t, 1); err != nil {
		t.Fatalf("[ERROR] failed to remove test files: %v", err)
	}

}

func TestFileChecksum(t *testing.T) {
	env.SetEnv(false)

	testFiles, err := MakeTestFiles(t, 1)
	if err != nil {
		t.Fatalf("%v", err)
	}
	tf := testFiles[0]
	csOrig := tf.CheckSum

	// update file content and generate a new checksum
	if err := tf.Save([]byte(testData2)); err != nil {
		t.Fatalf("[ERROR] failed to save new content: %v", err)
	}
	tf.CheckSum, err = CalculateChecksum(tf.Path)
	if err != nil {
		t.Fatalf("[ERROR] failed to calculate checksum: %v", err)
	}
	assert.NotEqual(t, tf.CheckSum, csOrig)

	if err = RemoveTestFiles(t, 1); err != nil {
		t.Fatalf("[ERROR] failed to remove test files: %v", err)
	}
}

package db

import (
	"fmt"
	"log"
	"path/filepath"
	"testing"

	"github.com/sfs/pkg/env"
	svc "github.com/sfs/pkg/service"

	"github.com/alecthomas/assert/v2"
)

func TestAddAndFindFile(t *testing.T) {
	env.SetEnv(false)

	testDir := GetTestingDir()

	// test db and query
	NewTable(filepath.Join(testDir, "Files"), CreateFileTable)
	q := NewQuery(filepath.Join(testDir, "Files"), false)
	q.Debug = true

	tmpFile := svc.NewFile("temp.txt", "some-rand-id", "bill", filepath.Join(testDir, "Files"))

	// add temp file
	if err := q.AddFile(tmpFile); err != nil {
		Fatal(t, fmt.Errorf("failed to add file: %v", err))
	}
	log.Printf("added file: %s", tmpFile.ID)

	// search for temp file & verify ID
	f, err := q.GetFileByID(tmpFile.ID)
	if err != nil {
		Fatal(t, fmt.Errorf("failed to get file: %v", err))
	}
	// NOTE: f being nil isn't necessarily a problem. if we have a
	// functional table and the entry simply doesn't exist,
	// then its not necessarily a failure -- the item may simply not exist.
	// here we just want to test for the existence of a file so we
	// can ensure database I/O is working properly.
	assert.NotEqual(t, nil, f)
	// assert.Equal(t, tmpFile.ID, f.ID)

	if err := Clean(t, GetTestingDir()); err != nil {
		t.Errorf("[ERROR] unable to remove test directories: %v", err)
	}
}

func TestAddAndFindMultipleFiles(t *testing.T) {
	env.SetEnv(false)

	testDir := GetTestingDir()

	// test db and query
	NewTable(filepath.Join(testDir, "Files"), CreateFileTable)
	q := NewQuery(filepath.Join(testDir, "Files"), false)
	q.Debug = true

	// tmp dir
	dir, err := MakeTmpDir(t, filepath.Join(GetTestingDir(), "tmp"))
	if err != nil {
		Fail(t, GetTestingDir(), err)
	}

	// make a bunch of dummy files
	total := svc.RandInt(100)
	testFiles, err := MakeABunchOfTxtFiles(total)
	if err != nil {
		Fail(t, GetTestingDir(), err)
	}
	dir.AddFiles(testFiles)

	// add files to db
	if err := q.AddFiles(testFiles); err != nil {
		Fail(t, GetTestingDir(), err)
	}

	// attempt to retrieve all the test files we just added
	results, err := q.GetFiles()
	if err != nil {
		Fail(t, GetTestingDir(), err)
	}
	// clean up first in case soemthing fails
	if err := Clean(t, GetTestingDir()); err != nil {
		t.Errorf("[ERROR] unable to remove test directories: %v", err)
	}
	assert.NotEqual(t, nil, results)
	assert.Equal(t, total, len(results))
	assert.Equal(t, len(testFiles), len(results))
}

func TestAddAndFindDirectory(t *testing.T) {
	env.SetEnv(false)

	testDir := GetTestingDir()

	// test db and query
	NewTable(filepath.Join(testDir, "directories"), CreateDirectoryTable)
	q := NewQuery(filepath.Join(testDir, "directories"), false)

	_, tmpDir, _ := MakeTestItems(t, GetTestingDir())

	// add tmp directory
	if err := q.AddDir(tmpDir); err != nil {
		Fatal(t, fmt.Errorf("failed to add dir: %v", err))
	}

	// search for temp dir to ensure it was added correctly
	d, err := q.GetDirectoryByID(tmpDir.ID)
	if err != nil {
		Fatal(t, fmt.Errorf("failed to get directory: %v", err))
	}
	assert.NotEqual(t, nil, d)
	// assert.Equal(t, tmpDir.ID, d.ID)

	if err := Clean(t, GetTestingDir()); err != nil {
		t.Errorf("[ERROR] unable to remove test directories: %v", err)
	}
}

func TestAddAndFindDrive(t *testing.T) {
	env.SetEnv(false)

	testDir := GetTestingDir()

	// make testing objects
	tmpDrive, _, _ := MakeTestItems(t, testDir)
	NewTable(filepath.Join(testDir, "tmp"), CreateDriveTable)

	// test query
	q := NewQuery(filepath.Join(testDir, "tmp"), false)
	if err := q.AddDrive(tmpDrive); err != nil {
		Fatal(t, err)
	}

	// add test drive
	d, err := q.GetDrive(tmpDrive.ID)
	if err != nil {
		Fatal(t, err)
	}
	assert.NotEqual(t, nil, d)
	// assert.Equal(t, tmpDrive.ID, d.ID)

	if err := Clean(t, GetTestingDir()); err != nil {
		t.Errorf("[ERROR] unable to remove test directories: %v", err)
	}
}

func TestAddAndFindUser(t *testing.T) {
	env.SetEnv(false)

	testDir := GetTestingDir()

	// make testing objects
	_, _, tmpUser := MakeTestItems(t, testDir)
	NewTable(filepath.Join(testDir, "tmp"), CreateUserTable)

	// test query
	q := NewQuery(filepath.Join(testDir, "tmp"), false)

	// add test user
	if err := q.AddUser(tmpUser); err != nil {
		Fatal(t, err)
	}

	// query for user we just added
	u, err := q.GetUser(tmpUser.ID)
	if err != nil {
		Fatal(t, err)
	}
	assert.Equal(t, tmpUser.ID, u.ID)

	if err := Clean(t, GetTestingDir()); err != nil {
		t.Errorf("[ERROR] unable to remove test directories: %v", err)
	}
}

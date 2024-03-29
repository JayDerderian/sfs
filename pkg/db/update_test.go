package db

import (
	"fmt"
	"log"
	"path/filepath"
	"testing"

	"github.com/alecthomas/assert/v2"
	"github.com/sfs/pkg/env"
	svc "github.com/sfs/pkg/service"
)

func TestCreateAndUpdateAFile(t *testing.T) {
	env.SetEnv(false)

	testDir := GetTestingDir()

	// test db and query
	NewTable(filepath.Join(testDir, "Files"), CreateFileTable)
	q := NewQuery(filepath.Join(testDir, "Files"), false)
	q.Debug = true

	tmpFile := svc.NewFile("temp.txt", "some-rand-id", "bill", filepath.Join(testDir, "Files"))

	// add temp file
	if err := q.AddFile(tmpFile); err != nil {
		t.Fatal(err)
	}

	tmpFile.Name = "some-doc.txt"

	if err := q.UpdateFile(tmpFile); err != nil {
		t.Fatal(err)
	}
	tmp, err := q.GetFileByID(tmpFile.ID)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, tmpFile.Name, tmp.Name)

	if err := Clean(t, GetTestingDir()); err != nil {
		t.Fatal(err)
	}
}

func TestCreateAndUpdateADirectory(t *testing.T) {
	env.SetEnv(false)

	testDir := GetTestingDir()

	// test db and query
	NewTable(filepath.Join(testDir, "Directories"), CreateDirectoryTable)
	q := NewQuery(filepath.Join(testDir, "Directories"), false)
	q.Debug = true

	tmpDir := svc.NewDirectory("tmp", "bill buttlicker", "some-rand-id", filepath.Join(testDir, "Directories"))

	// add temp directory
	if err := q.AddDir(tmpDir); err != nil {
		Fatal(t, err)
	}
	log.Printf("added directory: %s", tmpDir.ID)

	// update the directory
	tmpDir.Name = "pron"

	if err := q.UpdateDir(tmpDir); err != nil {
		Fatal(t, fmt.Errorf("failed to update directory: %v", err))
	}
	// pull the dir from the db and make sure the name is accurate
	d, err := q.GetDirectoryByID(tmpDir.ID)
	if err != nil {
		Fatal(t, err)
	}

	assert.Equal(t, tmpDir.Name, d.Name)

	if err := Clean(t, GetTestingDir()); err != nil {
		t.Fatal(err)
	}
}

func TestCreateAndUpdateADrive(t *testing.T) {
	env.SetEnv(false)

	testDir := GetTestingDir()

	NewTable(filepath.Join(testDir, "Drives"), CreateDriveTable)
	q := NewQuery(filepath.Join(testDir, "Drives"), false)
	q.Debug = true

	tmpDrive, _, _ := MakeTestItems(t, testDir)

	if err := q.AddDrive(tmpDrive); err != nil {
		Fatal(t, err)
	}

	tmpDrive.OwnerID = "some user"

	if err := q.UpdateDrive(tmpDrive); err != nil {
		Fatal(t, err)
	}

	d, err := q.GetDrive(tmpDrive.ID)
	if err != nil {
		Fatal(t, err)
	}
	assert.NotEqual(t, nil, d)
	assert.Equal(t, tmpDrive.OwnerID, "some user")
	assert.Equal(t, tmpDrive.OwnerID, d.OwnerID)

	if err := Clean(t, GetTestingDir()); err != nil {
		t.Fatal(err)
	}
}

func TestCreateAndUpdateAUser(t *testing.T) {
	testDir := GetTestingDir()

	NewTable(filepath.Join(testDir, "Users"), CreateUserTable)
	q := NewQuery(filepath.Join(testDir, "Users"), false)
	q.Debug = true

	_, _, tmpUser := MakeTestItems(t, testDir)

	if err := q.AddUser(tmpUser); err != nil {
		t.Fatal(err)
	}

	tmpUser.Name = "seymore butts"

	if err := q.UpdateUser(tmpUser); err != nil {
		t.Fatal(err)
	}

	u, err := q.GetUser(tmpUser.ID)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotEqual(t, nil, u)
	assert.Equal(t, tmpUser.Name, "seymore butts")
	assert.Equal(t, tmpUser.Name, u.Name)

	if err := Clean(t, GetTestingDir()); err != nil {
		t.Fatal(err)
	}
}

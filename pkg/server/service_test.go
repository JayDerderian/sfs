package server

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"
)

// --------- fixtures --------------------------------

func fakeServiceRoot() string {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("[ERROR] failed to get current working directory: %v", err)
	}
	return cwd
}

// -----------------------------------------------------

func TestSaveStateFile(t *testing.T) {
	svc := &Service{
		SvcRoot:   fakeServiceRoot(),
		StateFile: GetStateDir(),
	}
	if err := svc.SaveState(); err != nil {
		Fatal(t, err)
	}

	entries, err := os.ReadDir(GetStateDir())
	if err != nil {
		Fatal(t, err)
	}
	assert.NotEqual(t, 0, len(entries))
	assert.Equal(t, 1, len(entries))
	assert.True(t, strings.Contains(entries[0].Name(), "sfs-state") && strings.Contains(entries[0].Name(), ".json"))

	if err := Clean(t, GetStateDir()); err != nil {
		t.Errorf("[ERROR] unable to remove test directories: %v", err)
	}
}

func TestLoadServiceFromStateFile(t *testing.T) {
	svc := &Service{
		SvcRoot:   fakeServiceRoot(),
		StateFile: GetStateDir(),
	}
	if err := svc.SaveState(); err != nil {
		Fatal(t, err)
	}

	svc2, err := loadStateFile(svc.StateFile)
	if err != nil {
		Fatal(t, err)
	}

	// remove temp state file prior to checking so we don't accidentally
	// leave tmp files behind if an assert fails.
	if err := Clean(t, filepath.Join(fakeServiceRoot(), "state")); err != nil {
		t.Errorf("[ERROR] unable to remove test directories: %v", err)
	}

	assert.NotEqual(t, nil, svc2)
	assert.Equal(t, svc.SvcRoot, svc2.SvcRoot)
	assert.Equal(t, svc.StateFile, svc2.StateFile)
	assert.Equal(t, svc.UserDir, svc2.UserDir)
	assert.Equal(t, svc.DbDir, svc2.DbDir)
	assert.Equal(t, svc.Users, svc2.Users)
}

func TestLoadServiceFromStateFileAndDbs(t *testing.T) {
	testRoot := filepath.Join(GetTestingDir(), "tmp")
	testSvc, err := SvcInit(testRoot, true)
	if err != nil {
		Fatal(t, err)
	}

	// read service entries
	entries, err := os.ReadDir(testSvc.SvcRoot)
	if err != nil {
		t.Fatalf("%v", err)
	}
	assert.NotEqual(t, 0, len(entries))

	// load a new service instance from these resources
	svc, err := SvcLoad(testSvc.SvcRoot, true)
	if err != nil {
		Fatal(t, err)
	}
	assert.NotEqual(t, nil, svc)
	assert.Equal(t, testSvc.SvcRoot, svc.SvcRoot)
	assert.Equal(t, testSvc.StateFile, svc.StateFile)
	assert.Equal(t, testSvc.UserDir, svc.UserDir)
	assert.Equal(t, testSvc.DbDir, svc.DbDir)
	assert.Equal(t, testSvc.Users, svc.Users)

	if err := Clean(t, GetTestingDir()); err != nil {
		t.Errorf("[ERROR] unable to remove test directories: %v", err)
	}
}

func TestCreateNewService(t *testing.T) {
	testRoot := filepath.Join(GetTestingDir(), "tmp")
	testSvc, err := SvcInit(testRoot, true)
	if err != nil {
		Fatal(t, err)
	}

	entries, err := os.ReadDir(testSvc.SvcRoot)
	if err != nil {
		t.Fatalf("%v", err)
	}
	assert.NotEqual(t, 0, len(entries))

	// check that dbs and state subdirectores aren't empty and
	// the expected files exist. users can be empty since we don't
	// have any yet.
	for _, e := range entries {
		switch e.Name() {
		case "state":
			if isEmpty(filepath.Join(testRoot, "state")) {
				Fatal(t, fmt.Errorf("missing state file"))
			}
		case "dbs":
			if isEmpty(filepath.Join(testRoot, "dbs")) {
				Fatal(t, fmt.Errorf("missing state file"))
			}
		default: // skip anything else for now
			continue
		}
	}
	// clean up
	if err := Clean(t, GetTestingDir()); err != nil {
		t.Errorf("[ERROR] unable to remove test directories: %v", err)
	}
}

func TestGenBaseUserFiles(t *testing.T) {
	svc := &Service{
		SvcRoot: GetTestingDir(),
		UserDir: filepath.Join(GetTestingDir(), "users"),
	}

	GenBaseUserFiles(svc.UserDir)

	entries, err := os.ReadDir(svc.UserDir)
	if err != nil {
		Fatal(t, err)
	}
	assert.NotEqual(t, 0, len(entries))
	for _, e := range entries {
		assert.True(t, strings.Contains(e.Name(), ".json"))
	}

	if err := Clean(t, GetTestingDir()); err != nil {
		t.Errorf("[ERROR] unable to remove test directories: %v", err)
	}
}

func TestAllocateDrive(t *testing.T) {
	svc := &Service{
		SvcRoot: GetTestingDir(),
	}

	// create a temp "users" directory
	if err := os.Mkdir(filepath.Join(svc.SvcRoot, "users"), 0644); err != nil {
		t.Fatalf("%v", err)
	}

	// allocate a new tmp drive
	d := AllocateDrive("test", "me", GetTestingDir())
	assert.NotEqual(t, nil, d)

	// make sure all the basic user files are present
	u := filepath.Join(svc.SvcRoot, "users")
	drivePath := filepath.Join(u, "test")
	entries, err := os.ReadDir(drivePath)
	if err != nil {
		Fatal(t, err)
	}
	assert.NotEqual(t, 0, len(entries))

	if err := Clean(t, GetTestingDir()); err != nil {
		t.Errorf("[ERROR] unable to remove test directories: %v", err)
	}
}

// func TestSvcUserGets(t *testing.T) {}

// func TestSvcUserRemoves(t *testing.T) {}

// func TestSvcClearAll(t *testing.T) {}

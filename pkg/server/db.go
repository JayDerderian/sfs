package server

import (
	"fmt"
	"path/filepath"

	"github.com/sfs/pkg/auth"
	"github.com/sfs/pkg/db"
	"github.com/sfs/pkg/files"
)

// ----- db utils --------------------------------

// NOTE uses the service root as defined in the .env file.
//
// will need a way to update that when svc.SaveState() is called ?
func GetDbDir() string {
	c := ServiceConfig()
	return filepath.Join(c.ServiceRoot, "dbs")
}

func GetUserDb() string {
	return filepath.Join(GetDbDir(), "users")
}

func GetFilesDb() string {
	return filepath.Join(GetDbDir(), "files")
}

// get file info from db
func findFile(fileID string, dbDir string) (*files.File, error) {
	q := db.NewQuery(dbDir, false)
	f, err := q.GetFile(fileID)
	if err != nil {
		return nil, err
	}
	if f == nil {
		return nil, fmt.Errorf("no file found with ID %s", fileID)
	}
	return f, nil
}

// get user data from db
func findUser(userID string, dbDir string) (*auth.User, error) {
	q := db.NewQuery(dbDir, false)
	u, err := q.GetUser(userID)
	if err != nil {
		return nil, err
	}
	return u, nil
}

// get directory data from db
func findDir(dirID string, dbDir string) (*files.Directory, error) {
	q := db.NewQuery(dbDir, false)
	d, err := q.GetDirectory(dirID)
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, fmt.Errorf("no directory found with id %s", dirID)
	}
	return d, nil
}

// get drive data from db
func findDrive(driveID string, dbDir string) (*files.Drive, error) {
	q := db.NewQuery(dbDir, false)
	d, err := q.GetDrive(driveID)
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, fmt.Errorf("no drive found with id %s", driveID)
	}
	return d, nil
}
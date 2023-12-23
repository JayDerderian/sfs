package server

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sfs/pkg/auth"
	"github.com/sfs/pkg/db"
	svc "github.com/sfs/pkg/service"
)

/*
Service Drive directory.
Container for all user Drives (dirs w/metadata and a sub dir for the users stuff).

Top level entry point for internal user file system and their operations.
Will likely be the entry point used for when a server is spun up.

All service configurations may end up living here.
*/
type Service struct {
	InitTime time.Time `json:"init_time"`

	// path for sfs service on the server
	SvcRoot string `json:"service_root"`

	// path to state file
	StateFile string `json:"sf_file"`

	// path to users directory
	UserDir string `json:"user_dir"`

	// path to database directory
	DbDir string `json:"db_dir"`

	// db singleton connection
	Db *db.Query

	// admin mode. allows for expanded permissions when working with
	// the internal sfs file systems.
	AdminMode bool   `json:"admin_mode"`
	Admin     string `json:"admin"`
	AdminKey  string `json:"admin_key"`

	// key: user id, val is user struct.
	//
	// user structs contain a path string to the users Drive directory,
	// so this can be used for measuring disc size and executing
	// health checks
	Users map[string]*auth.User `json:"users"`
}

// intialize a new empty service struct
func NewService(svcRoot string) *Service {
	return &Service{
		InitTime: time.Now().UTC(),
		SvcRoot:  svcRoot,

		// we don't set StateFile because we assume it
		// doesn't exist when NewService is called
		StateFile: "",
		UserDir:   filepath.Join(svcRoot, "users"),
		DbDir:     filepath.Join(svcRoot, "dbs"),
		Db:        db.NewQuery(filepath.Join(svcRoot, "dbs"), true),

		// admin mode is optional.
		// these are standard default values
		AdminMode: false,
		Admin:     "admin",
		AdminKey:  "default",

		Users: make(map[string]*auth.User),
	}
}

/*
SaveState is meant to capture the current value of
the following fields when saving service state to disk:

	InitTime time.Time `json:"init_time"`

	SvcRoot string `json:"service_root"`  // directory path for sfs service on the server
	StateFile string `json:"state_file"`  // path to state file directory

	UserDir string `json:"user_dir"` // path to user drives directory
	DbDir string `json:"db_dir"`     // path to data directory

	// admin mode. allows for expanded permissions when working with
	// the internal sfs file systems.
	AdminMode bool   `json:"admin_mode"`
	Admin     string `json:"admin"`
	AdminKey  string `json:"admin_key"`

all information about user file metadata  are saved in the database.
the above fields are saved as a json file.
*/
func (s *Service) SaveState() error {
	sfDir := filepath.Join(s.SvcRoot, "state")

	// make sure we only have one state file at a time
	if err := s.cleanSfDir(sfDir); err != nil {
		return err
	}

	// marshal state instance and write out
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("unable to marshal service state: %v", err)
	}
	sfName := fmt.Sprintf("sfs-state-%s.json", time.Now().UTC().Format("2006-01-02T15-04-05"))
	s.StateFile = filepath.Join(sfDir, sfName)

	return os.WriteFile(s.StateFile, data, svc.PERMS)
}

// remove previous state file(s) before writing out.
// we only want the most recent one available at a time.
func (s *Service) cleanSfDir(sfDir string) error {
	if entries, err := os.ReadDir(sfDir); err == nil {
		for _, entry := range entries {
			sf := filepath.Join(sfDir, entry.Name())
			if err := os.Remove(sf); err != nil {
				return err
			}
		}
	} else {
		log.Printf("[WARNING] failed to remove previous state file(s): %v", err)
	}
	return nil
}

// ------- init ---------------------------------------

// initialize a new server-side sfs service from either a state file/dbs or
// create a new service from scratch.
func Init(new bool, admin bool) (*Service, error) {
	c := ServiceConfig()
	if !new {
		// load from state file and dbs
		svc, err := SvcLoad(c.SvcRoot)
		if err != nil {
			return nil, fmt.Errorf("failed to load service config: %v", err)
		}
		if admin {
			setAdmin(svc)
		}
		return svc, nil
	} else {
		// initialize new sfs service
		svc, err := SvcInit(c.SvcRoot)
		if err != nil {
			return nil, err
		}
		if admin {
			setAdmin(svc)
		}
		return svc, nil
	}
}

func setAdmin(svc *Service) {
	cfg := ServerConfig()
	svc.AdminMode = true
	svc.Admin = cfg.Admin
	svc.AdminKey = cfg.AdminKey
}

// searches for the service state file.
// will returna an empty string if there was an error
func findStateFile(svcRoot string) (string, error) {
	sfPath := filepath.Join(svcRoot, "state")
	entries, err := os.ReadDir(sfPath)
	if err != nil {
		return "", fmt.Errorf("unable to read state file directory: %s \n%v", sfPath, err)
	}
	// should only ever be one state file at a time
	if len(entries) > 1 {
		log.Printf("[WARNING] multiple state files found under: %s", sfPath)
		for i, e := range entries {
			log.Printf("%d.	%s\n", i+1, e.Name())
		}
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), "sfs-state") {
			return filepath.Join(sfPath, entry.Name()), nil
		}
	}
	return "", nil
}

// load from a service state file. returns a new empty service struct.
//
// *does not instatiate svc, db, or user paths.* must be set elsewhere
func loadStateFile(sfPath string) (*Service, error) {
	file, err := os.ReadFile(sfPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %v", err)
	}
	svc := new(Service)
	if err := json.Unmarshal(file, svc); err != nil {
		return nil, fmt.Errorf("failed unmarshal service state file: %v", err)
	}
	svc.StateFile = sfPath
	svc.InitTime = time.Now().UTC()
	return svc, nil
}

// populate svc.Users map from users database
func loadUsers(svc *Service) (*Service, error) {
	usrs, err := svc.Db.GetUsers()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve user data from Users database: %v", err)
	}
	if len(usrs) == 0 {
		log.Print("[WARNING] no users found in Users database")
		return nil, nil
	}
	// NOTE: this assumes the physical files for each
	// user being loaded are already present. calling svc.AddUser()
	// will allocate a new drive service with new base files.
	for _, u := range usrs {
		if _, exists := svc.Users[u.ID]; !exists {
			svc.Users[u.ID] = u
		} else {
			log.Printf("[DEBUG] user %v already exists", u.ID)
		}
	}
	return svc, nil
}

/*
generate a root directory for a new sfs service.
the root sfs service directory should have the following structure:

root/
|---users/
|   |---userDriveA/
|   |---userDriveB/
|   (etc)
|---state/
|   |----sfs-state-date:hour:min:sec.json
|---dbs/
|   |---users
|   |---drives
|   |---directories
|   |---files
*/

// initialize a new service and corresponding databases
func SvcInit(svcRoot string) (*Service, error) {
	// make root service directory (wherever it should located)
	log.Print("creating root service directory...")
	if err := os.Mkdir(svcRoot, 0644); err != nil {
		return nil, fmt.Errorf("failed to make service root directory: %v", err)
	}

	//create top-level service directories
	log.Print("creating service subdirectories...")
	svcPaths := []string{
		filepath.Join(svcRoot, "users"),
		filepath.Join(svcRoot, "state"),
		filepath.Join(svcRoot, "dbs"),
	}
	for _, p := range svcPaths {
		if err := os.Mkdir(p, 0644); err != nil {
			return nil, fmt.Errorf("failed to make service directory: %v", err)
		}
	}

	// create new service databases
	log.Print("creating service databases...")
	if err := db.InitDBs(svcPaths[2]); err != nil {
		return nil, fmt.Errorf("failed to initialize service databases: %v", err)
	}

	// create new service instance and save initial state
	log.Print("initializng new service instance...")
	svc := NewService(svcRoot)
	if err := svc.SaveState(); err != nil {
		return nil, fmt.Errorf("failed to save service state: %v", err)
	}

	// update NEW_SERVICE variable so future boot ups won't
	// create a new service every time
	e := NewE()
	if err := e.Set("NEW_SERVICE", "false"); err != nil {
		return nil, fmt.Errorf("failed to update service .env file: %v", err)
	}

	log.Print("all set :)")
	return svc, nil
}

//   - are the databases present?
//   - is the statefile present?
//
// if not, raise an error, otherwise returns the
// path to the state file upon success
func preChecks(svcRoot string) (string, error) {
	entries, err := os.ReadDir(svcRoot)
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return "", fmt.Errorf("no statefile, user, or database directories found! \n%v", err)
	}
	for _, entry := range entries {
		if entry.Name() == "state" || entry.Name() == "dbs" {
			if isEmpty(filepath.Join(svcRoot, entry.Name())) {
				return "", fmt.Errorf("%s directory is empty \n%v", entry.Name(), err)
			}
		}
	}
	sfPath, err := findStateFile(svcRoot)
	if err != nil {
		return "", err
	}
	if sfPath == "" {
		return "", fmt.Errorf("no state file found")
	}
	return sfPath, nil
}

// intialize sfs service from a state file
func svcLoad(sfPath string) (*Service, error) {
	svc, err := loadStateFile(sfPath)
	if err != nil {
		return nil, err
	}
	return svc, nil
}

// read in an external service state file
//
// populates users map through querying the users database
func SvcLoad(svcPath string) (*Service, error) {
	// ensure (at least) the necessary dbs
	// and state files are present
	sfPath, err := preChecks(svcPath)
	if err != nil {
		return nil, err
	}
	svc, err := svcLoad(sfPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize service: %v", err)
	}
	// instantiate DB connections
	// svc.Db = db.NewQuery(svc.DbDir, true)
	// attempt to populate from users database if state file had no user data
	// make sure we have a path to the db dir and current state file for this session
	if len(svc.Users) == 0 {
		log.Printf("[WARNING] state file had no user data. attempting to populate from users database...")
		_, err := loadUsers(svc)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve user data: %v", err)
		}
	}
	return svc, nil
}

// re-initialize users map and service state file
func (s *Service) resetUserMap() error {
	s.Users = nil
	s.Users = make(map[string]*auth.User)
	if err := os.Remove(s.StateFile); err != nil {
		return fmt.Errorf("failed to remove state file: %v", err)
	}
	if err := s.SaveState(); err != nil {
		return fmt.Errorf("failed to save state file: %v", err)
	}
	return nil
}

// SFS reset function to clear dbs (not delete tables, just all data),
// reset sfs state file, and user root directory. primarily for testing.
func (s *Service) Reset(svcPath string) error {
	if s.AdminMode {
		// clear databases
		for _, dbName := range s.Db.DBs {
			if err := s.Db.ClearTable(dbName); err != nil {
				return fmt.Errorf("failed to clear databases: %v", err)
			}
		}
		// clear users directory of all roots and subdirectories
		for _, user := range s.Users {
			if err := os.RemoveAll(user.SvcRoot); err != nil {
				return fmt.Errorf("failed to remove user files: %v", err)
			}
		}
		if err := s.resetUserMap(); err != nil {
			return fmt.Errorf("failed to reset user map: %v", err)
		}
	} else {
		log.Print("[INFO] must be in admin mode to reset service")
	}
	return nil
}

// --------- drives --------------------------------

/*
build a new privilaged drive directory for a client on the sfs server
with base state file info for user and drive json files

must be under ../svcroot/users/<username>

drives should have the following structure:

user/
|---meta/
|   |---user.json
|   |---drive.json
|---root/    <---- user files & directories live here
|---state/
|   |---userID-d-m-y-hh-mm-ss.json
*/
func AllocateDrive(name, ownerID, svcRoot string) (*svc.Drive, error) {
	// new user service file paths
	allUsers := filepath.Join(svcRoot, "users")
	userRoot := filepath.Join(allUsers, name)
	contentsRoot := filepath.Join(userRoot, "root")
	metaRoot := filepath.Join(userRoot, "meta")
	// make each directory
	dirs := []string{userRoot, contentsRoot, metaRoot}
	for _, d := range dirs {
		if err := os.Mkdir(d, 0644); err != nil {
			return nil, err
		}
	}
	// gen root and drive objects
	rt := svc.NewRootDirectory(name, ownerID, contentsRoot)
	drv := svc.NewDrive(svc.NewUUID(), name, ownerID, userRoot, rt.ID, rt)
	return drv, nil
}

// check for whether a drive exists
func (s *Service) DriveExists(driveID string) bool {
	if d, err := s.Db.GetDrive(driveID); err != nil {
		// return true to not accidentally allocate a new drive.
		// just because the DB errored out doesn't mean
		// the drive or its files don't exist.
		log.Printf("[WARNING] error getting drive: %v", err)
		return true
	} else {
		return d != nil // nil means not found
	}
}

// save drive state to DB
func (s *Service) SaveDrive(d *svc.Drive) error {
	if err := s.Db.UpdateDrive(d); err != nil {
		return err
	}
	return nil
}

// search DB for drive info, if available. returns a
// *svc.Drive pointer if successful, nil or error otherwise
func (s *Service) FindDrive(driveID string) (*svc.Drive, error) {
	drv, err := s.Db.GetDrive(driveID)
	if err != nil {
		return nil, err
	}
	if drv == nil {
		log.Printf("[INFO] drive %s not found", driveID)
		return nil, nil
	}
	root, err := s.Db.GetDirectory(drv.RootID)
	if err != nil {
		return nil, err
	}
	if root == nil {
		return nil, fmt.Errorf("root %s not found for drive %s", drv.RootID, drv.ID)
	}
	drv.Root = s.Populate(root)
	return drv, nil
}

// remove a drive and all its files and directories, as well
// as its info from the database
func (s *Service) RemoveDrive(driveID string) error {
	drv, err := s.Db.GetDrive(driveID)
	if err != nil {
		return err
	}
	if drv == nil {
		log.Printf("[INFO] drive %s not found", driveID)
		return nil
	}
	// remove drive files/directories
	if err := drv.Root.Clean(drv.Root.Path); err != nil {
		return err
	}
	// remove drive from db
	if err := s.Db.RemoveDrive(driveID); err != nil {
		return err
	}
	log.Printf("[INFO] drive %s removed", driveID)
	return nil
}

// add drive and all files and subdirectoris to the service instance
func (s *Service) AddDrive(drv *svc.Drive) error {
	// add all files and subdirectories to the database
	files := drv.Root.WalkFs()
	for _, f := range files {
		if err := s.Db.AddFile(f); err != nil {
			return err
		}
	}
	subDirs := drv.Root.WalkDs()
	for _, d := range subDirs {
		if err := s.Db.AddDir(d); err != nil {
			return err
		}
	}
	// add drive and root dir to db
	if err := s.Db.AddDrive(drv); err != nil {
		return err
	}
	if err := s.Db.AddDir(drv.Root); err != nil {
		return err
	}
	return nil
}

// --------- users --------------------------------

func (s *Service) TotalUsers() int {
	return len(s.Users)
}

// checks service instance and user db for whether a user exists
func (s *Service) UserExists(userID string) bool {
	if _, exists := s.Users[userID]; exists {
		// make sure they exist in the DB too
		exists, err := s.Db.UserExists(userID)
		if err != nil {
			log.Fatalf("failed to check user database: %v", err)
		}
		return exists
	} else {
		// last shot, try the DB
		exists, err := s.Db.UserExists(userID)
		if err != nil {
			log.Fatalf("failed to check user database: %v", err)
		}
		return exists
	}
}

// generate new user instance, and create drive and other base files
func (s *Service) addUser(user *auth.User) error {
	// check to see if this user already has a drive
	if dID, err := s.Db.GetDriveID(user.ID); err != nil {
		return fmt.Errorf("failed to get drive ID: %v", err)
	} else if dID != "" {
		return fmt.Errorf("user (%s) already has a drive (%s): ", user.ID, dID)
	}

	// allocate new drive and base service files
	newDrive, err := AllocateDrive(user.Name, user.Name, s.SvcRoot)
	if err != nil {
		return fmt.Errorf("failed to allocate new drive for user %s \n%v", user.ID, err)
	}
	user.DriveID = newDrive.ID
	user.DrvRoot = newDrive.Root.Path

	// save info to the db, then update service user map
	if err := s.Db.AddUser(user); err != nil {
		return fmt.Errorf("failed to add user to database: %v", err)
	}
	if err := s.Db.AddDrive(newDrive); err != nil {
		return fmt.Errorf("failed to add drive to database: %v", err)
	}
	if err := s.Db.AddDir(newDrive.Root); err != nil {
		return fmt.Errorf("failed to add drive root directory to database: %v", err)
	}
	s.Users[user.ID] = user
	if err := s.SaveState(); err != nil {
		log.Printf("[WARNING] failed to save state file: %v", err)
	}
	return nil
}

// allocate a new service drive for a new user.
//
// creates a new service drive, adds the user to the
// user database and service instance, and adds the
// drive to the drive database.
func (s *Service) AddUser(newUser *auth.User) error {
	if _, exists := s.Users[newUser.ID]; !exists {
		if err := s.addUser(newUser); err != nil {
			return err
		}
		log.Printf("[INFO] added user (name=%s id=%s)", newUser.Name, newUser.ID)
		if err := s.SaveState(); err != nil {
			log.Printf("[WARNING] failed to save service state: %s", err)
			return nil
		}
	} else {
		return fmt.Errorf("user (id=%s) already exists", newUser.ID)
	}
	return nil
}

// remove users's drive and all files and directories within,
// as well as all drive information from the database
func (s *Service) removeUser(driveID string) error {
	if d, err := s.Db.GetDrive(driveID); err == nil {
		if d != nil && d.DriveRoot != "" {
			// get root directory for this drive
			root, err := s.Db.GetDirectory(d.RootID)
			if err != nil {
				return err
			}
			if d.Root == nil {
				d.Root = root
			}
			// remove all files and directories
			if err := d.Root.Clean(d.DriveRoot); err != nil {
				return err
			}
			//remove users root dir itself
			if err := os.RemoveAll(d.DriveRoot); err != nil {
				return err
			}
			// remove drive from database
			if err := s.Db.RemoveDrive(driveID); err != nil {
				return err
			}
			// remove root directory from db
			if err := s.Db.RemoveDirectory(root.ID); err != nil {
				return err
			}
			log.Printf("[INFO] drive (id=%s) removed", driveID)
		} else {
			log.Printf("[DEBUG] drive (id=%s) not found", driveID)
		}
	} else {
		return err
	}
	return nil
}

// remove a user and all their files and directories
func (s *Service) RemoveUser(userID string) error {
	if usr, exists := s.Users[userID]; exists {
		// remove users drive and all files/directories
		if err := s.removeUser(usr.DriveID); err != nil {
			return err
		}
		// remove user from database
		if err := s.Db.RemoveUser(usr.ID); err != nil {
			return err
		}
		// delete from service instance
		delete(s.Users, usr.ID)
		log.Printf("[INFO] user (id=%s) removed", userID)
		if err := s.SaveState(); err != nil {
			return err
		}
	} else {
		log.Printf("[WARNING] user (id=%s) not found", userID)
	}
	return nil
}

// find a user. if not in the instance, then it will query the database.
//
// returns nil if user isn't found
func (s *Service) FindUser(userId string) (*auth.User, error) {
	if u, exists := s.Users[userId]; !exists {
		u, err := s.Db.GetUser(userId)
		if err != nil {
			return nil, err
		}
		if u == nil {
			log.Printf("[INFO] user (id=%s) not found", userId)
			return nil, nil
		}
		s.Users[u.ID] = u // add to the map since we didn't find it initially
		return u, nil
	} else {
		return u, nil
	}
}

func (s *Service) updateUser(user *auth.User) error {
	if err := s.Db.UpdateUser(user); err != nil {
		return fmt.Errorf("failed to update user in database: %v", err)
	}
	s.Users[user.ID] = user
	log.Printf("[INFO] user (id=%s) updated", user.ID)
	return nil
}

// update user info
func (s *Service) UpdateUser(user *auth.User) error {
	if _, exists := s.Users[user.ID]; exists {
		u, err := s.Db.GetUser(user.ID)
		if err != nil {
			return err
		}
		if u != nil { // user exists, update it
			if err := s.updateUser(user); err != nil {
				return err
			}
			log.Printf("[INFO] user (id=%s) updated", user.ID)
			if err = s.SaveState(); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("user %s not found in user database", user.ID)
		}
	} else {
		return fmt.Errorf("user %s not found", user.ID)
	}
	return nil
}

// ---------- drives-------------------------------------

// discover populates the given root directory with the users file and
// sub directories, updates the database as it does so, and returns
// the the directory object when finished.
//
// this should ideally be used for starting a new sfs service in a
// users root directly that already has files and/or subdirectories.
func (s *Service) Discover(dir *svc.Directory) *svc.Directory {
	// traverse users SFS file system and populate internal structures
	dir = dir.Walk()

	// send everything to the database
	files := dir.WalkFs()
	for _, file := range files {
		if err := s.Db.AddFile(file); err != nil {
			return nil
		}
	}
	dirs := dir.WalkDs()
	for _, d := range dirs {
		if err := s.Db.AddDir(d); err != nil {
			return nil
		}
	}
	// add root directory itself
	if err := s.Db.AddDir(dir); err != nil {
		return nil
	}
	return dir
}

// Populate() populates a drive's root directory with all the users
// files and subdirectories by searching the DB with the name
// of each file or directory Populate() discoveres as it traverses the
// users SFS filesystem.
//
// Note that Populate() ignores files and subdirectories it doesn't find in the
// database as its traversing the file system. This may or may not be a good thing.
func (s *Service) Populate(root *svc.Directory) *svc.Directory {
	if root.Path == "" {
		log.Print("[WARNING] can't traverse directory without a path")
		return nil
	}
	if root.IsNil() {
		log.Printf(
			"[WARNING] can't traverse directory with empty or nil maps: \nfiles=%v dirs=%v",
			root.Files, root.Dirs,
		)
		return nil
	}
	return s.populate(root)
}

func (s *Service) populate(dir *svc.Directory) *svc.Directory {
	entries, err := os.ReadDir(dir.Path)
	if err != nil {
		log.Printf("[ERROR]: %v", err)
		return dir
	}
	if len(entries) == 0 {
		log.Printf("[INFO] dir (id=%s) has no entries: ", dir.ID)
		return dir
	}
	for _, entry := range entries {
		entryPath := filepath.Join(dir.Path, entry.Name())
		item, err := os.Stat(entryPath)
		if err != nil {
			log.Printf("[ERROR] could not get stat for entry %s \nerr: %v", entryPath, err)
			return dir
		}
		// add directory then recurse
		if item.IsDir() {
			// TODO: make sure subDir's Dir map is initialized!
			subDir, err := s.Db.GetDirectoryByName(item.Name())
			if err != nil {
				log.Printf("[ERROR] could not get directory from db: %v \nerr: %v", item.Name(), err)
				continue
			}
			if subDir == nil {
				continue // not found
			}
			// confirm this belongs to the correct owner.
			if subDir.OwnerID != dir.OwnerID {
				log.Printf("[WARNING] directory owner ID mismatch. orig: %v, new: %v", dir.OwnerID, subDir.OwnerID)
				// continue
			}
			subDir = s.populate(subDir)
			dir.AddSubDir(subDir)
		} else { // add file
			file, err := s.Db.GetFileByName(item.Name())
			if err != nil {
				log.Printf("[ERROR] could not get file (%s) from db: %v", item.Name(), err)
				continue
			}

			if file == nil {
				continue // not found
			}
			// confirm this belongs to the correct owner.
			if file.OwnerID != dir.OwnerID {
				log.Printf("[WARNING] file owner ID mismatch. orig: %v, new: %v", dir.OwnerID, file.OwnerID)
				// continue
			}
			dir.AddFile(file)
		}
	}
	return dir
}

// retrives a drive and its root directory from the DB, then uses the
// DB to populate the drive as it recusively traverses the users SFS file system.
func (s *Service) GetDrive(driveID string) (*svc.Drive, error) {
	drive, err := s.Db.GetDrive(driveID)
	if err != nil {
		return nil, err
	}
	// get root dir for this drive
	root, err := s.Db.GetDirectory(drive.RootID)
	if err != nil {
		return nil, err
	}
	drive.Root = s.Populate(root)
	return drive, nil
}

func (s *Service) GetDriveByUserID(userID string) (*svc.Drive, error) {
	drive, err := s.Db.GetDriveByUserID(userID) // get drive for this ID
	if err != nil {
		return nil, err
	}
	if drive == nil {
		return nil, fmt.Errorf("no drive found for user %s", userID)
	}
	// get root dir for this drive
	root, err := s.Db.GetDirectory(drive.RootID)
	if err != nil {
		return nil, err
	}
	drive.Root = s.Populate(root)
	return drive, nil
}

// --------- directories --------------------------------

// find a directory in the database and populate it with
// its files and subdirectories.
func (s *Service) FindDir(dirID string) (*svc.Directory, error) {
	dir, err := s.Db.GetDirectory(dirID)
	if err != nil {
		return nil, err
	}
	dir = s.Populate(dir)
	return dir, nil
}

// add a sub-directory to the given drive directory
func (s *Service) AddDir(dirID string, newDir *svc.Directory) error {
	destDir, err := s.Db.GetDirectory(dirID)
	if err != nil {
		return err
	}
	destDir.AddSubDir(newDir)
	if err := s.Db.AddDir(newDir); err != nil {
		return err
	}
	return nil
}

// remove a directory from a user's drive service.
func (s *Service) RemoveDir(driveID string, dirID string) error {
	// "root" is the user's drive directory.
	// it's assumed dirID is a sub-directory within the drive.
	root, err := s.Db.GetDirectory(driveID)
	if err != nil {
		return err
	}
	if root == nil {
		log.Printf("[WARNING] root %s not found in database", driveID)
		return nil
	}
	if root.ID == dirID {
		log.Printf(
			"[WARNING] can't delete user's drive instance. \ndriveID=%s dirID=%s",
			driveID, dirID,
		)
		return nil
	}
	if err := root.RemoveSubDir(dirID); err != nil {
		return err
	}
	if err := s.Db.RemoveDirectory(dirID); err != nil {
		return err
	}
	return nil
}

// ---------- files --------------------------------

// find a file in the database
func (s *Service) FindFile(userID string, fileID string) (*svc.File, error) {
	drive, err := s.GetDriveByUserID(userID) // get drive for this ID
	if err != nil {
		return nil, err
	}
	if drive == nil {
		return nil, fmt.Errorf("no drive found for user %s", userID)
	}
	file := drive.GetFile(fileID)
	if file == nil {
		return nil, fmt.Errorf("file %s not found", fileID)
	}
	return file, nil
}

// get all files for this user
func (s *Service) FindFiles(userID string, fileIds []string) (map[string]*svc.File, error) {
	drive, err := s.GetDriveByUserID(userID)
	if err != nil {
		return nil, err
	}
	if drive == nil {
		return nil, fmt.Errorf("no drive found for user %s", userID)
	}
	files := drive.GetFiles()
	if len(files) == 0 {
		log.Printf("[INFO] no files for user %s", userID)
	}
	return files, nil
}

// add a file to the service
func (s *Service) AddFile(userID string, dirID string, file *svc.File) error {
	drive, err := s.GetDriveByUserID(userID)
	if err != nil {
		return err
	}
	if drive == nil {
		return fmt.Errorf("no drive found for user %s", userID)
	}
	if err := drive.AddFile(dirID, file); err != nil {
		return err
	}
	// add file to the database
	if err := s.Db.AddFile(file); err != nil {
		return err
	}
	return nil
}

// update a file in the service
func (s *Service) UpdateFile(userID string, fileID string, data []byte) error {
	drive, err := s.GetDriveByUserID(userID)
	if err != nil {
		return err
	}
	if drive == nil {
		return fmt.Errorf("no drive found for user %s", userID)
	}
	file, err := s.Db.GetFile(fileID)
	if err != nil {
		return err
	}
	if err := drive.Root.UpdateFile(file, data); err != nil {
		return err
	}
	if err := s.Db.UpdateFile(file); err != nil {
		return err
	}
	return nil
}

// delete a file in the service. uses the users drive to delete the file.
// removes physical file and updates database.
func (s *Service) DeleteFile(userID string, dirID string, fileID string) error {
	drive, err := s.Db.GetDriveByUserID(userID)
	if err != nil {
		return err
	}
	if drive == nil {
		log.Printf("[WARNING] drive not found for user %s", userID)
		return nil
	}
	file, err := s.Db.GetFile(fileID)
	if err != nil {
		return err
	}
	if err := drive.RemoveFile(dirID, file); err != nil {
		return err
	}
	if err := s.Db.RemoveFile(fileID); err != nil {
		return err
	}
	return nil
}

// --------- sync --------------------------------

// retrieve a sync index for a given drive. used by the client
// to compare against and initiate a sync operation, if necessary
// will be used as the first step in a sync operation on the client side.
// sync index will be nil if not found.
func (s *Service) GetSyncIdx(driveID string) (*svc.SyncIndex, error) {
	// check for drive existance
	drive, err := s.Db.GetDrive(driveID)
	if err != nil {
		return nil, fmt.Errorf("failed to get drive: %v", err)
	}
	if drive == nil {
		log.Printf("[WARNING] drive %s not found", driveID)
		return nil, nil
	}
	// build sync index if necessary
	if len(drive.SyncIndex.LastSync) == 0 || drive.SyncIndex == nil {
		log.Printf("[WARNING] sync index for %s not found. creating...", driveID)
		ownerID, err := drive.GetOwnerID()
		if err != nil {
			return nil, err
		}
		drive.SyncIndex = drive.Root.WalkS(svc.NewSyncIndex(ownerID))
	}
	return drive.SyncIndex, nil
}

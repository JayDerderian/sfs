package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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

	// path to user file directory
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
		Db:        db.NewQuery(filepath.Join(svcRoot, "dbs"), false),

		// admin mode is optional.
		// these are standard default values
		AdminMode: false,
		Admin:     "admin",
		AdminKey:  "default",

		Users: make(map[string]*auth.User),
	}
}

// --------- service functions --------------------------------

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
	file, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("unable to marshal service state: %v", err)
	}

	sfName := fmt.Sprintf("sfs-state-%s.json", time.Now().UTC().Format("2006-01-02T15-04-05"))
	sfPath := filepath.Join(s.SvcRoot, "state")
	s.StateFile = filepath.Join(sfPath, sfName)

	return os.WriteFile(s.StateFile, file, 0644)
}

// ------- init ---------------------------------------

// initialize a new sfs service from either a state file/dbs or
// create a new service from scratch.
func Init(new bool, admin bool) (*Service, error) {
	c := ServiceConfig()
	if !new {
		// ---- load from state file and dbs
		svc, err := SvcLoad(c.S.SvcRoot, false)
		if err != nil {
			return nil, fmt.Errorf("failed to load service config: %v", err)
		}
		if admin {
			setAdmin(svc)
		}
		return svc, nil
	} else {
		// ----- initialize new sfs service
		svc, err := SvcInit(c.S.SvcRoot, false)
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
	svc.Admin = cfg.Server.Admin
	svc.AdminKey = cfg.Server.AdminKey
}

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
		// NOTE: this might not be the most *current* version,
		// just the one that's present at the moment.
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

	svc := &Service{}
	if err := json.Unmarshal(file, svc); err != nil {
		return nil, fmt.Errorf("failed unmarshal service state file: %v", err)
	}
	svc.Db.Singleton = true
	svc.StateFile = sfPath
	svc.InitTime = time.Now().UTC()

	return svc, nil
}

// populate svc.Users map from users database
func loadUsers(svc *Service) (*Service, error) {
	q := db.NewQuery(filepath.Join(svc.DbDir, "users"), false)
	usrs, err := q.GetUsers()
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
func SvcInit(svcRoot string, debug bool) (*Service, error) {
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

	// update .env file NEW_SERVICE variable so future boot ups won't
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
func SvcLoad(svcPath string, debug bool) (*Service, error) {
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
	// attempt to populate from users database
	// if state file had no user data
	if len(svc.Users) == 0 {
		log.Printf("[WARNING] state file had no user data. attempting to populate from users database...")
		_, err := loadUsers(svc)
		if err != nil {
			log.Fatalf("[ERROR] failed to retrieve user data: %v", err)
		}
	}
	// make sure we have a path to the db dir
	// and current state file for this session
	svc.DbDir = filepath.Join(svcPath, "dbs")
	svc.StateFile = sfPath
	return svc, nil
}

// ------ utils --------------------------------

// generate some base line meta data for this service instance.
// should generate a users.json file (which will keep track of active users),
// and a drives.json, containing info about each drive, its total size, its location,
// owner, init date, passwords, etc.
func GenBaseUserFiles(usrInfoDir string) {
	fileNames := []string{"user.json", "drive.json"}
	for i := 0; i < len(fileNames); i++ {
		saveJSON(usrInfoDir, fileNames[i], make(map[string]interface{}))
	}
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
func AllocateDrive(name string, owner string, svcRoot string) (*svc.Drive, error) {
	// new user service file paths
	usrsDir := filepath.Join(svcRoot, "users")
	svcDir := filepath.Join(usrsDir, name)
	usrRoot := filepath.Join(svcDir, "root")
	metaRoot := filepath.Join(svcDir, "meta")

	// make each directory
	dirs := []string{svcDir, usrRoot, metaRoot}
	for _, d := range dirs {
		if err := os.Mkdir(d, 0644); err != nil {
			return nil, err
		}
	}

	// gen base files for this user
	GenBaseUserFiles(metaRoot)

	// gen root and drive objects
	rt := svc.NewRootDirectory(name, owner, usrRoot)
	drv := svc.NewDrive(svc.NewUUID(), name, owner, svcDir, rt)

	return drv, nil
}

// save drive state to DB
func (s *Service) SaveDrive(d *svc.Drive) error {
	s.Db.WhichDB("drives")
	if err := s.Db.AddDrive(d); err != nil {
		return err
	}
	return nil
}

// search DB for drive info, if available
func (s *Service) FindDrive(driveID string) (*svc.Drive, error) {
	s.Db.WhichDB("drives")
	drv, err := s.Db.GetDrive(driveID)
	if err != nil {
		return nil, err
	}
	return drv, nil
}

// --------- users --------------------------------

func (s *Service) TotalUsers() int {
	return len(s.Users)
}

// save user state to their state file
func (s *Service) SaveUser(u *auth.User) error {
	if err := s.UpdateUser(u); err != nil {
		return err
	}
	if err := u.SaveState(); err != nil {
		return err
	}
	return nil
}

// generate new user instance, and create drive and other base files
func (s *Service) addUser(userID string, r *http.Request) error {

	// TODO: figure out how to retrieve name, userName, and email fields from a request.
	// probably some JSON blob of some kind

	user := auth.NewUser(
		"bill buttlicker", "billBB", "bill@bill.com", userID, s.SvcRoot, false,
	)

	// allocate new drive and base service files
	d, err := AllocateDrive(user.Name, user.Name, s.SvcRoot)
	if err != nil {
		return fmt.Errorf("failed to allocate new drive for user %s \n%v", user.ID, err)
	}
	user.DriveID = d.ID

	s.Db.WhichDB("users")
	if err := s.Db.AddUser(user); err != nil {
		return fmt.Errorf("failed to add user to database: %v", err)
	}
	if err := s.Db.AddDrive(d); err != nil {
		return fmt.Errorf("failed to add drive to database: %v", err)
	}

	s.Users[user.ID] = user
	if err := s.SaveState(); err != nil {
		log.Printf("[WARNING] failed to save service state: %v", err)
	}
	return nil
}

// allocate a new service drive for a newly added user
func (s *Service) AddUser(userID string, r *http.Request) error {
	if _, exists := s.Users[userID]; !exists {
		s.Db.WhichDB("users")
		u, err := s.Db.GetUser(userID) // try DB if not in instance
		if err != nil {
			return err
		} else if u != nil {
			return fmt.Errorf("%s already exists", userID)
		}
		// add new user to service and create new drive
		if err = s.addUser(userID, r); err != nil {
			return err
		}
		log.Printf("[INFO] added user: %s", userID)
		if err := s.SaveState(); err != nil {
			return err
		}
	} else {
		log.Printf("[DEBUG] user (id=%s) already present", userID)
	}
	return nil
}

// remove users's drive and all files and directories within
func (s *Service) removeUser(driveID string) error {
	s.Db.WhichDB("drives")
	if d, err := s.Db.GetDrive(driveID); err == nil {
		if d != nil {
			// remove all files and directories
			if err := d.Root.Clean(d.Root.Path); err != nil {
				return err
			}
			//remove users root dir itself
			if err := os.Remove(d.Root.Path); err != nil {
				return err
			}
			log.Printf("[INFO] drive (id=%s) deleted", driveID)
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
		if err := s.removeUser(usr.DriveID); err != nil {
			return err
		}
		s.Db.WhichDB("users")
		if err := s.Db.RemoveUser(usr.ID); err != nil {
			return err
		}
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

// find a user. if not in the instance, then it will query the database
func (s *Service) FindUser(userId string) (*auth.User, error) {
	if u, exists := s.Users[userId]; !exists {
		s.Db.WhichDB("users")
		u, err := s.Db.GetUser(userId) // try DB if not in instance
		if err != nil {
			return nil, err
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
	s.Db.DBPath = filepath.Join(s.SvcRoot, "dbs")

	log.Printf("[INFO] user (id=%s) updated", user.ID)
	return nil
}

func (s *Service) UpdateUser(user *auth.User) error {
	if _, exists := s.Users[user.ID]; !exists {
		s.Db.WhichDB("users")
		u, err := s.Db.GetUser(user.ID) // try DB if not in instance
		if err != nil {
			return err
		}
		if u != nil { // user exists, update it
			if err := s.updateUser(user); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("user %v not found", user.ID)
		}
	} else {
		s.Db.WhichDB("users")
		if err := s.updateUser(user); err != nil {
			return err
		}
		return nil
	}
	return nil
}

// --------- sync --------------------------------

func (s *Service) syncUp() error {
	return nil
}

func (s *Service) syncDown() error {
	return nil
}

// run a sync operation for a user. uses the supplied index,
// which should have ToUpdate already populated, and builds a
// batch of files to be either uploaded to or downloaded from
// a given client
func (s *Service) StartSync(driveID string, up, down bool, idx *svc.SyncIndex) error {
	return nil
}

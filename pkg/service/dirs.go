package service

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/sfs/pkg/auth"
)

/*
File for handling all things related to creating the user's cloud directory

Should have a root directory called "nimbus" as a base line, after that, users
can specify the directory layout through a .yaml configuration, or through directory
creation at the command line.

NOTE:

	files contain the mutex lock, so lock time is dependent on whenever a method
	using a directory object calls a file's .Save() function within its internal Dirs list

	may want to explore moving lock use to the directory level, rather than the file level (?)
	or maybe just roll with it ...??
*/

type Directory struct {
	ID      string  `json:"id"`       // dir UUID
	NMap    NameMap `json:"nmap"`     // name map
	Name    string  `json:"name"`     // dir name
	OwnerID string  `json:"owner"`    // owner UUID
	DriveID string  `json:"drive_id"` // drive ID this directory belongs to

	// size in MB
	Size float64 `json:"size"`

	// absolute path to this directory.
	// should be something like:
	// .../sfs/user/root/../this_directory
	Path string `json:"path"`

	// security attributes
	Protected bool   `json:"protected"`
	AuthType  string `json:"auth_type"`
	Key       string `json:"key"`

	// allows for automatic file overwriting or
	// directory replacement
	Overwrite bool `json:"overwrite"`

	// Last time this directory was modified
	LastSync time.Time `json:"last_sync"`

	// server API endpoint for this directory
	Endpoint string `json:"endpoint"`

	// map of files in this directory.
	// key is file uuid, value is file pointer
	Files map[string]*File `json:"files"`

	// map of subdirectories.
	// key is the directory uuid, value is the directory pointer
	Dirs map[string]*Directory `json:"directories"`

	// pointer to parent directory (if not root).
	Parent *Directory

	// disignator for whether this directory is considerd the "root" directory
	Root     bool   `json:"root"`
	RootPath string `json:"rootPath"`
}

// create a new root directory object. does not create physical directory.
func NewRootDirectory(dirName string, ownerID string, driveID string, rootPath string) *Directory {
	cfg := NewSvcCfg()
	uuid := auth.NewUUID()
	return &Directory{
		ID:        uuid,
		NMap:      newNameMap(dirName, uuid),
		Name:      dirName,
		OwnerID:   ownerID,
		DriveID:   driveID,
		Protected: false,
		Key:       "default",
		Overwrite: false,
		LastSync:  time.Now().UTC(),
		Dirs:      make(map[string]*Directory, 0),
		Files:     make(map[string]*File, 0),
		Endpoint:  fmt.Sprint(Endpoint, ":", cfg.Port, "/v1/dirs/", uuid),
		Parent:    nil,
		Root:      true,
		Path:      rootPath,
		RootPath:  rootPath,
	}
}

// NOTE: the Parent pointer isn't assigned by default! Must be assigned externally.
// This is mainly because I wanted an easier way to facilitate
// testing without having to create an entire mocked system.
// I'm sure I won't regret this.
func NewDirectory(dirName string, ownerID string, driveID string, path string) *Directory {
	cfg := NewSvcCfg()
	uuid := auth.NewUUID()
	return &Directory{
		ID:        uuid,
		NMap:      newNameMap(dirName, uuid),
		Name:      dirName,
		OwnerID:   ownerID,
		DriveID:   driveID,
		Protected: false,
		Key:       "default",
		Overwrite: false,
		LastSync:  time.Now().UTC(),
		Dirs:      make(map[string]*Directory, 0),
		Files:     make(map[string]*File, 0),
		Endpoint:  fmt.Sprint(Endpoint, ":", cfg.Port, "/v1/dirs/", uuid),
		Root:      false,
		Path:      path,
	}
}

func (d *Directory) ToJSON() ([]byte, error) {
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return nil, err
	}
	return data, nil
}

func UnmarshalDirStr(data string) (*Directory, error) {
	dir := new(Directory)
	if err := json.Unmarshal([]byte(data), &dir); err != nil {
		return nil, err
	}
	return dir, nil
}

func (d *Directory) IsRoot() bool {
	return d.Root
}

func (d *Directory) HasParent() bool {
	return !d.IsRoot() && d.Parent == nil
}

func (d *Directory) IsNil() bool {
	return d.Files == nil && d.Dirs == nil
}

func (d *Directory) IsEmpty() bool {
	return len(d.Files) == 0 && len(d.Dirs) == 0
}

// Remove all *internal data structure representations* of files and directories
// *Does not* remove actual files or sub directories themselves!
func (d *Directory) clear() {
	d.Files = nil
	d.Dirs = nil
	d.Files = make(map[string]*File, 0)
	d.Dirs = make(map[string]*Directory, 0)
	log.Printf("[INFO] dirID (%s) all directories and files cleared", d.ID)
}

// clears *in-memory* file and directory data structures.
// does not actually remove physical files or directories.
//
// used to securely run clear().
func (d *Directory) Clear(password string) {
	if !d.Protected {
		d.clear()
	} else {
		if password == d.Key {
			d.clear()
		} else {
			log.Print("[INFO] wrong password. contents not cleared")
		}
	}
}

// clean all files and subdirectories from the top-level directory
func clean(dirPath string) error {
	d, err := os.Open(dirPath)
	if err != nil {
		return err
	}
	defer d.Close()
	names, err := d.Readdirnames(-1)
	if err != nil {
		return fmt.Errorf("unable to read directory: %v", err)
	}
	for _, name := range names {
		if err = os.RemoveAll(filepath.Join(dirPath, name)); err != nil {
			return fmt.Errorf("unable to remove file or directory: %v", err)
		}
	}
	return nil
}

// calls clean() which ***removes all physical files and subdirectories
// from a drive starting at the given path***. use with caution!
//
// also calls dir.clear() which removes internal data structure references.
func (d *Directory) Clean(dirPath string) error {
	if !d.Protected {
		if err := clean(dirPath); err != nil {
			return err
		}
		d.clear()
		return nil
	} else {
		log.Printf("[INFO] drive is protected.")
	}
	return nil
}

func (d *Directory) HasFile(fileID string) bool {
	if _, exists := d.Files[fileID]; exists {
		return true
	}
	return false
}

func (d *Directory) HasDir(dirID string) bool {
	if _, exists := d.Dirs[dirID]; exists {
		return true
	}
	return false
}

// returns the size of a directory with all its contents, including subdirectories
func (d *Directory) DirSize() (float64, error) {
	var size float64
	err := filepath.Walk(d.Path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			// TODO: investigate how the conversion of
			// int64 to float64 can effect results.
			size += float64(info.Size())
		}
		return nil
	})
	return size, err
}

/*
get the parent directory for this directory.

only root directories can have a nil parent pointer
since they will have a valid *drive pointer to
point to the parent drive
*/
func (d *Directory) GetParent() *Directory {
	if !d.HasParent() {
		log.Fatal("no parent for non-root directory!")
	}
	return d.Parent
}

// -------- password protection and other simple security stuff

func (d *Directory) SetPassword(password string, newPassword string) error {
	if password == d.Key {
		d.Key = newPassword
		log.Printf("[INFO] password updated")
		return nil
	}
	return fmt.Errorf("[ERROR] wrong password")
}

func (d *Directory) Lock(password string) bool {
	if password == d.Key {
		d.Protected = true
		return true
	}
	log.Printf("[INFO] wrong password")
	return false
}

func (d *Directory) Unlock(password string) bool {
	if password == d.Key {
		d.Protected = false
		return true
	}
	log.Printf("[INFO] wrong password")
	return false
}

// --------- file management

// updates internal file map and file's sync time
func (d *Directory) addFile(file *File) {
	if _, exists := d.Files[file.ID]; !exists {
		file.DirID = d.ID
		file.DriveID = d.DriveID
		file.LastSync = time.Now().UTC()
		d.Files[file.ID] = file
		log.Printf("[INFO] file %s (%s) added", file.Name, file.ID)
	} else {
		log.Printf("[INFO] file %s (%s) already exists", file.Name, file.ID)
	}
}

func (d *Directory) AddFile(file *File) {
	if !d.Protected {
		if !d.HasFile(file.ID) {
			d.addFile(file)
		} else {
			log.Printf("[INFO] file %s (%s) already present in directory", file.Name, file.ID)
		}
	} else {
		log.Printf("[INFO] directory %s (%s) locked", d.Name, d.ID)
	}
}

func (d *Directory) AddFiles(files []*File) {
	if len(files) == 0 {
		log.Printf("[INFO] no files recieved")
		return
	}
	if !d.Protected {
		for _, f := range files {
			if !d.HasFile(f.ID) {
				d.addFile(f)
			} else {
				log.Printf("[INFO] file (%v) already exists)", f.ID)
			}
		}
	} else {
		log.Printf("[INFO] directory %s (%s) locked", d.Name, d.ID)
	}
}

// save new data to a file. file will be created or truncated,
// depending on its state at time of writing. does not check
// subdirectories for the existence of this file.
func (d *Directory) UpdateFile(f *File, data []byte) error {
	if !d.Protected {
		if d.HasFile(f.ID) {
			if err := f.Save(data); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("[ERROR] file (id=%s) does not belong to this directory", f.ID)
		}
	} else {
		log.Printf("[INFO] directory %s (%s) locked", d.Name, d.ID)
	}
	return nil
}

func (d *Directory) removeFile(fileID string) error {
	if file, ok := d.Files[fileID]; ok {
		if err := os.Remove(file.ServerPath); err != nil {
			return err
		}
		delete(d.Files, file.ID)
		d.LastSync = time.Now().UTC()
		log.Printf("[INFO] file %s removed", file.ID)
	} else {
		return fmt.Errorf("file %s not found", file.ID)
	}
	return nil
}

// removes file from internal file map and deletes physical file.
func (d *Directory) RemoveFile(fileID string) error {
	if !d.Protected {
		if err := d.removeFile(fileID); err != nil {
			return fmt.Errorf("failed to remove file: %s", err)
		}
	} else {
		log.Printf("[INFO] directory protected. unlock before removing files")
	}
	return nil
}

// returns a file map containing all files starting at this directory.
func (d *Directory) GetFileMap() map[string]*File {
	if len(d.Files) == 0 {
		log.Printf("[WARNING] dir (%s) has no files", d.ID)
	}
	return d.WalkFs()
}

// get a slice of all files starting from this directory.
// returns a nil slice if no files are found.
func (d *Directory) GetFiles() []*File {
	fileMap := d.WalkFs()
	if len(fileMap) == 0 {
		log.Printf("[INFO] dir (%s) has no files", d.ID)
		return nil
	}
	var i int
	files := make([]*File, len(fileMap))
	for _, f := range fileMap {
		files[i] = f
		i++
	}
	return files
}

// find a file within the given directory or subdirectories, starting at the current directory.
// returns nil if no such file exists
func (d *Directory) FindFile(fileID string) *File {
	return d.WalkF(fileID)
}

// -------- sub directory methods

// creates a new subdirectory and updates internal data structures.
func (d *Directory) addSubDir(dir *Directory) error {
	if _, exists := d.Dirs[dir.ID]; !exists {
		dir.Parent = d
		dir.DriveID = d.DriveID
		d.Dirs[dir.ID] = dir
		d.Dirs[dir.ID].LastSync = time.Now().UTC()
		log.Printf("[INFO] dir %s (id=%s) added", dir.Name, dir.ID)
	} else {
		return fmt.Errorf("dir %s (id=%s) already exists", dir.Name, dir.ID)
	}
	return nil
}

// add a single sub directory to the current directory.
// sets dir's parent pointer to the directory this
// function is attached to.
//
// does not create physical directory.
func (d *Directory) AddSubDir(dir *Directory) error {
	if !d.Protected {
		d.addSubDir(dir)
	} else {
		log.Printf("[DEBUG] dir %s is protected", d.Name)
	}
	return nil
}

// add a slice of directory objects to the current directory.
// does not create physical directories.
func (d *Directory) AddSubDirs(dirs []*Directory) error {
	if len(dirs) == 0 {
		log.Printf("[INFO] dir list is empty")
		return nil
	}
	if !d.Protected {
		for _, dir := range dirs {
			d.addSubDir(dir)
		}
	} else {
		log.Printf("[INFO] %s (id=%s) is protected", d.Name, d.ID)
	}
	return nil
}

func (d *Directory) removeDir(dirID string) error {
	if dir, exists := d.Dirs[dirID]; exists {
		if err := os.RemoveAll(dir.Path); err != nil {
			return fmt.Errorf("unable to remove directory %s: %v", dirID, err)
		}
		delete(d.Dirs, dirID)
		log.Printf("[INFO] directory (id=%s)  removed", dirID)
	} else {
		log.Printf("[INFOs] directory (id=%s) is not found", dirID)
	}
	return nil
}

// removes a physical sub-directy and *all of its child directories*
// as well as the clearing the internal data structures.
//
// use with caution!
func (d *Directory) RemoveSubDir(dirID string) error {
	if !d.Protected {
		if err := d.removeDir(dirID); err != nil {
			return err
		}
		// remove from subdir map & update sync time
		delete(d.Dirs, dirID)
		d.LastSync = time.Now().UTC()
		log.Printf("[INFO] directory %s deleted", dirID)
	} else {
		log.Printf("[INFO] directory %s is protected", dirID)
	}
	return nil
}

// removes *ALL* sub directories and their children for a given directory
//
// calls d.Clean() which recursively deletes all subdirctories and their children
func (d *Directory) RemoveSubDirs() error {
	if !d.Protected {
		if err := d.Clean(d.Path); err != nil {
			return err
		}
		log.Printf("[INFO] dir(%s) all sub directories deleted", d.ID)
	} else {
		log.Printf("[INFO] dir(%s) is protected. no sub directories deleted", d.ID)
	}
	return nil
}

// directly returns the subdirectory if it exists within the *current* directory.
func (d *Directory) GetSubDir(dirID string) *Directory {
	if dir, ok := d.Dirs[dirID]; ok {
		return dir
	} else {
		log.Printf("[INFO] dir(%s) not found", dirID)
		return nil
	}
}

// returns a map of all subdirectories starting from the current directory.
// returns an empty map if nothing is not found
func (d *Directory) GetSubDirs() map[string]*Directory {
	if len(d.Dirs) == 0 {
		log.Printf("[INFO] directory %s is empty", d.ID)
		return nil
	}
	return d.Dirs
}

// attempts to locate the directory or subdirectory starting from the given directory.
// returns nil if not found.
func (d *Directory) FindDir(dirID string) *Directory {
	return d.WalkD(dirID)
}

// ------------------------------------------------------------

/*
Walk() populates all files and subdirectory maps (and their files and subdirectories,
and so on) until we reach the end of the local directory tree.
should be used only when instantiating a root directory object
for the *first* time, as it will generate new file and directory objects
with their own ID's, and will need to be treated as persistent items rather than
ephemeral ones.
*/
func (d *Directory) Walk() *Directory {
	if d.Path == "" {
		log.Print("[WARNING] can't traverse directory without a path")
		return d
	}
	if d.IsNil() {
		log.Printf(
			"[WARNING] can't traverse directory with nil maps: \nfiles=%v dirs=%v",
			d.Files, d.Dirs,
		)
		return d
	}
	return walk(d)
}

// walk recursively descends the directory tree and populates all files and subdirectory maps
func walk(d *Directory) *Directory {
	entries, err := os.ReadDir(d.Path)
	if err != nil {
		log.Printf("[ERROR] could not read directory: %v", err)
		return d
	}
	if len(entries) == 0 {
		log.Printf("[INFO] dir (id=%s) has no entries. nothing to search. ", d.ID)
		return d
	}
	for _, entry := range entries {
		entryPath := filepath.Join(d.Path, entry.Name())
		item, err := os.Stat(entryPath)
		if err != nil {
			log.Printf("[ERROR] could not get stat for entry %s \nerr: %v", entryPath, err)
			return d
		}
		if item.IsDir() {
			sd := NewDirectory(item.Name(), d.OwnerID, d.DriveID, entryPath)
			sd = walk(sd)
			d.AddSubDir(sd)
		} else {
			file := NewFile(item.Name(), d.DriveID, d.OwnerID, entryPath)
			d.AddFile(file)
		}
	}
	return d
}

/*
WalkF() recursively traverses sub directories starting at a given directory (or root),
attempting to find the desired file with the given file ID.
*/
func (d *Directory) WalkF(fileID string) *File {
	return walkF(d, fileID)
}

func walkF(dir *Directory, fileID string) *File {
	if file, found := dir.Files[fileID]; found {
		return file
	}
	if len(dir.Dirs) == 0 {
		log.Printf("[INFO] dir %s (%s) has no sub directories. nothing to search", dir.Name, dir.ID)
		return nil
	}
	for _, subDirs := range dir.Dirs {
		if file := walkF(subDirs, fileID); file != nil {
			return file
		}
	}
	return nil
}

/*
WalkFs() recursively traversies all subdirectories and returns
a map of all files available for a given user
*/
func (d *Directory) WalkFs() map[string]*File {
	return walkFs(d, make(map[string]*File))
}

func walkFs(dir *Directory, files map[string]*File) map[string]*File {
	for _, file := range dir.Files {
		if _, exists := files[file.ID]; !exists {
			files[file.ID] = file
		}
	}
	if len(dir.Dirs) == 0 {
		log.Printf("[INFO] dir (id=%s) has no subdirectories. nothing to search", dir.ID)
		return files
	}
	for _, subDir := range dir.Dirs {
		return walkFs(subDir, files)
	}
	return nil
}

/*
WalkD() recursively traverses sub directories starting at a given directory (or root),
attempting to find the desired sub directory with the given directory ID.

Returns nil if the directory is not found
*/
func (d *Directory) WalkD(dirID string) *Directory {
	return walkD(d, dirID)
}

func walkD(dir *Directory, dirID string) *Directory {
	if len(dir.Dirs) == 0 {
		log.Printf("[INFO] %s (id=%s) has no sub directories. nothing to search", dir.Name, dir.ID)
		return nil
	}
	if d, ok := dir.Dirs[dirID]; ok {
		return d
	}
	for _, subDirs := range dir.Dirs {
		if sd := walkD(subDirs, dirID); sd != nil {
			return sd
		}
	}
	return nil
}

/*
WalkDs() recursively traverses sub directories starting at a
given directory (or root) constructing a map of all sub directories.

Returns an empty map if nothing is not found
*/
func (d *Directory) WalkDs() map[string]*Directory {
	return walkDs(d, make(map[string]*Directory))
}

func walkDs(dir *Directory, dirMap map[string]*Directory) map[string]*Directory {
	if len(dir.Dirs) == 0 {
		log.Printf("[INFO] %s (id=%s) has no sub directories. nothing to search", dir.Name, dir.ID)
		return dirMap
	}
	for _, subDir := range dir.Dirs {
		if _, exists := dirMap[subDir.ID]; !exists {
			dirMap[subDir.ID] = subDir
		}
		return walkDs(subDir, dirMap)
	}
	return dirMap
}

func buildSync(dir *Directory, idx *SyncIndex) *SyncIndex {
	for _, file := range dir.Files {
		if _, exists := idx.LastSync[file.ID]; !exists {
			idx.LastSync[file.ID] = file.LastSync
		}
	}
	return idx
}

/*
WalkS() recursively traverses each subdirectory starting from
the given directory and returns a *SyncIndex pointer containing
the last sync times for each file in each directory and subdirectories.
*/
func (d *Directory) WalkS(idx *SyncIndex) *SyncIndex {
	return walkS(d, idx)
}

func walkS(dir *Directory, idx *SyncIndex) *SyncIndex {
	if len(dir.Files) > 0 {
		idx = buildSync(dir, idx)
	} else {
		log.Printf("[INFO] dir %s (%s) has no files", dir.Name, dir.ID)
	}
	if len(dir.Dirs) == 0 {
		log.Printf("[INFO] dir %s (%s) has no sub directories", dir.Name, dir.ID)
		return idx
	}
	for _, subDirs := range dir.Dirs {
		if sIndx := walkS(subDirs, idx); sIndx != nil {
			return sIndx
		}
	}
	return nil
}

func buildUpdate(d *Directory, idx *SyncIndex) *SyncIndex {
	for _, file := range d.Files {
		if _, exists := idx.LastSync[file.ID]; exists {
			// check if the time difference between most recent sync
			// and last sync is greater than zero.
			if file.LastSync.Sub(idx.LastSync[file.ID]) > 0 {
				idx.ToUpdate[file.ID] = file
			}
		} else {
			continue // this wasn't found previously, ignore
		}
	}
	return idx
}

// d.WalkU() populates the ToUpdate map of a given SyncIndex
func (d *Directory) WalkU(idx *SyncIndex) *SyncIndex {
	return walkU(d, idx)
}

// walkU recursively walks the directory tree and checks the last sync time
// of each file in each subdirectory, populating the ToUpdate map of a given SyncIndex
// as needed.
func walkU(dir *Directory, idx *SyncIndex) *SyncIndex {
	if len(dir.Files) > 0 {
		idx = buildUpdate(dir, idx)
	} else {
		log.Printf("[INFO] dir %s (%s) has no files", dir.Name, dir.ID)
	}
	if len(dir.Dirs) == 0 {
		log.Printf("[INFO] dir %s (%s) has no sub directories", dir.Name, dir.ID)
		return idx
	}
	for _, subDirs := range dir.Dirs {
		if uIndx := walkU(subDirs, idx); uIndx != nil {
			return uIndx
		}
	}
	return nil
}

// TODO: look into how to make this generic. this way
// we can loosen the requirements of the type for the argument
// to op(), and possibly the return type(s)

// WalkO() searches each subdirectory recursively and performes
// a supplied function on each file in the directory, returning
// an error if the function fails
//
// functions should have the following signature: func(file *File) error
func (d *Directory) WalkO(op func(file *File) error) error {
	if len(d.Files) == 0 {
		log.Printf("[INFO] dir %s (%s) has no files", d.Name, d.ID)
	}
	if len(d.Dirs) == 0 {
		log.Printf("[INFO] dir %s (%s) has no sub directories", d.Name, d.ID)
		return nil
	}
	return walkO(d, op)
}

func walkO(dir *Directory, op func(f *File) error) error {
	if len(dir.Files) > 0 {
		for _, file := range dir.Files {
			if err := op(file); err != nil {
				// we don't exit right away because this exception may only apply
				// to a single file.
				log.Printf("[INFO] unable to run operation on %s \n%v\n continuing...", dir.Name, err)
				continue
			}
		}
	} else {
		log.Printf("[INFO] dir %s (%s) has no files", dir.Name, dir.ID)
	}
	if len(dir.Dirs) == 0 {
		log.Printf("[INFO] dir %s (%s) has no sub directories", dir.Name, dir.ID)
		return nil
	}
	for _, subDirs := range dir.Dirs {
		if err := walkO(subDirs, op); err != nil {
			return err
		}
	}
	return nil
}

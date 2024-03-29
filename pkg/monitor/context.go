package monitor

import (
	"io/fs"
	"path/filepath"
)

// used for keeping track of current
// items in the directory as its being monitored
type DirCtx struct {
	dirpath   string
	currItems map[string]EItem
}

func NewDirCtx(dirPath string) *DirCtx {
	return &DirCtx{
		dirpath:   dirPath,
		currItems: make(map[string]EItem),
	}
}

func (ctx *DirCtx) Clear() {
	ctx.currItems = nil
	ctx.currItems = make(map[string]EItem)
}

func (ctx *DirCtx) HaveItem(itemName string) bool {
	if _, ok := ctx.currItems[itemName]; ok {
		return true
	}
	return false
}

func getItemType(item fs.DirEntry) string {
	var itype string
	if item.IsDir() {
		itype = "directory"
	} else {
		itype = "file"
	}
	return itype
}

// adds all new fs.DirEntry objects to the current context and returns
// a slice of the newly added entries.
func (ctx *DirCtx) AddItems(newItems []fs.DirEntry) []EItem {
	diffs := make([]EItem, 0)
	for _, item := range newItems {
		if !ctx.HaveItem(item.Name()) {
			eitem := EItem{
				itype: getItemType(item),
				name:  item.Name(),
				path:  filepath.Join(ctx.dirpath, item.Name()),
			}
			diffs = append(diffs, eitem)
			ctx.currItems[eitem.Name()] = eitem
		}
	}
	return diffs
}

// remove items from context
func (ctx *DirCtx) RemoveItems(remove []fs.DirEntry) []EItem {
	diffs := make([]EItem, 0)
	for _, item := range remove {
		removed := EItem{
			itype: getItemType(item),
			name:  item.Name(),
			path:  filepath.Join(item.Name(), ctx.dirpath),
		}
		delete(ctx.currItems, item.Name())
		diffs = append(diffs, removed)
	}
	return diffs
}

package renter

import (
	"fmt"
	"sync"

	"github.com/turtledex/TurtleDexCore/modules"
	"github.com/turtledex/errors"
)

// uniqueRefreshPaths is a helper struct for determining the minimum number of
// directories that will need to have callThreadedBubbleMetadata called on in
// order to properly update the affected directory tree. Since bubble calls
// itself on the parent directory when it finishes with a directory, only a call
// to the lowest level child directory is needed to properly update the entire
// directory tree.
type uniqueRefreshPaths struct {
	childDirs  map[modules.TurtleDexPath]struct{}
	parentDirs map[modules.TurtleDexPath]struct{}

	r  *Renter
	mu sync.Mutex
}

// newUniqueRefreshPaths returns an initialized uniqueRefreshPaths struct
func (r *Renter) newUniqueRefreshPaths() *uniqueRefreshPaths {
	return &uniqueRefreshPaths{
		childDirs:  make(map[modules.TurtleDexPath]struct{}),
		parentDirs: make(map[modules.TurtleDexPath]struct{}),

		r: r,
	}
}

// callAdd adds a path to uniqueRefreshPaths.
func (urp *uniqueRefreshPaths) callAdd(path modules.TurtleDexPath) error {
	urp.mu.Lock()
	defer urp.mu.Unlock()

	// Check if the path is in the parent directory map
	if _, ok := urp.parentDirs[path]; ok {
		return nil
	}

	// Check if the path is in the child directory map
	if _, ok := urp.childDirs[path]; ok {
		return nil
	}

	// Add path to the childDir map
	urp.childDirs[path] = struct{}{}

	// Check all path elements to make sure any parent directories are removed
	// from the child directory map and added to the parent directory map
	for !path.IsRoot() {
		// Get the parentDir of the path
		parentDir, err := path.Dir()
		if err != nil {
			contextStr := fmt.Sprintf("unable to get parent directory of %v", path)
			return errors.AddContext(err, contextStr)
		}
		// Check if the parentDir is in the childDirs map
		if _, ok := urp.childDirs[parentDir]; ok {
			// Remove from childDir map and add to parentDir map
			delete(urp.childDirs, parentDir)
		}
		// Make sure the parentDir is in the parentDirs map
		urp.parentDirs[parentDir] = struct{}{}
		// Set path equal to the parentDir
		path = parentDir
	}
	return nil
}

// callNumChildDirs returns the number of child directories currently being
// tracked.
func (urp *uniqueRefreshPaths) callNumChildDirs() int {
	urp.mu.Lock()
	defer urp.mu.Unlock()
	return len(urp.childDirs)
}

// callNumParentDirs returns the number of parent directories currently being
// tracked.
func (urp *uniqueRefreshPaths) callNumParentDirs() int {
	urp.mu.Lock()
	defer urp.mu.Unlock()
	return len(urp.parentDirs)
}

// callRefreshAll uses the uniqueRefreshPaths's Renter to call
// callThreadedBubbleMetadata on all the directories in the childDir map
func (urp *uniqueRefreshPaths) callRefreshAll() {
	urp.mu.Lock()
	defer urp.mu.Unlock()
	for sp := range urp.childDirs {
		go urp.r.callThreadedBubbleMetadata(sp)
	}
}

// callRefreshAllBlocking uses the uniqueRefreshPaths's Renter to call
// managedBubbleMetadata on all the directories in the childDir map
func (urp *uniqueRefreshPaths) callRefreshAllBlocking() (err error) {
	urp.mu.Lock()
	defer urp.mu.Unlock()
	for sp := range urp.childDirs {
		err = errors.Compose(err, urp.r.managedBubbleMetadata(sp))
	}
	return
}

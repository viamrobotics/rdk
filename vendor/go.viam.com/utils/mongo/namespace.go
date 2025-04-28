// Package mongoutils contains utilities for working with MongoDB more effectively.
package mongoutils

import (
	"fmt"
	"strings"
	"sync"

	"go.viam.com/utils"
)

var (
	namespaces          = map[*string][]*string{}
	unmanagedNamespaces = map[string][]string{}
	namespacesMu        sync.Mutex
)

// RegisterNamespace globally registers the given database and collection as in use
// with MongoDB. It will error if there's a duplicate registration.
func RegisterNamespace(db, coll *string) error {
	namespacesMu.Lock()
	defer namespacesMu.Unlock()
	colls := namespaces[db]
	for _, existingColl := range colls {
		if coll == existingColl {
			return nil
		}
		if *coll == *existingColl {
			return fmt.Errorf("%q defined in more than one locations", *coll)
		}
	}
	colls = append(colls, coll)
	namespaces[db] = colls
	return nil
}

// RegisterUnmanagedNamespace registers a namespace that is known of, but is not directly
// owned by this program. It will not qualify for randomization.
func RegisterUnmanagedNamespace(db, coll string) error {
	namespacesMu.Lock()
	defer namespacesMu.Unlock()
	colls := unmanagedNamespaces[db]
	for _, existingColl := range colls {
		if coll == existingColl {
			return fmt.Errorf("%q defined in more than one locations", unmanagedNamespaces)
		}
	}
	colls = append(colls, coll)
	unmanagedNamespaces[db] = colls
	return nil
}

// MustRegisterNamespace ensures the given database and collection can be registered
// and panics otherwise.
func MustRegisterNamespace(db, coll *string) {
	if err := RegisterNamespace(db, coll); err != nil {
		panic(err)
	}
}

type randomizedName struct {
	ptr  *string
	from string
	to   string
}

func getNamespaces() map[string][]string {
	namespacesCopy := map[string][]string{}
	for db, colls := range namespaces {
		namespacesCopy[*db] = nil
		for _, coll := range colls {
			namespacesCopy[*db] = append(namespacesCopy[*db], *coll)
		}
	}
	return namespacesCopy
}

// Namespaces returns a copy of all registered namespaces.
func Namespaces() map[string][]string {
	namespacesMu.Lock()
	defer namespacesMu.Unlock()
	return getNamespaces()
}

// UnmanagedNamespaces returns a copy of all unmanaged namespaces.
func UnmanagedNamespaces() map[string][]string {
	namespacesMu.Lock()
	defer namespacesMu.Unlock()
	namespacesCopy := map[string][]string{}
	for db, colls := range unmanagedNamespaces {
		namespacesCopy[db] = append(namespacesCopy[db], colls...)
	}
	return namespacesCopy
}

// RandomizeNamespaces is a utility to be used by tests to remap all registered namespaces
// before tests run in order to isolate where test data is stored. The returned restore function
// should be called after tests are done in order to restore the namespaces to their former state.
func RandomizeNamespaces() (newNamespaces map[string][]string, restore func()) {
	namespacesMu.Lock()
	defer namespacesMu.Unlock()
	oldNamespaces := map[randomizedName][]randomizedName{}
	for db, colls := range namespaces {
		// APP-5672: We choose 20 random lower case characters as a simple heuristic to avoid
		// collisions. We use lower case letters specifically because MongoDB, for historical
		// reasons, disallows creating a database with the same letters but different capitalization.
		newDBName := randomizedName{ptr: db, from: *db, to: "test-" + strings.ToLower(utils.RandomAlphaString(20))}
		oldNamespaces[newDBName] = nil
		for _, coll := range colls {
			newCollName := randomizedName{ptr: coll, from: *coll, to: strings.ToLower(utils.RandomAlphaString(20))}
			oldNamespaces[newDBName] = append(oldNamespaces[newDBName], newCollName)
			*coll = newCollName.to
		}
		*db = newDBName.to
	}

	return getNamespaces(), func() {
		for db, colls := range oldNamespaces {
			*db.ptr = db.from
			for _, coll := range colls {
				*coll.ptr = coll.from
			}
		}
	}
}

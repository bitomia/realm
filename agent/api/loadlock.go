package api

import "sync"

var provisionLocks sync.Map

func lockLoad(loadName string) func() {
	value, _ := provisionLocks.LoadOrStore(loadName, &sync.Mutex{})
	mutex := value.(*sync.Mutex)
	mutex.Lock()
	return mutex.Unlock
}

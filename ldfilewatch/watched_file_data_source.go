// Package ldfilewatch allows the LaunchDarkly client to read feature flag data from
// a file, with automatic reloading.
package ldfilewatch

import (
	"fmt"
	"path"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"

	ld "gopkg.in/launchdarkly/go-client.v4"
)

const retryDuration = time.Second

// WatchFiles sets up a mechanism for FileDataSource to reload its source files whenever one of them has
// been modified. Use it as follows:
//
//     fileSource, err := ldfiledata.NewFileDataSource(featureStore,
//         ldfiledata.FilePaths("./test-data/my-flags.json"),
//         ldfiledata.UseReloader(ldfilewatch.WatchFiles))
func WatchFiles(paths []string, logger ld.Logger, reload func(), closeCh <-chan struct{}) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("Unable to create file watcher: %s", err)
	}

	retryChannel := make(chan struct{}, 1)
	scheduleRetry := func() {
		time.AfterFunc(retryDuration, func() {
			select {
			case retryChannel <- struct{}{}: // don't need multiple retries so no need to block
			default:
			}
		})
	}
	go func() {
		for {
			realPaths := map[string]bool{}
			for _, p := range paths {
				absDirPath := path.Dir(p)
				realDirPath, err := filepath.EvalSymlinks(absDirPath)
				if err != nil {
					logger.Printf(`Unable to evaluate symlinks for "%s": %s`, absDirPath, err)
					scheduleRetry()
				}

				realPath := path.Join(realDirPath, path.Base(p))
				realPaths[realPath] = true
				_ = watcher.Add(realPath) // ok if this doesn't find the file: we're still watching the directory

				if err = watcher.Add(realDirPath); err != nil {
					logger.Printf(`Unable to watch directory "%s" for file "%s": %s`, realDirPath, p, err)
					scheduleRetry()
				}
			}

			// We do the reload here rather than after WaitForUpdates, even though that means there will be a
			// redundant load when we first start up, because otherwise there's a potential race condition where
			// file changes could happen before we had set up our file watcher.
			reload()

		WaitForUpdates:
			for {
				select {
				case <-closeCh:
					err := watcher.Close()
					if err != nil {
						logger.Printf("Error closing Watcher: %s", err)
					}
					return
				case event := <-watcher.Events:
					if realPaths[event.Name] {
						// Consume extra events
					ConsumeExtraEvents:
						for {
							select {
							case <-watcher.Events:
							default:
								break ConsumeExtraEvents
							}
						}
						break WaitForUpdates
					}
				case err := <-watcher.Errors:
					logger.Println("ERROR: ", err)
				case <-retryChannel:
				ConsumeExtraRetries:
					for {
						select {
						case <-retryChannel:
						default:
							break ConsumeExtraRetries
						}
					}
					break WaitForUpdates
				}
			}
		}
	}()

	return nil
}

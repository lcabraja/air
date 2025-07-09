package runner

import (
	"fmt"
	"path/filepath"
	"strings"
)

// mainLog prints log messages with the "main" color scheme
func (e *Engine) mainLog(format string, v ...interface{}) {
	if e.logger == nil {
		fmt.Printf(format+"\n", v...)
		return
	}
	e.logger.main()(format, v...)
}

// mainDebug prints debug log messages
func (e *Engine) mainDebug(format string, v ...interface{}) {
	if !e.debugMode {
		return
	}
	e.mainLog(format, v...)
}

// withLock executes a function while holding the engine's mutex
func (e *Engine) withLock(fn func()) {
	e.mu.Lock()
	defer e.mu.Unlock()
	fn()
}

// isExcludeDir checks if a directory should be excluded from watching
func (e *Engine) isExcludeDir(path string) bool {
	cleanPath := filepath.Clean(path)
	for _, d := range e.config.Build.ExcludeDir {
		if cleanPath == d {
			return true
		}
	}
	return false
}

// isTmpDir checks if a directory is the temporary build directory
func (e *Engine) isTmpDir(path string) bool {
	return path == e.config.tmpPath()
}

// isTestDataDir checks if a directory is a testdata directory
func (e *Engine) isTestDataDir(path string) bool {
	return strings.HasSuffix(path, "testdata")
}

// isIncludeExt checks if a file has an extension that should be watched
func (e *Engine) isIncludeExt(path string) bool {
	ext := filepath.Ext(path)
	for _, v := range e.config.Build.IncludeExt {
		if ext == "."+v {
			return true
		}
	}
	return false
}

// checkIncludeFile checks if a file should be included in watching
func (e *Engine) checkIncludeFile(path string) bool {
	for _, v := range e.config.Build.IncludeFile {
		if strings.HasSuffix(path, v) {
			return true
		}
	}
	return false
}

// isExcludeFile checks if a file should be excluded from watching
func (e *Engine) isExcludeFile(path string) bool {
	for _, v := range e.config.Build.ExcludeFile {
		if strings.HasSuffix(path, v) {
			return true
		}
	}
	return false
}

// isExcludeRegex checks if a file matches any exclude regex patterns
func (e *Engine) isExcludeRegex(path string) (bool, error) {
	for _, reg := range e.config.Build.ExcludeRegex {
		if matched, err := filepath.Match(reg, filepath.Base(path)); err != nil {
			return false, err
		} else if matched {
			return true, nil
		}
	}
	return false, nil
}

// checkIncludeDir checks if a directory should be included in watching
func (e *Engine) checkIncludeDir(path string) (bool, bool) {
	cleanPath := filepath.Clean(path)

	// If no IncludeDir is specified, include all directories
	if len(e.config.Build.IncludeDir) == 0 {
		return true, true
	}

	for _, d := range e.config.Build.IncludeDir {
		if cleanPath == d {
			return true, true
		}
		if strings.HasPrefix(cleanPath, d) {
			return false, true
		}
	}

	return false, false
}

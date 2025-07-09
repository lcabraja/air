package runner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
)

type Engine struct {
	config     *Config
	watcher    *fsnotify.Watcher
	running    atomic.Bool
	currentCmd *exec.Cmd
	mu         sync.Mutex
	debugMode  bool
	logger     *logger
	ll         sync.Mutex // lock for logger

	// Channels for control
	eventCh chan string
	exitCh  chan bool
}

func NewEngine(cfgPath string, args map[string]TomlInfo, debugMode bool) (*Engine, error) {
	cfg, err := InitConfig(cfgPath, args)
	if err != nil {
		return nil, err
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	e := &Engine{
		config:    cfg,
		watcher:   watcher,
		eventCh:   make(chan string, 1000),
		exitCh:    make(chan bool),
		debugMode: debugMode,
		logger:    newLogger(cfg),
	}

	return e, nil
}

func (e *Engine) Run() {
	if err := e.setupWatcher(); err != nil {
		fmt.Printf("Error setting up watcher: %v\n", err)
		return
	}
	defer e.watcher.Close()

	e.running.Store(true)
	firstRun := make(chan bool, 1)
	firstRun <- true

	for {
		select {
		case <-e.exitCh:
			fmt.Println("Shutting down...")
			return
		case event := <-e.watcher.Events:
			if !e.isValidChange(event) {
				continue
			}
			fmt.Printf("File changed: %s\n", event.Name)
			e.rebuild()
		case err := <-e.watcher.Errors:
			fmt.Printf("Watcher error: %v\n", err)
		case <-firstRun:
			e.rebuild()
		}
	}
}

func (e *Engine) setupWatcher() error {
	// Watch only specific directories
	dirsToWatch := []string{".", "app", "cmd", "internal", "pkg", "src"}
	for _, dir := range dirsToWatch {
		if _, err := os.Stat(dir); err == nil {
			if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if info.IsDir() && !e.isExcluded(path) {
					return e.watcher.Add(path)
				}
				return nil
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *Engine) isExcluded(path string) bool {
	excludeDirs := []string{".git", "node_modules", "vendor", "_air", "tmp"}
	base := filepath.Base(path)
	for _, dir := range excludeDirs {
		if base == dir {
			return true
		}
	}
	return false
}

func (e *Engine) isValidChange(event fsnotify.Event) bool {
	// Only trigger on write or create
	if event.Op&fsnotify.Write != fsnotify.Write && event.Op&fsnotify.Create != fsnotify.Create {
		return false
	}

	// Only watch .go files
	return filepath.Ext(event.Name) == ".go"
}

func (e *Engine) rebuild() {
	e.killCurrentProcess()

	// Build
	e.mainLog("Building...")
	buildCmd := exec.Command("go", "build", "-o", e.config.Build.Bin)
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		e.mainLog("Build failed: %v", err)
		return
	}

	// Run
	e.mainLog("Running...")
	e.mu.Lock()
	cmd := exec.Command(e.config.Build.Bin)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		e.mainLog("Failed to start process: %v", err)
		e.mu.Unlock()
		return
	}

	e.currentCmd = cmd
	e.mu.Unlock()

	// Wait in a goroutine
	go func() {
		cmd.Wait()
	}()
}

func (e *Engine) killCurrentProcess() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.currentCmd != nil && e.currentCmd.Process != nil {
		e.mainLog("Killing process %d", e.currentCmd.Process.Pid)

		// Try to kill the process group first
		if err := syscall.Kill(-e.currentCmd.Process.Pid, syscall.SIGINT); err == nil {
			time.Sleep(100 * time.Millisecond)
		}

		// Force kill if still running
		if err := syscall.Kill(-e.currentCmd.Process.Pid, syscall.SIGKILL); err != nil {
			// If process group kill failed, try direct process kill
			_ = syscall.Kill(e.currentCmd.Process.Pid, syscall.SIGKILL)
		}

		e.currentCmd.Wait()
		e.currentCmd = nil
	}
}

func (e *Engine) Stop() {
	e.killCurrentProcess()
	close(e.exitCh)
	e.running.Store(false)
}

// TriggerRefresh manually triggers a rebuild
func (e *Engine) TriggerRefresh() {
	e.rebuild()
}

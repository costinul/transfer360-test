package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type PubSubEmulator struct {
	ProjectID string
	Port      int
	DataDir   string
	hostPort  string
	cmd       *exec.Cmd
	mutex     sync.Mutex
	isRunning bool
	errChan   chan error
}

func NewPubSubEmulator(projectID string, port int) *PubSubEmulator {
	dataDir := filepath.Join(os.TempDir(), "pubsub-emulator-data")

	return &PubSubEmulator{
		ProjectID: projectID,
		Port:      port,
		DataDir:   dataDir,
		isRunning: false,
		errChan:   make(chan error, 1),
	}
}

func (em *PubSubEmulator) Start(ctx context.Context) error {
	em.mutex.Lock()
	defer em.mutex.Unlock()

	if em.isRunning {
		return fmt.Errorf("emulator already running")
	}

	if err := em.initializeDirectory(); err != nil {
		return err
	}

	if err := em.prepareCommand(); err != nil {
		return err
	}

	readyCh, errorCh := em.startMonitoring(ctx)

	err := em.waitForEmulator(ctx, readyCh, errorCh)
	if err != nil {
		return err
	}

	os.Setenv("PUBSUB_EMULATOR_HOST", em.Host())

	return nil
}

func (em *PubSubEmulator) initializeDirectory() error {
	if err := os.MkdirAll(em.DataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}
	return nil
}

func (em *PubSubEmulator) prepareCommand() error {
	hostPort := fmt.Sprintf("localhost:%d", em.Port)
	em.hostPort = hostPort

	em.cmd = exec.Command("gcloud", "beta", "emulators", "pubsub", "start",
		"--project="+em.ProjectID,
		"--host-port="+hostPort,
		"--data-dir="+em.DataDir)

	return nil
}

func (em *PubSubEmulator) startMonitoring(ctx context.Context) (chan struct{}, chan error) {
	stderr, err := em.cmd.StderrPipe()
	if err != nil {
		return nil, nil
	}

	stdout, err := em.cmd.StdoutPipe()
	if err != nil {
		return nil, nil
	}

	if err := em.cmd.Start(); err != nil {
		return nil, nil
	}

	// Channel to signal when the emulator is ready
	readyCh := make(chan struct{})

	// Channel to collect errors from monitoring goroutines
	errorCh := make(chan error, 2)

	// Monitor stderr
	go func() {
		select {
		case <-ctx.Done():
			return
		default:
			em.monitorOutput(stderr, readyCh, errorCh, true)
		}
	}()

	go func() {
		select {
		case <-ctx.Done():
			return
		default:
			em.monitorOutput(stdout, nil, errorCh, false)
		}
	}()

	go func() {
		select {
		case <-ctx.Done():
			return
		default:
			em.monitorProcess(errorCh)
		}
	}()

	return readyCh, errorCh
}

// monitorOutput monitors emulator output for ready signal or errors
func (em *PubSubEmulator) monitorOutput(
	reader io.Reader,
	readyCh chan struct{},
	errorCh chan error,
	checkReady bool,
) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Println(line)

		// Check for ready signal if this is the stream we're monitoring for it
		if checkReady && readyCh != nil && strings.Contains(line, "Server started") {
			select {
			case readyCh <- struct{}{}:
			default:
			}
		}

		// Check for error conditions in all streams
		if strings.Contains(line, "Address already in use") ||
			strings.Contains(line, "BindException") ||
			strings.Contains(line, "already in use: bind") {
			select {
			case errorCh <- fmt.Errorf("emulator failed to start: port %d already in use", em.Port):
			default:
			}
			return
		}
	}

	if err := scanner.Err(); err != nil {
		select {
		case errorCh <- fmt.Errorf("error reading emulator output: %w", err):
		default:
		}
	}
}

func (em *PubSubEmulator) monitorProcess(errorCh chan error) {
	startTime := time.Now()
	err := em.cmd.Wait()

	// Check if this is an early exit
	if time.Since(startTime) < 3*time.Second {
		errorCh <- fmt.Errorf("emulator process exited immediately: %v", err)
		return
	}

	em.mutex.Lock()
	defer em.mutex.Unlock()

	if em.isRunning {
		em.isRunning = false
		select {
		case em.errChan <- fmt.Errorf("emulator process exited unexpectedly: %v", err):
		default:
		}
	}
}

func (em *PubSubEmulator) waitForEmulator(
	ctx context.Context,
	readyCh chan struct{},
	errorCh chan error,
) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	select {
	case <-readyCh:
		em.isRunning = true
		return nil
	case err := <-errorCh:
		em.stopUnlocked()
		return err
	case <-time.After(3 * time.Second):
		select {
		case err := <-errorCh:
			em.stopUnlocked()
			return err
		default:
		}
	case <-ctx.Done():
		em.stopUnlocked()
		return ctx.Err()
	}

	select {
	case <-readyCh:
		em.isRunning = true
		return nil
	case err := <-errorCh:
		em.stopUnlocked()
		return err
	case <-timeoutCtx.Done():
		em.stopUnlocked()
		return fmt.Errorf("timeout waiting for emulator to start (no 'Server started' message detected within 30 seconds)")
	case <-ctx.Done():
		em.stopUnlocked()
		return ctx.Err()
	}
}

// stopUnlocked stops the emulator without acquiring the mutex
func (em *PubSubEmulator) stopUnlocked() {
	if !em.isRunning {
		return
	}

	fmt.Println("Stopping Pub/Sub emulator...")

	// Try multiple approaches to ensure all processes are terminated

	// 1. Try to kill the main process if we still have a reference
	if em.cmd != nil && em.cmd.Process != nil {
		fmt.Println("Killing main emulator process...")
		em.cmd.Process.Kill()
	}

	// 2. Kill processes by port
	fmt.Printf("Killing processes using port %d...\n", em.Port)
	if runtime.GOOS == "windows" {
		exec.Command("cmd", "/c", fmt.Sprintf("for /f \"tokens=5\" %%a in ('netstat -aon ^| findstr :%d') do taskkill /F /PID %%a", em.Port)).Run()
	} else {
		exec.Command("lsof", "-ti", fmt.Sprintf(":%d", em.Port)).Output()
		exec.Command("kill", "-9", fmt.Sprintf("$(lsof -ti :%d)", em.Port)).Run()
	}

	// 3. Kill Java processes
	fmt.Println("Killing Java processes...")
	if runtime.GOOS == "windows" {
		exec.Command("taskkill", "/F", "/IM", "java.exe").Run()
	} else {
		exec.Command("pkill", "-f", "java").Run()
	}

	// 4. Kill emulator processes
	fmt.Println("Killing emulator processes...")
	if runtime.GOOS == "windows" {
		exec.Command("taskkill", "/F", "/FI", "WINDOWTITLE eq *emulator*").Run()
		exec.Command("taskkill", "/F", "/FI", "WINDOWTITLE eq *pubsub*").Run()
		exec.Command("taskkill", "/F", "/FI", "WINDOWTITLE eq *gcloud*").Run()
	} else {
		exec.Command("pkill", "-f", "emulator").Run()
		exec.Command("pkill", "-f", "pubsub").Run()
		exec.Command("pkill", "-f", "gcloud").Run()
	}

	// 5. Kill by parent PID if available
	if em.cmd != nil && em.cmd.Process != nil {
		fmt.Printf("Killing process tree with parent PID %d...\n", em.cmd.Process.Pid)
		if runtime.GOOS == "windows" {
			exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", em.cmd.Process.Pid)).Run()
		} else {
			exec.Command("pkill", "-P", fmt.Sprintf("%d", em.cmd.Process.Pid)).Run()
		}
	}

	os.Unsetenv("PUBSUB_EMULATOR_HOST")
	em.isRunning = false
	fmt.Println("Pub/Sub emulator stopped")
}

func (em *PubSubEmulator) Stop() {
	em.mutex.Lock()
	defer em.mutex.Unlock()
	em.stopUnlocked()
}

func (em *PubSubEmulator) Host() string {
	return em.hostPort
}

func (em *PubSubEmulator) IsRunning() bool {
	em.mutex.Lock()
	defer em.mutex.Unlock()
	return em.isRunning
}

func (em *PubSubEmulator) Error() <-chan error {
	return em.errChan
}

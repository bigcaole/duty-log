package scheduler

import (
	"sync"
	"sync/atomic"
	"testing"
)

func resetBackupRunGuard() {
	backupRunGuard.mu.Lock()
	backupRunGuard.running = false
	backupRunGuard.mu.Unlock()
}

func TestTryStartBackupJobLifecycle(t *testing.T) {
	resetBackupRunGuard()

	if ok := tryStartBackupJob(); !ok {
		t.Fatalf("expected first start to succeed")
	}
	if ok := tryStartBackupJob(); ok {
		t.Fatalf("expected second start to be blocked")
	}

	finishBackupJob()

	if ok := tryStartBackupJob(); !ok {
		t.Fatalf("expected start after finish to succeed")
	}
	finishBackupJob()
}

func TestTryStartBackupJobConcurrent(t *testing.T) {
	resetBackupRunGuard()

	var successCount int32
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if tryStartBackupJob() {
				atomic.AddInt32(&successCount, 1)
			}
		}()
	}
	wg.Wait()

	if got := atomic.LoadInt32(&successCount); got != 1 {
		t.Fatalf("expected only one successful start, got %d", got)
	}

	finishBackupJob()
}

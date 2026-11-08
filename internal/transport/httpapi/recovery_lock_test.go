package httpapi

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestDOCXRecoveryLockReleasedAfterCrash(t *testing.T) {
	root := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestDOCXRecoveryLockChild$")
	cmd.Env = append(os.Environ(), "DOCX_RECOVERY_LOCK_TEST_ROOT="+root)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	line, err := bufio.NewReader(out).ReadString('\n')
	if err != nil || line != "locked\n" {
		t.Fatal("child did not acquire lock", line, err)
	}
	if unlock, err := lockRecovery(root); err == nil {
		unlock()
		t.Fatal("concurrent process acquired recovery lock")
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	unlock, err := lockRecovery(root)
	if err != nil {
		t.Fatal("crashed process retained lock", err)
	}
	unlock()
}

func TestDOCXRecoveryLockChild(t *testing.T) {
	root := os.Getenv("DOCX_RECOVERY_LOCK_TEST_ROOT")
	if root == "" {
		return
	}
	unlock, err := lockRecovery(root)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	fmt.Println("locked")
	time.Sleep(time.Minute)
}

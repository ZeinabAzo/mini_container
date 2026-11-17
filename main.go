package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: minicontainer run <image-path> <command>")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "run":
		parent()
	case "child":
		child()
	default:
		fmt.Println("Unknown command")
		os.Exit(1)
	}
}

func parent() {
	fmt.Printf("[DEBUG] Parent process started (PID: %d)\n", os.Getpid())

	cmd := exec.Command("/proc/self/exe", append([]string{"child"}, os.Args[2:]...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// ADDED: CLONE_NEWNS creates the isolated Mount namespace
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWIPC | syscall.CLONE_NEWNS,
	}

	if err := cmd.Run(); err != nil {
		fmt.Printf("[ERROR] Parent: Command failed: %v\n", err)
		os.Exit(1)
	}
}

func child() {
	fmt.Printf("[DEBUG] Child process waking up inside container (PID: %d)\n", os.Getpid())
	syscall.Sethostname([]byte("container-box"))

	if len(os.Args) < 4 {
		fmt.Println("Usage: minicontainer run <image-path> <command>")
		os.Exit(1)
	}
	imagePath := os.Args[2]
	cmdName := os.Args[3]
	cmdArgs := os.Args[4:]

	// Initialize the isolated filesystem
	fmt.Println("[DEBUG] Child: Starting filesystem setup...")
	setupFilesystem(imagePath)

	fmt.Printf("[DEBUG] Child: Running user command: %s %v\n", cmdName, cmdArgs)
	cmd := exec.Command(cmdName, cmdArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("[ERROR] Child: Command failed: %v\n", err)
		os.Exit(1)
	}
}

func setupFilesystem(imagePath string) {
	// 1. Prevent mount operations inside the container from leaking to the host
	fmt.Println("[DEBUG] FS-1. Setting up private mount propagation...")
	syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, "")

	// 2. Set up OverlayFS directories
	fmt.Println("[DEBUG] FS-2. Creating OverlayFS directories...")
	merged := "overlay_merged"
	os.MkdirAll(imagePath, 0777)       // Lower directory (read-only alpine)
	os.MkdirAll("overlay_upper", 0777) // Upper directory (writable changes)
	os.MkdirAll("overlay_work", 0777)  // Work directory (used internally by OverlayFS)
	os.MkdirAll(merged, 0777)          // Merged directory (what the container sees)

	// 3. Mount OverlayFS
	fmt.Println("[DEBUG] FS-3. Mounting OverlayFS...")
	opts := fmt.Sprintf("lowerdir=%s,upperdir=overlay_upper,workdir=overlay_work", imagePath)
	syscall.Mount("overlay", merged, "overlay", 0, opts)

	// 4. Pivot root requires the new root to be a mount point itself, so we bind mount it
	fmt.Println("[DEBUG] FS-4. Binding merged directory to itself...")
	syscall.Mount(merged, merged, "bind", syscall.MS_BIND|syscall.MS_REC, "")

	// 5. Pivot root and detach the old host filesystem
	fmt.Println("[DEBUG] FS-5. Pivoting root...")
	os.MkdirAll(merged+"/oldroot", 0777)
	syscall.PivotRoot(merged, merged+"/oldroot")
	
	fmt.Println("[DEBUG] FS-6. Changing directory and cleaning up old root...")
	os.Chdir("/")
	syscall.Unmount("/oldroot", syscall.MNT_DETACH)
	os.RemoveAll("/oldroot")

	// 6. Mount /proc so tools like 'ps' and 'top' work inside the container
	fmt.Println("[DEBUG] FS-7. Mounting /proc...")
	os.MkdirAll("/proc", 0777)
	syscall.Mount("proc", "/proc", "proc", 0, "")
	fmt.Println("[DEBUG] FS-8. Filesystem setup complete!")
}
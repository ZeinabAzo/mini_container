package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: minicontainer run <command>")
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

// parent() runs on the host and sets up the boundaries
func parent() {
	fmt.Printf("Parent process running on host (PID: %d)\n", os.Getpid())

	// /proc/self/exe tells Linux to run a new copy of this exact same program
	// We pass "child" so it hits the other side of our switch statement
	cmd := exec.Command("/proc/self/exe", append([]string{"child"}, os.Args[2:]...)...)
	
	// Connect the container's input/output to our terminal
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// THE KERNEL MAGIC: We ask Linux to clone the process into new namespaces
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWIPC,
	}

	if err := cmd.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

// child() wakes up inside the isolated namespaces
func child() {
	fmt.Printf("Child process running in container (PID: %d)\n", os.Getpid())

	// Set the custom hostname (isolated by UTS namespace)
	syscall.Sethostname([]byte("container-box"))

	// Execute the command the user asked for (e.g., /bin/sh)
	if len(os.Args) < 3 {
		fmt.Println("Please provide a command to run (like /bin/sh)")
		os.Exit(1)
	}

	cmd := exec.Command(os.Args[2], os.Args[3:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("Command failed: %v\n", err)
		os.Exit(1)
	}
}
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"syscall"
)

type Container struct {
	Args []string
	Uid  int
	Gid  int
}

func (c *Container) Start() error {
	cmd := &exec.Cmd{
		Path:   "/proc/self/exe",
		Args:   append([]string{os.Args[0], "child"}, c.Args...),
		Env:    os.Environ(),
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		SysProcAttr: &syscall.SysProcAttr{
			Cloneflags: syscall.CLONE_NEWUSER |
				syscall.CLONE_NEWPID |
				syscall.CLONE_NEWUTS |
				syscall.CLONE_NEWNS |
				syscall.CLONE_NEWNET |
				syscall.CLONE_NEWIPC,
			UidMappings: []syscall.SysProcIDMap{
				{
					ContainerID: 0,
					HostID:      c.Uid,
					Size:        1,
				},
			},
			GidMappings: []syscall.SysProcIDMap{
				{
					ContainerID: 0,
					HostID:      c.Gid,
					Size:        1,
				},
			},
		},
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error starting container: %v", err)
	}
	log.Printf("container pid: %d\n", cmd.Process.Pid)
	return cmd.Wait()
}

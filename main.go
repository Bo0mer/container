package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func main() {
	switch os.Args[1] {
	case "run":
		parent()
	case "child":
		child()
	default:
		panic(fmt.Sprintf("What should i do? Args: %v", os.Args))
	}
}

func parent() {
	c := &Container{
		Args: os.Args[2:],
		Uid:  os.Getuid(),
		Gid:  os.Getgid(),
	}
	log.Fatal(c.Start())
}

func child() {
	mounts := []Mount{
		Mount{
			Source: "proc",
			Target: "/proc",
			FsType: "proc",
			Flags:  syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV,
		},
		Mount{
			Source: "tmpfs",
			Target: "/dev",
			FsType: "tmpfs",
			Flags:  syscall.MS_NOSUID | syscall.MS_STRICTATIME,
			Data:   "mode=755",
		},
		Mount{
			Source: "sysfs",
			Target: "/sys",
			FsType: "sysfs",
			Flags:  syscall.MS_NOSUID | syscall.MS_NOEXEC | syscall.MS_NODEV | syscall.MS_RDONLY,
		},
	}

	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("error getting working directory: %v\n", err)
	}
	mount(mounts, wd)

	// set rootfs
	if err := pivotRoot(wd); err != nil {
		log.Fatalf("error pivot root on %s: %v\n", wd, err)
	}

	// set hostname
	setHostname("container")
	cmd := &exec.Cmd{
		Path:   os.Args[2],
		Args:   os.Args[2:],
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Env:    os.Environ(),
	}
	if err := cmd.Start(); err != nil {
		log.Fatalf("error running %s: %v\n", cmd.Path, err)
	}
	if err := cmd.Wait(); err != nil {
		log.Fatalf("error waiting %s: %v\n", cmd.Path, err)
	}
}

func setHostname(hostname string) {
	if err := syscall.Sethostname([]byte(hostname)); err != nil {
		log.Fatalf("error setting hostname: %v\n", err)
	}
}

type Mount struct {
	Source string
	Target string
	FsType string
	Flags  uintptr
	Data   string
}

func mount(mounts []Mount, root string) {
	for _, m := range mounts {
		target := filepath.Join(root, m.Target)
		if err := syscall.Mount(m.Source, target, m.FsType, m.Flags, m.Data); err != nil {
			log.Fatalf("error mounting %s: %v\n", target, err)
		}
		log.Printf("mounted %s\n", target)
	}
}

func pivotRoot(root string) error {
	// we need this to satisfy restriction:
	// "new_root and put_old must not be on the same filesystem as the current root"
	if err := syscall.Mount(root, root, "", syscall.MS_BIND, ""); err != nil {
		return fmt.Errorf("Mount rootfs to itself error: %v", err)
	}
	// create rootfs/.pivot_root as path for old_root
	pivotDir := filepath.Join(root, ".pivot_root")
	if err := os.Mkdir(pivotDir, 0777); err != nil {
		return err
	}
	// pivot_root to rootfs, now old_root is mounted in rootfs/.pivot_root
	// mounts from it still can be seen in `mount`
	if err := syscall.PivotRoot(root, pivotDir); err != nil {
		return fmt.Errorf("pivot_root %v", err)
	}
	// change working directory to /
	// it is recommendation from man-page
	if err := syscall.Chdir("/"); err != nil {
		return fmt.Errorf("chdir / %v", err)
	}
	// path to pivot root now changed, update
	pivotDir = filepath.Join("/", ".pivot_root")
	// umount rootfs/.pivot_root(which is now /.pivot_root) with all submounts
	// now we have only mounts that we mounted ourselves in `mount`
	if err := syscall.Unmount(pivotDir, syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("unmount pivot_root dir %v", err)
	}
	// remove temporary directory
	return os.Remove(pivotDir)
}

//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	vfuse "github.com/eniz1806/VaultS3/internal/fuse"
)

func runMount(args []string) {
	if len(args) < 2 {
		fmt.Println(`Usage: vaults3-cli mount <bucket> <mountpoint>

Mount a VaultS3 bucket as a local filesystem directory.

Examples:
  vaults3-cli mount my-bucket /mnt/vaults3
  vaults3-cli mount my-bucket ./mnt`)
		os.Exit(1)
	}

	requireCreds()

	bucket := args[0]
	mountpoint := args[1]

	// Create mountpoint if it doesn't exist
	if err := os.MkdirAll(mountpoint, 0755); err != nil {
		fatal(fmt.Sprintf("create mountpoint: %v", err))
	}

	cfg := vfuse.MountConfig{
		Endpoint:  endpoint,
		AccessKey: accessKey,
		SecretKey: secretKey,
		Bucket:    bucket,
		Region:    region,
	}

	fmt.Printf("Mounting %s at %s (endpoint: %s)\n", bucket, mountpoint, endpoint)
	fmt.Println("Press Ctrl+C to unmount")

	server, err := vfuse.Mount(mountpoint, cfg)
	if err != nil {
		fatal(fmt.Sprintf("mount failed: %v", err))
	}

	// Handle Ctrl+C for clean unmount
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\nUnmounting...")
		server.Unmount()
	}()

	server.Wait()
	fmt.Println("Unmounted")
}

func runUmount(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: vaults3-cli umount <mountpoint>")
		os.Exit(1)
	}
	// On macOS/Linux, use fusermount or umount
	fmt.Printf("To unmount, run: fusermount -u %s\n", args[0])
}

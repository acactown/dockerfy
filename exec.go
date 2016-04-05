package main

import (
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/net/context"
)

func runCmd(ctx context.Context, cancel context.CancelFunc, cmd string, args ...string) {
	defer wg.Done()

	process := exec.Command(cmd, args...)
	process.Stdin = os.Stdin
	process.Stdout = os.Stdout
	process.Stderr = os.Stderr

	// start the process
	err := process.Start()
	if err != nil {
		log.Fatalf("Error starting command: `%s` - %s\n", toString(process), err)
	}

	// Setup signaling -- a separate channel for goroutine for each command
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGKILL)

	wg.Add(1)
	go func() {
		defer wg.Done()

		select {
		case sig := <-sigs:
			log.Printf("Received signal: %s\n", sig)
			signalProcessWithTimeout(process, sig)
			if cancel != nil {
				cancel()
			}
		case <-ctx.Done():
			log.Printf("Done waiting for signals")
			// exit when context is done
		}
	}()

	err = process.Wait()

	if err == nil {
		log.Println("Command finished successfully: " + toString(process))
	} else {
		log.Printf("Command `%s` exited with error: %s\n", toString(process), err)
		if cancel != nil {
			cancel()
		}
		// OPTIMIZE: This could be cleaner
		// os.Exit(err.(*exec.ExitError).Sys().(syscall.WaitStatus).ExitStatus())
	}
}

func signalProcessWithTimeout(process *exec.Cmd, sig os.Signal) {
	done := make(chan struct{})

	go func() {
		process.Process.Signal(sig) // pretty sure this doesn't do anything. It seems like the signal is automatically sent to the command?
		process.Wait()
		close(done)
	}()
	select {
	case <-done:
		return
	case <-time.After(10 * time.Second):
		log.Println("Killing command due to timeout.")
		process.Process.Kill()
	}
}
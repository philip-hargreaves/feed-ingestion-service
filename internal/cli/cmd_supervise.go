package cli

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

func handlerSupervise(s *state, cmd command) error {
	if len(cmd.args) == 0 {
		return newUsageError("Supervise requires agg arguments", "feeder supervise <time_between_reqs> [workers] [batch_size] [domain_delay]")
	}

	executablePathFn := s.executablePath
	if executablePathFn == nil {
		executablePathFn = os.Executable
	}

	executablePath, err := executablePathFn()
	if err != nil {
		return fmt.Errorf("Couldn't determine executable path: %w", err)
	}

	const (
		maxRestartsInWindow = 5
		restartWindow       = time.Minute
		baseBackoff         = time.Second
		maxBackoff          = 30 * time.Second
	)

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signalChan)

	var restartTimes []time.Time
	restartAttempt := 0

	for {
		childArgs := append([]string{"agg"}, cmd.args...)
		childCmd := exec.Command(executablePath, childArgs...)
		childCmd.Stdout = os.Stdout
		childCmd.Stderr = os.Stderr
		childCmd.Stdin = os.Stdin

		err := childCmd.Start()
		if err != nil {
			return fmt.Errorf("Couldn't start agg child process: %w", err)
		}

		fmt.Printf("Supervisor started agg (pid=%d)\n", childCmd.Process.Pid)

		childDone := make(chan error, 1)
		go func() {
			childDone <- childCmd.Wait()
		}()

		startedAt := time.Now()

		select {
		case receivedSignal := <-signalChan:
			fmt.Printf("Supervisor received signal %s. Stopping agg.\n", receivedSignal.String())
			_ = childCmd.Process.Signal(receivedSignal)

			select {
			case <-childDone:
			case <-time.After(10 * time.Second):
				fmt.Println("Agg did not stop gracefully, killing process.")
				_ = childCmd.Process.Kill()
				<-childDone
			}
			return nil

		case waitErr := <-childDone:
			if waitErr == nil {
				fmt.Println("Agg exited cleanly. Supervisor stopping.")
				return nil
			}

			now := time.Now()
			restartTimes = append(restartTimes, now)
			filtered := restartTimes[:0]
			for _, t := range restartTimes {
				if now.Sub(t) <= restartWindow {
					filtered = append(filtered, t)
				}
			}
			restartTimes = filtered

			if len(restartTimes) > maxRestartsInWindow {
				return fmt.Errorf("Agg crashed too often (%d times in %s), supervisor stopping", len(restartTimes), restartWindow)
			}

			if time.Since(startedAt) > restartWindow {
				restartAttempt = 0
			}

			backoff := baseBackoff * time.Duration(1<<restartAttempt)
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			if restartAttempt < 30 {
				restartAttempt++
			}

			fmt.Printf("Agg crashed: %v\n", waitErr)
			fmt.Printf("Restarting agg in %s...\n", backoff)

			timer := time.NewTimer(backoff)
			select {
			case receivedSignal := <-signalChan:
				timer.Stop()
				fmt.Printf("Supervisor received signal %s during backoff. Exiting.\n", receivedSignal.String())
				return nil
			case <-timer.C:
			}
		}
	}
}

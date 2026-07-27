//go:build windows

package wslhost

// BindWindowsStopWaiters installs real waiters on an existing coordinator.
func BindWindowsStopWaiters(c *StopCoordinator) {
	if c == nil {
		return
	}
	c.Wait, c.Sleeper = RealGracefulWaiter{}, RealSleeper{}
}

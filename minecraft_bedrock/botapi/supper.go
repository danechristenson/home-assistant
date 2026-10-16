package main

// Supper time: a countdown message to everyone each minute, then the
// tracked players (everyone, if none are tracked) are kicked and kept out
// for a while: if they rejoin, they're kicked again.

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

const (
	supperCountdown = 5               // minutes of warnings before the kick
	supperLockout   = 1 * time.Minute // how long they're kept out after
)

var supper = struct {
	sync.Mutex
	cancel       chan struct{}
	kickAt       time.Time // while counting down
	lockoutUntil time.Time // after the kick
	minute       time.Duration
	// How long to wait before kicking someone who rejoins during the lockout
	// (kicking while they're still joining doesn't work)
	rejoinDelay time.Duration
}{minute: time.Minute, rejoinDelay: 3 * time.Second}

// supperState is what Home Assistant shows: "off", "countdown" or "lockout"
func supperState() map[string]any {
	supper.Lock()
	defer supper.Unlock()
	now := time.Now()
	switch {
	case supper.cancel != nil && now.Before(supper.kickAt):
		left := int((supper.kickAt.Sub(now) + supper.minute - 1) / supper.minute)
		return map[string]any{"state": "countdown", "minutes_left": left, "until": supper.kickAt.UTC().Format(time.RFC3339)}
	case now.Before(supper.lockoutUntil):
		return map[string]any{"state": "lockout", "minutes_left": 0, "until": supper.lockoutUntil.UTC().Format(time.RFC3339)}
	}
	return map[string]any{"state": "off", "minutes_left": 0, "until": nil}
}

// startSupper begins the countdown; it does nothing if one is running
func startSupper() error {
	if !serverRunning() {
		return errNotRunning
	}
	supper.Lock()
	if supper.cancel != nil {
		supper.Unlock()
		return nil
	}
	cancel := make(chan struct{})
	minute := supper.minute
	supper.cancel = cancel
	supper.kickAt = time.Now().Add(supperCountdown * minute)
	supper.Unlock()
	log.Printf("[INFO] Supper time: %d minute countdown", supperCountdown)
	requestPublish()

	go func() {
		for left := supperCountdown; left > 0; left-- {
			unit := "minutes"
			if left == 1 {
				unit = "minute"
			}
			_ = console(fmt.Sprintf("say Supper time in %d %s! Finish up and save your things.", left, unit))
			requestPublish()
			select {
			case <-cancel:
				return
			case <-time.After(minute):
			}
		}
		supper.Lock()
		if supper.cancel != cancel {
			supper.Unlock()
			return
		}
		supper.cancel = nil
		supper.lockoutUntil = time.Now().Add(supperLockout * minute / time.Minute)
		supper.Unlock()
		_ = console("say Supper time!")
		for _, player := range onlinePlayers() {
			if supperTarget(player.name, player.xuid) {
				_ = console(`kick "` + player.name + `" Supper time!`)
			}
		}
		log.Printf("[INFO] Supper time: kicked, keeping them out until %s", time.Now().Add(supperLockout*minute/time.Minute).Format("15:04"))
		requestPublish()
		time.Sleep(supperLockout*minute/time.Minute + time.Second)
		requestPublish() // the lockout has ended
	}()
	return nil
}

func cancelSupper() {
	supper.Lock()
	wasCounting := supper.cancel != nil
	if supper.cancel != nil {
		close(supper.cancel)
		supper.cancel = nil
	}
	supper.lockoutUntil = time.Time{}
	supper.Unlock()
	if wasCounting && serverRunning() {
		_ = console("say Supper time is off. Keep playing!")
	}
	log.Printf("[INFO] Supper time cancelled")
	requestPublish()
}

// supperTarget is whether supper time applies to a player: tracked players,
// or everyone when nobody is tracked
func supperTarget(gamertag, xuid string) bool {
	stats.Lock()
	defer stats.Unlock()
	if len(stats.tracked) == 0 {
		return true
	}
	name := trackedName(gamertag, xuid)
	for _, tracked := range stats.tracked {
		if strings.EqualFold(tracked, name) {
			return true
		}
	}
	return false
}

// supperJoin kicks a supper-time player who rejoins during the lockout
func supperJoin(gamertag, xuid string) {
	supper.Lock()
	locked, delay := time.Now().Before(supper.lockoutUntil), supper.rejoinDelay
	supper.Unlock()
	if locked && supperTarget(gamertag, xuid) {
		go func() {
			time.Sleep(delay)
			_ = console(`kick "` + gamertag + `" It's still supper time!`)
		}()
	}
}

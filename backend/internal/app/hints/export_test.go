package hints

import "time"

// WarmUpAttempts и SetWarmUpPause открывают тестам предел попыток и паузу между ними.
const WarmUpAttempts = warmUpAttempts

func SetWarmUpPause(s *Service, d time.Duration) { s.warmUpPause = d }

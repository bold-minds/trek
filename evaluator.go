package trek

import (
	"sort"
	"time"
)

// Decide evaluates a request context against active sessions and returns a decision.
// This is a pure function with no I/O - suitable for conformance testing.
func Decide(now time.Time, serviceName string, ctx RequestContext, sessions []Session) Decision {
	if len(sessions) == 0 {
		return NoMatchDecision(ReasonNoMatch)
	}

	var candidates []Session
	var hadExpired bool

	for _, sess := range sessions {
		if !isSessionApplicable(now, serviceName, sess) {
			if sess.ExpiresAt.Before(now) || sess.ExpiresAt.Equal(now) {
				hadExpired = true
			}
			continue
		}

		if MatchSelector(sess.Selector, ctx) {
			candidates = append(candidates, sess)
		}
	}

	if len(candidates) == 0 {
		if hadExpired {
			return NoMatchDecision(ReasonExpired)
		}
		return NoMatchDecision(ReasonNoMatch)
	}

	winner := selectWinner(candidates)

	return Decision{
		Matched:        true,
		SessionID:      winner.ID,
		EffectiveLevel: winner.Level,
		ReasonCode:     ReasonMatched,
		Labels:         copyLabels(winner.Labels),
		Caps:           winner.Caps,
	}
}

// isSessionApplicable checks if a session should be considered for evaluation.
func isSessionApplicable(now time.Time, serviceName string, sess Session) bool {
	if sess.ExpiresAt.Before(now) || sess.ExpiresAt.Equal(now) {
		return false
	}

	if !isServiceInScope(serviceName, sess.ServiceScope) {
		return false
	}

	return true
}

// isServiceInScope checks if the current service is within the session's service scope.
func isServiceInScope(serviceName string, scope []string) bool {
	if scope == nil {
		return true
	}
	if len(scope) == 0 {
		return false
	}
	for _, s := range scope {
		if s == serviceName {
			return true
		}
	}
	return false
}

// selectWinner chooses the best session from matching candidates using tie-break rules.
func selectWinner(candidates []Session) Session {
	if len(candidates) == 1 {
		return candidates[0]
	}

	sort.Slice(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]

		if a.Level.Priority() != b.Level.Priority() {
			return a.Level.Priority() > b.Level.Priority()
		}

		specA := SelectorSpecificity(a.Selector)
		specB := SelectorSpecificity(b.Selector)
		if specA != specB {
			return specA > specB
		}

		if !a.ExpiresAt.Equal(b.ExpiresAt) {
			return a.ExpiresAt.Before(b.ExpiresAt)
		}

		return a.ID < b.ID
	})

	return candidates[0]
}

// copyLabels creates a copy of the labels map to avoid mutation.
func copyLabels(labels map[string]string) map[string]string {
	if labels == nil {
		return map[string]string{}
	}
	result := make(map[string]string, len(labels))
	for k, v := range labels {
		result[k] = v
	}
	return result
}

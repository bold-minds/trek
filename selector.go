package trek

import "strings"

// MatchSelector checks if a request context matches a selector.
// All specified selector fields must match (AND semantics).
func MatchSelector(sel Selector, ctx RequestContext) bool {
	if sel.UserID != "" && sel.UserID != ctx.UserID {
		return false
	}
	if sel.RequestID != "" && sel.RequestID != ctx.RequestID {
		return false
	}
	if sel.TenantID != "" && sel.TenantID != ctx.TenantID {
		return false
	}
	if sel.Route != "" && !matchRoute(sel.Route, ctx.Route) {
		return false
	}
	if len(sel.Custom) > 0 && !matchCustom(sel.Custom, ctx.Custom) {
		return false
	}
	return true
}

// matchRoute checks if a request route matches a selector route.
// Supports exact match and prefix match with trailing *.
func matchRoute(selectorRoute, requestRoute string) bool {
	if strings.HasSuffix(selectorRoute, "*") {
		prefix := strings.TrimSuffix(selectorRoute, "*")
		return strings.HasPrefix(requestRoute, prefix)
	}
	return selectorRoute == requestRoute
}

// matchCustom checks if all selector custom fields exist in request custom with matching values.
func matchCustom(selectorCustom, requestCustom map[string]string) bool {
	if requestCustom == nil {
		return len(selectorCustom) == 0
	}
	for key, selectorValue := range selectorCustom {
		requestValue, exists := requestCustom[key]
		if !exists || requestValue != selectorValue {
			return false
		}
	}
	return true
}

// SelectorSpecificity returns the count of non-null fields in a selector.
// Used for tie-breaking. Custom counts as 1 regardless of number of keys.
func SelectorSpecificity(sel Selector) int {
	count := 0
	if sel.UserID != "" {
		count++
	}
	if sel.RequestID != "" {
		count++
	}
	if sel.TenantID != "" {
		count++
	}
	if sel.Route != "" {
		count++
	}
	if len(sel.Custom) > 0 {
		count++
	}
	return count
}

// IsEmptySelector returns true if no selector fields are specified.
func IsEmptySelector(sel Selector) bool {
	return SelectorSpecificity(sel) == 0
}

package middleware

import (
	"net/http"
	"net/netip"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mcpx/boilerplate/core"
)

type ipRuleSet struct {
	allow    []netip.Prefix
	deny     []netip.Prefix
	allowAll bool
}

type IPFilter struct {
	store         *core.Store
	projectKey    string
	refreshEvery  time.Duration
	envRules      ipRuleSet
	mu            sync.Mutex
	cachedRules   ipRuleSet
	nextRefreshAt time.Time
}

func NewIPFilter(store *core.Store, projectKey string) *IPFilter {
	allowRaw := strings.TrimSpace(os.Getenv("IP_ALLOWLIST"))
	denyRaw := strings.TrimSpace(os.Getenv("IP_DENYLIST"))
	envRules := ipRuleSet{
		allow:    parseCIDRs(allowRaw),
		deny:     parseCIDRs(denyRaw),
		allowAll: isAllowAll(allowRaw),
	}
	return &IPFilter{
		store:        store,
		projectKey:   strings.TrimSpace(projectKey),
		envRules:     envRules,
		refreshEvery: time.Minute,
	}
}

func (f *IPFilter) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := strings.TrimSpace(c.ClientIP())
		addr, err := netip.ParseAddr(ip)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "ip_not_allowed"})
			return
		}

		rules := f.currentRules()
		if matchesPrefix(rules.deny, addr) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "ip_not_allowed"})
			return
		}
		if !rules.allowAll && len(rules.allow) > 0 && !matchesPrefix(rules.allow, addr) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "ip_not_allowed"})
			return
		}
		c.Next()
	}
}

func (f *IPFilter) currentRules() ipRuleSet {
	if f.store == nil || f.projectKey == "" {
		return f.envRules
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if time.Now().Before(f.nextRefreshAt) {
		return mergeRules(f.envRules, f.cachedRules)
	}

	f.nextRefreshAt = time.Now().Add(f.refreshEvery)
	rows, err := f.store.ListIPRules(f.projectKey)
	if err != nil {
		return mergeRules(f.envRules, f.cachedRules)
	}

	allow := []netip.Prefix{}
	deny := []netip.Prefix{}
	for _, row := range rows {
		if !row.Enabled {
			continue
		}
		prefixes := parseCIDRs(row.CIDR)
		if len(prefixes) == 0 {
			continue
		}
		if strings.ToLower(strings.TrimSpace(row.RuleType)) == "deny" {
			deny = append(deny, prefixes...)
			continue
		}
		allow = append(allow, prefixes...)
	}

	f.cachedRules = ipRuleSet{allow: allow, deny: deny, allowAll: false}
	return mergeRules(f.envRules, f.cachedRules)
}

func mergeRules(env ipRuleSet, db ipRuleSet) ipRuleSet {
	return ipRuleSet{
		allow:    append(append([]netip.Prefix{}, env.allow...), db.allow...),
		deny:     append(append([]netip.Prefix{}, env.deny...), db.deny...),
		allowAll: env.allowAll,
	}
}

func parseCIDRs(raw string) []netip.Prefix {
	out := []netip.Prefix{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" || part == "*" || strings.EqualFold(part, "all") {
			continue
		}
		if strings.Contains(part, "/") {
			if prefix, err := netip.ParsePrefix(part); err == nil {
				out = append(out, prefix)
			}
			continue
		}
		if addr, err := netip.ParseAddr(part); err == nil {
			bits := 32
			if addr.Is6() {
				bits = 128
			}
			out = append(out, netip.PrefixFrom(addr, bits))
		}
	}
	return out
}

func isAllowAll(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return true
	}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "*" || strings.EqualFold(part, "all") {
			return true
		}
	}
	return false
}

func matchesPrefix(prefixes []netip.Prefix, addr netip.Addr) bool {
	for _, prefix := range prefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

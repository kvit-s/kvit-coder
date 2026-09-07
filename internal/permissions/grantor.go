package permissions

import (
	"fmt"
	"sync"
)

// Grantor holds the grants that apply to a run and can add to them. The three
// stores are separate files with different lifetimes, and "once" is not written
// down at all.
type Grantor struct {
	mu      sync.Mutex
	session *Store
	project *Store
	global  *Store
	once    map[string]bool
}

// NewGrantor opens the three grant files. A nil store is fine — it contributes
// nothing and refuses grants at that scope.
func NewGrantor(session, project, global *Store) *Grantor {
	return &Grantor{
		session: session,
		project: project,
		global:  global,
		once:    map[string]bool{},
	}
}

// Rules renders every grant as policy rules, nearest scope first.
func (g *Grantor) Rules() []Rule {
	if g == nil {
		return nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()

	var out []Rule
	for pattern := range g.once {
		out = append(out, Rule{Pattern: pattern, Effect: EffectAllow, Source: "once",
			Reason: "it was allowed for this one command"})
	}
	out = append(out, g.session.Rules()...)
	out = append(out, g.project.Rules()...)
	out = append(out, g.global.Rules()...)
	return out
}

// Grant records permission for a pattern at the given scope.
func (g *Grantor) Grant(scope GrantScope, pattern string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	switch scope {
	case GrantOnce:
		g.once[pattern] = true
		return nil
	case GrantSession:
		return grantTo(g.session, "this session", pattern)
	case GrantProject:
		return grantTo(g.project, "this project", pattern)
	case GrantGlobal:
		return grantTo(g.global, "everywhere", pattern)
	}
	return fmt.Errorf("unknown grant scope %q", scope)
}

func grantTo(store *Store, what, pattern string) error {
	if store == nil {
		return fmt.Errorf("there is nowhere to record a grant for %s", what)
	}
	return store.Allow(pattern)
}

// Where says which file a grant at this scope is written to, so the person
// making it knows where to go to take it back.
func (g *Grantor) Where(scope GrantScope) string {
	switch scope {
	case GrantSession:
		if g.session != nil {
			return g.session.Path()
		}
	case GrantProject:
		if g.project != nil {
			return g.project.Path()
		}
	case GrantGlobal:
		if g.global != nil {
			return g.global.Path()
		}
	}
	return ""
}

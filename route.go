package fulfillment

type Route struct {
	handler    HandlerFunc
	matchers   []matcher
	middleware []MiddlewareFunc
}

type matcher interface {
	Match(*Context) bool
}

type MatcherFunc func(*Context) bool

func (f MatcherFunc) Match(c *Context) bool {
	return f(c)
}

func (r *Route) addMatcher(fn matcher) *Route {
	r.matchers = append(r.matchers, fn)
	return r
}

type messageAttributeMatcher struct {
	key, value string
}

func (m *messageAttributeMatcher) Match(c *Context) bool {
	return c.MessageAttribute(m.key) == m.value
}

func (r *Route) MessageAttribute(key, value string) *Route {
	return r.addMatcher(&messageAttributeMatcher{key, value})
}

type attributeMatcher struct {
	key, value string
}

func (m *attributeMatcher) Match(c *Context) bool {
	return c.Attribute(m.key) == m.value
}

func (r *Route) Attribute(key, value string) *Route {
	return r.addMatcher(&attributeMatcher{key, value})
}

func (r *Route) matches(c *Context) bool {
	for _, m := range r.matchers {
		if !m.Match(c) {
			return false
		}
	}
	return true
}

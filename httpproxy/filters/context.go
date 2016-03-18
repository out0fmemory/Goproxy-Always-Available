package filters

import (
	"net"
	"net/http"
	"strings"
)

const (
	VenderHeader string = "X-Vender-Info"
)

type VenderKey string

func (v VenderKey) String() string {
	return string(v)
}

type Context struct {
	ln           net.Listener
	rw           http.ResponseWriter
	venderString string
	venderValues map[VenderKey]string
	values       map[string]interface{}
	hijacked     bool
}

func NewContext(ln net.Listener, rw http.ResponseWriter, req *http.Request) *Context {
	var c Context
	c.ln = ln
	c.rw = rw
	c.values = make(map[string]interface{})
	c.venderString = req.Header.Get(VenderHeader)
	c.venderValues = make(map[VenderKey]string)

	if c.venderString != "" {
		for _, part := range strings.Split(strings.TrimSpace(c.venderString), ";") {
			part = strings.TrimSpace(part)
			if i := strings.Index(part, "="); i > 0 {
				name, val := part[:i], part[i+1:]
				c.venderValues[VenderKey(name)] = val
			}
		}
	}
	return &c
}

func (c *Context) SetString(name string, value string) {
	c.set(name, value)
}

func (c *Context) SetBool(name string, value bool) {
	c.set(name, value)
}

func (c *Context) SetInt(name string, value int) {
	c.set(name, value)
}

func (c *Context) SetStringMap(name string, value map[string]string) {
	c.set(name, value)
}

func (c *Context) set(name string, value interface{}) {
	c.values[name] = value
}

func (c *Context) GetString(name string) (string, bool) {
	v, ok := c.values[name]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	return s, true
}

func (c *Context) GetBool(name string) (bool, bool) {
	v, ok := c.values[name]
	if !ok {
		return false, false
	}
	s, ok := v.(bool)
	if !ok {
		return false, false
	}
	return s, true
}

func (c *Context) GetInt(name string) (int, bool) {
	v, ok := c.values[name]
	if !ok {
		return 0, false
	}
	s, ok := v.(int)
	if !ok {
		return 0, false
	}
	return s, true
}

func (c *Context) GetStringMap(name string) (map[string]string, bool) {
	v, ok := c.values[name]
	if !ok {
		return nil, false
	}
	s, ok := v.(map[string]string)
	if !ok {
		return nil, false
	}
	return s, true
}

func (c *Context) GetListener() net.Listener {
	return c.ln
}

func (c *Context) GetResponseWriter() http.ResponseWriter {
	return c.rw
}

func (c *Context) GetVenderString() string {
	return c.venderString
}

func (c *Context) SetHijacked(hijacked bool) {
	c.hijacked = hijacked
}

func (c *Context) Hijacked() bool {
	return c.hijacked
}

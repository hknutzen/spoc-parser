// Package parser implements a parser for source files of Netspoc
// policy language.  Input may be provided in a variety of forms (see
// the various Parse* functions); the output is an abstract syntax
// tree (AST) representing the Netspoc source. The parser is invoked
// through one of the Parse* functions.
//
package parser

import (
	"github.com/hknutzen/spoc-parser/ast"
	"github.com/hknutzen/spoc-parser/scanner"
	"net"
	"strconv"
	"strings"
)

// The parser structure holds the parser's internal state.
type parser struct {
	scanner scanner.Scanner
	fname   string

	// Next token
	pos int    // token position
	tok string // token literal, one token look-ahead
}

func (p *parser) init(src []byte, fname string) {
	p.scanner.Init(src, fname)
	p.fname = fname

	p.next()
}

// ----------------------------------------------------------------------------
// Parsing support

// Advance to the next token.
func (p *parser) next() {
	p.pos, p.tok = p.scanner.Token()
}

func (p *parser) syntaxErr(format string, args ...interface{}) {
	p.scanner.SyntaxErr(format, args...)
}

func (p *parser) expect(tok string) int {
	pos := p.pos
	if p.tok != tok {
		p.syntaxErr("Expected '%s'", tok)
	}
	p.next() // make progress
	return pos
}

func (p *parser) check(tok string) bool {
	if p.tok != tok {
		return false
	}
	p.next() // make progress
	return true
}

func isSimpleName(n string) bool {
	return n != "" && strings.IndexAny(n, ".:/@") == -1
}

func isDomain(n string) bool {
	for _, part := range strings.Split(n, ".") {
		if !isSimpleName(part) {
			return false
		}
	}
	return n != ""
}

func (p *parser) verifyHostname(name string) {
	err := false
	if strings.HasPrefix(name, "id:") {
		id := name[3:]
		i := strings.Index(id, "@")
		// Leading "@" is ok.
		err = i > 0 && !isDomain(id[:i]) || !isDomain(id[i+1:])
	} else {
		err = !isSimpleName(name)
	}
	if err {
		p.syntaxErr("Hostname expected")
	}
}

func isNetworkName(n string) bool {
	i := strings.Index(n, "/")
	return (i == -1 || isSimpleName(n[:i])) && isSimpleName(n[i+1:])
}

func (p *parser) verifyNetworkName(n string) {
	if !isNetworkName(n) {
		p.syntaxErr("Name or bridged name expected")
	}
}

func (p *parser) verifySimpleName(n string) {
	if !isSimpleName(n) {
		p.syntaxErr("Name expected")
	}
}

func isRouterName(n string) bool {
	i := strings.Index(n, "@")
	return (i == -1 || isSimpleName(n[:i])) && isSimpleName(n[i+1:])
}

func (p *parser) user() *ast.User {
	start := p.pos
	p.next()
	a := new(ast.User)
	a.Start = start
	a.Next = p.pos
	return a
}

func (p *parser) objectRef(typ, name string) ast.Element {
	start := p.pos
	p.next()
	a := new(ast.ObjectRef)
	a.Start = start
	a.Typ = typ
	a.Name = name
	a.Next = p.pos
	return a
}

func (p *parser) hostRef(typ, name string) ast.Element {
	p.verifyHostname(name)
	return p.objectRef(typ, name)
}

func (p *parser) networkRef(typ, name string) ast.Element {
	p.verifyNetworkName(name)
	return p.objectRef(typ, name)
}

func (p *parser) simpleRef(typ, name string) ast.Element {
	p.verifySimpleName(name)
	return p.objectRef(typ, name)
}

func (p *parser) selector() string {
	result := p.tok
	if !(result == "auto" || result == "all") {
		p.syntaxErr("Expected [auto|all]")
	}
	p.next()
	p.expect("]")
	return result
}

func (p *parser) intfRef(typ, name string) ast.Element {
	i := strings.Index(name, ".")
	if i == -1 {
		p.syntaxErr("Interface name expected")
	}
	router := name[:i]
	net := name[i+1:]
	err := !isRouterName(router)
	start := p.pos
	p.next()
	var ext string
	if net == "[" {
		ext = p.selector()
	} else {
		i := strings.Index(net, ".")
		if i != -1 {
			ext = net[i+1:]
			err = err || !isSimpleName(ext)
			net = net[:i]
		}
		err = err || !isNetworkName(net)
	}
	if err {
		p.syntaxErr("Interface name expected")
	}
	a := new(ast.IntfRef)
	a.Start = start
	a.Typ = typ
	a.Router = router
	a.Network = net   // If Network is "",
	a.Extension = ext // then Extension contains selector.
	a.Next = p.pos
	return a
}

func (p *parser) simpleAuto(start int, typ string) ast.Element {
	list := p.union("]")
	a := new(ast.SimpleAuto)
	a.Start = start
	a.Typ = typ
	a.Elements = list
	a.Next = p.pos
	return a
}

func (p *parser) ipPrefix() *net.IPNet {
	if i := strings.Index(p.tok, "/"); i != -1 {
		if ip := net.ParseIP(p.tok[:i]); ip != nil {
			if len, err := strconv.Atoi(p.tok[i+1:]); err == nil {
				bits := 8
				if ip4 := ip.To4(); ip4 != nil {
					bits *= net.IPv4len
				} else {
					bits *= net.IPv6len
				}
				if mask := net.CIDRMask(len, bits); mask != nil {
					p.next()
					return &net.IPNet{IP: ip, Mask: mask}
				}
			}
			p.syntaxErr("Prefixlen expected")
		} else {
			p.syntaxErr("IP address expected")
		}
	}
	p.syntaxErr("Expected 'IP/prefixlen'")
	return nil
}

func (p *parser) aggAuto(start int, typ string) ast.Element {
	var n *net.IPNet
	if p.check("ip") {
		p.check("=")
		n = p.ipPrefix()
		p.expect("&")
	}
	list := p.union("]")
	a := new(ast.AggAuto)
	a.Start = start
	a.Typ = typ
	a.Net = n
	a.Elements = list
	a.Next = p.pos
	return a
}

func (p *parser) intfAuto(start int, typ string) ast.Element {
	m := false
	if p.check("managed") {
		m = true
		p.expect("&")
	}
	list := p.union("]")
	p.expect(".[")
	s := p.selector()
	a := new(ast.IntfAuto)
	a.Start = start
	a.Typ = typ
	a.Managed = m
	a.Selector = s
	a.Elements = list
	a.Next = p.pos
	return a
}

func (p *parser) typedName() (string, string) {
	tok := p.tok
	i := strings.Index(tok, ":")
	if i == -1 {
		p.syntaxErr("Typed name expected")
	}
	typ := tok[:i]
	name := tok[i+1:]
	return typ, name
}

var elementType = map[string]func(*parser, string, string) ast.Element{
	"host":      (*parser).hostRef,
	"network":   (*parser).networkRef,
	"interface": (*parser).intfRef,
	"any":       (*parser).simpleRef,
	"area":      (*parser).simpleRef,
	"group":     (*parser).simpleRef,
}

var autoGroupType map[string]func(*parser, int, string) ast.Element

func init() {
	autoGroupType = map[string]func(*parser, int, string) ast.Element{
		"host":      (*parser).simpleAuto,
		"network":   (*parser).simpleAuto,
		"interface": (*parser).intfAuto,
		"any":       (*parser).aggAuto,
	}
}

func (p *parser) extendedName() ast.Element {
	if p.check("user") {
		return p.user()
	}
	typ, name := p.typedName()
	if name == "[" {
		start := p.pos
		p.next()
		m, found := autoGroupType[typ]
		if !found {
			p.syntaxErr("Unexpected automatic group")
		}
		return m(p, start, typ)
	}
	m, found := elementType[typ]
	if !found {
		p.syntaxErr("Unknown element type")
	}
	return m(p, typ, name)
}

func (p *parser) complement() ast.Element {
	start := p.pos
	if p.check("!") {
		el := p.extendedName()
		a := new(ast.Complement)
		a.Start = start
		a.Element = el
		a.Next = p.pos
		return a
	} else {
		return p.extendedName()
	}
}

func (p *parser) intersection() ast.Element {
	var intersection []ast.Element
	start := p.pos
	intersection = append(intersection, p.complement())
	for p.check("&") {
		intersection = append(intersection, p.complement())
	}
	if len(intersection) > 1 {
		a := new(ast.Intersection)
		a.Start = start
		a.List = intersection
		a.Next = p.pos
		return a
	} else {
		return intersection[0]
	}
}

// Read comma separated list of objects stopped by stopToken.
// Return AST with list of read elements.
func (p *parser) union(stopToken string) []ast.Element {
	var union []ast.Element
	union = append(union, p.intersection())

	for !p.check(stopToken) {
		p.expect(",")

		// Allow trailing comma.
		if p.check(stopToken) {
			break
		}
		union = append(union, p.intersection())
	}
	return union
}

func (p *parser) description() *ast.Description {
	start := p.pos
	if p.check("description") {
		if p.tok != "=" {
			p.syntaxErr("Expected '='")
		}
		p.pos, p.tok = p.scanner.ToEOL()
		text := p.tok
		p.next()
		a := new(ast.Description)
		a.Start = start
		a.Text = text
		a.Next = p.pos
		return a
	}
	return nil
}

func (p *parser) group() ast.Toplevel {
	start := p.pos
	name := p.tok
	p.next()
	p.expect("=")
	description := p.description()
	var list []ast.Element
	if !p.check(";") {
		list = p.union(";")
	}
	a := new(ast.Group)
	a.Start = start
	a.Name = name
	a.Description = description
	a.Elements = list
	a.Next = p.pos
	return a
}

var globalType = map[string]func(*parser) ast.Toplevel{
	//	"router":  parser.router,
	//	"network": parser.network,
	//	"any":     parser.aggregate,
	//	"area":    parser.area,
	"group": (*parser).group,
}

func (p *parser) toplevel() ast.Toplevel {
	typ, name := p.typedName()

	// Check for xxx:xxx | router:xx@xx | network:xx/xx
	if !(typ == "router" && isRouterName(name) ||
		typ == "network" && isNetworkName(name) || isSimpleName(name)) {
		p.syntaxErr("Invalid token")
	}
	m, found := globalType[typ]
	if !found {
		p.syntaxErr("Unknown global definition")
	}
	ast := m(p)
	ast.SetFname(p.fname)
	return ast
}

// ----------------------------------------------------------------------------
// Source files

func (p *parser) file() []ast.Toplevel {
	var list []ast.Toplevel
	for p.tok != "" {
		list = append(list, p.toplevel())
	}

	return list
}

func ParseFile(src []byte, fname string) []ast.Toplevel {
	p := new(parser)
	p.init(src, fname)
	return p.file()
}

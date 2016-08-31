package slim

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"unicode"

	"github.com/mattn/go-slim/vm"
)

type state int

const (
	sNeutral state = iota
	sTag
	sId
	sClass
	sAttrKey
	sAttrValue
	sEq
	sText
	sExpr
)

var emptyElement = []string{
	"doctype",
	"area",
	"base",
	"basefont",
	"br",
	"col",
	"frame",
	"hr",
	"img",
	"input",
	"isindex",
	"link",
	"meta",
	"param",
	"embed",
	"keygen",
	"command",
}

type Value interface{}

type Func func(...Value) (Value, error)

type Funcs map[string]Func
type Values map[string]Value

type Attr struct {
	Name  string
	Value string
}

type Node struct {
	Name     string
	Id       string
	Class    []string
	Attr     []Attr
	Text     string
	Expr     string
	Children []*Node
}

func (n *Node) NewChild() *Node {
	n.Children = append(n.Children, new(Node))
	return n.Children[len(n.Children)-1]
}

type stack struct {
	n    int
	node *Node
}

func isEmptyElement(n string) bool {
	for _, e := range emptyElement {
		if e == n {
			return true
		}
	}
	return false
}

func printNode(out io.Writer, v *vm.VM, n *Node, indent int) error {
	if n.Name == "" {
		for _, c := range n.Children {
			if err := printNode(out, v, c, indent); err != nil {
				return err
			}
		}
	} else {
		// FIXME
		doctype := n.Name == "doctype"
		if doctype {
			out.Write([]byte(strings.Repeat(" ", indent*2) + "<!" + n.Name + " html"))
			n.Attr = nil
		} else {
			if n.Name[len(n.Name)-1] == ':' {
				n.Name = n.Name[:len(n.Name)-1]
				if n.Name == "javascript" {
					n.Name = "script"
				}
			}
			out.Write([]byte(strings.Repeat(" ", indent*2) + "<" + n.Name))
		}
		if n.Id != "" {
			out.Write([]byte(" id=\"" + n.Id + "\""))
		}
		if len(n.Class) > 0 {
			out.Write([]byte(" class=\""))
			for i, c := range n.Class {
				if i > 0 {
					out.Write([]byte(" "))
				}
				out.Write([]byte(c))
			}
			out.Write([]byte("\""))
		}
		if len(n.Attr) > 0 {
			for _, a := range n.Attr {
				if a.Value == "" {
					out.Write([]byte(" " + a.Name))
				} else {
					fmt.Fprintf(out, " %s=%q", a.Name, a.Value)
				}
			}
		}
		if !isEmptyElement(n.Name) {
			out.Write([]byte(">"))
			cr := true
			if n.Expr != "" {
				expr, err := v.Compile(n.Expr)
				if err != nil {
					return err
				}
				fe, ok := expr.(*vm.ForExpr)
				if ok {
					rhs, ok := v.Get(fe.Rhs)
					if !ok {
						return errors.New("invalid token: " + fe.Rhs)
					}
					ra := reflect.ValueOf(rhs)
					switch ra.Type().Kind() {
					case reflect.Array, reflect.Slice:
					default:
						return errors.New("can't iterate: " + fe.Rhs)
					}
					out.Write([]byte("\n"))
					l := ra.Len()
					for i := 0; i < l; i++ {
						x := ra.Index(i).Interface()
						v.Set(fe.Lhs1, x)
						for _, c := range n.Children {
							if err := printNode(out, v, c, indent+1); err != nil {
								return err
							}
						}
					}
				} else {
					r, err := v.Eval(expr)
					if err != nil {
						return err
					}
					out.Write([]byte(fmt.Sprint(r)))
					cr = false
				}
				out.Write([]byte(n.Text))
			} else if len(n.Children) > 0 {
				out.Write([]byte("\n"))
				for _, c := range n.Children {
					if err := printNode(out, v, c, indent+1); err != nil {
						return err
					}
				}
				out.Write([]byte(n.Text))
			} else if n.Text != "" {
				out.Write([]byte(n.Text))
				cr = false
			} else if cr {
				out.Write([]byte("\n"))
			}
			if cr {
				out.Write([]byte(strings.Repeat(" ", indent*2)))
			}
			out.Write([]byte("</" + n.Name + ">\n"))
		} else if doctype {
			out.Write([]byte(">\n"))
		} else {
			out.Write([]byte("/>\n"))
		}
	}
	return nil
}

type Template struct {
	root *Node
	fm   Funcs
}

func ParseFile(name string) (*Template, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return Parse(f)
}

func Parse(in io.Reader) (*Template, error) {
	scanner := bufio.NewScanner(in)
	root := new(Node)
	node := root
	stk := []stack{}
	last := -1
	for scanner.Scan() {
		l := scanner.Text()
		rs := []rune(l)
		st := sNeutral
		tag := ""
		id := ""
		class := ""
		aname := ""
		avalue := ""
		for n := 0; n < len(rs); n++ {
			eol := n == len(rs)-1
			r := rs[n]
		break_st:
			switch st {
			case sNeutral:
				if unicode.IsSpace(r) {
					break
				}
				switch r {
				case '=':
					st = sExpr
					break break_st
				case '|':
					st = sText
					break break_st
				case '-':
					st = sExpr
					break break_st
				case '#':
					node.Name = "div"
					st = sId
					last = n
					break break_st
				case '.':
					node.Name = "div"
					st = sClass
					last = n
					break break_st
				}
				if r > 255 {
					node.Text = string(r)
					st = sText
					break break_st
				}

				st = sTag
				tag += string(r)

				if n > last {
					last = n
					if node.Name != "" && node.Name[len(node.Name)-1] == ':' {
						node.Text = tag
						st = sText
						break break_st
					}
					node = node.NewChild()
					stk = append(stk, stack{n: n, node: node})
				} else if n == last {
					last = n
					cur := root
					for cur != nil {
						var tmp *Node
						if len(cur.Children) == 0 {
							break
						}
						tmp = cur.Children[len(cur.Children)-1]
						if tmp == nil || tmp == node {
							break
						}
						cur = tmp
					}
					if cur.Name != "" && cur.Name[len(cur.Name)-1] == ':' {
						node.Text = tag
						st = sText
						break break_st
					}
					node = cur.NewChild()
					stk[len(stk)-1].node = node
				} else if n < last {
					last = n
					node = nil
					for i := 0; i < len(stk); i++ {
						if stk[i].n >= n {
							node = stk[i-1].node
							stk = stk[:i]
							break
						}
					}
					if node == nil {
						node = root.NewChild()
						stk = stk[:1]
					} else {
						if node.Name != "" && node.Name[len(node.Name)-1] == ':' {
							node.Text = tag
							st = sText
							break break_st
						}
						node = node.NewChild()
					}
				}
				node.Name = tag
			case sTag:
				if eol {
					tag += string(r)
					node.Name = tag
					break
				}
				switch r {
				case '=':
					if tag == "" {
						node.Name = "div"
					} else {
						node.Name = tag
					}
					st = sExpr
				case '#':
					if tag == "" {
						node.Name = "div"
					} else {
						node.Name = tag
					}
					st = sId
				case '.':
					if tag == "" {
						node.Name = "div"
					} else {
						node.Name = tag
					}
					st = sClass
				default:
					if tag == "" && unicode.IsLetter(r) {
						node.Text = string(r)
						st = sText
						break break_st
					}
					if unicode.IsSpace(r) {
						node.Name = tag
						st = sAttrKey
					} else {
						tag += string(r)
					}
				}
			case sId:
				if eol {
					if unicode.IsLetter(r) {
						id += string(r)
						node.Id = id
					}
					break
				}
				switch r {
				case '.':
					node.Id = id
					st = sClass
				default:
					if !unicode.IsLetter(r) {
						node.Id = id
						st = sEq
					} else {
						id += string(r)
					}
				}
			case sClass:
				if eol {
					if unicode.IsLetter(r) {
						class += string(r)
						node.Class = append(node.Class, class)
					}
					break
				}
				switch r {
				case '.':
					if class != "" {
						node.Class = append(node.Class, class)
						class = ""
					}
				default:
					if !unicode.IsLetter(r) {
						if class != "" {
							node.Class = append(node.Class, class)
						}
						st = sEq
					} else {
						class += string(r)
					}
				}
			case sAttrKey:
				if eol {
					aname += string(r)
					if avalue != "" {
						node.Attr = append(node.Attr, Attr{Name: strings.TrimSpace(aname), Value: ""})
					} else {
						node.Text = strings.TrimSpace(aname)
					}
					break
				}
				switch r {
				case '=':
					if aname == "" {
						st = sExpr
					} else {
						st = sAttrValue
					}
				default:
					aname += string(r)
				}
			case sAttrValue:
				if eol {
					if unicode.IsLetter(r) || r == '"' {
						avalue += string(r)
						if avalue[0] == '"' && avalue[len(avalue)-1] == '"' {
							avalue = avalue[1 : len(avalue)-1]
						}
						node.Attr = append(node.Attr, Attr{Name: aname, Value: strings.TrimSpace(avalue)})
					}
					break
				}
				if avalue != "" && unicode.IsSpace(r) {
					if avalue[0] == '"' {
						if avalue[len(avalue)-1] == '"' {
							avalue = avalue[1 : len(avalue)-1]
						} else {
							avalue += string(r)
							break
						}
					}
					node.Attr = append(node.Attr, Attr{Name: aname, Value: strings.TrimSpace(avalue)})
					aname = ""
					avalue = ""
					st = sAttrKey
				} else {
					avalue += string(r)
				}
			case sEq:
				if r != '=' && !unicode.IsSpace(r) {
					node.Expr += string(r)
					st = sExpr
				}
			case sExpr:
				node.Expr += string(r)
			case sText:
				node.Text += string(r)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return &Template{root, nil}, nil
}

func (t *Template) FuncMap(m Funcs) {
	t.fm = m
}

func (t *Template) Execute(out io.Writer, value interface{}) error {
	v := vm.New()

	if t.fm != nil {
		for key, val := range t.fm {
			v.Set(key, val)
		}
	}
	if value != nil {
		rv := reflect.ValueOf(value)
		rt := rv.Type()
		if rt.Kind() == reflect.Map {
			for _, rk := range rv.MapKeys() {
				v.Set(rk.String(), rv.MapIndex(rk).Interface())
			}
		} else if rt.Kind() == reflect.Struct {
			for i := 0; i < rt.NumField(); i++ {
				v.Set(rt.Field(i).Name, rv.Field(i).Interface())
			}
		}
	}
	return printNode(out, v, t.root, 0)
}

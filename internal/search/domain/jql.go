// Package domain is the Search context: JQL (Jira Query Language) — lexing, parsing into a query and
// evaluating it over issue facts — and saved filters. Pure, no I/O.
package domain

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

// ErrInvalidJQL wraps every ParseError.
var ErrInvalidJQL = errors.New("invalid JQL")

// ParseError points at the offending position (0-based, in bytes) of the query.
type ParseError struct {
	Pos int
	Msg string
}

func (e *ParseError) Error() string { return fmt.Sprintf("JQL at %d: %s", e.Pos, e.Msg) }
func (e *ParseError) Unwrap() error { return ErrInvalidJQL }

func fail(pos int, format string, args ...any) error {
	return &ParseError{Pos: pos, Msg: fmt.Sprintf(format, args...)}
}

// MaxLength bounds a query (a guard against pathological input).
const MaxLength = 1000

// Issue is what a query sees of an issue (filled by the application from Issue Tracking).
type Issue struct {
	ID, Key, Project     string
	Number               int
	Summary, Description string
	Type                 string
	Status, StatusName   string
	Category             string // todo, in_progress, done
	Priority             string
	Assignee, Reporter   string // user ids
	Sprints, FixVersions []string
	Resolution           string
	Parent               string // parent key
	Points               *float64
	StartDate, DueDate   string            // YYYY-MM-DD, "" = not set
	Custom               map[string]string // custom field values by lower-case field name
}

// CustomType is a custom field's type as queries see it: text, number, date or select.
type CustomType string

const (
	CustomText   CustomType = "text"
	CustomNumber CustomType = "number"
	CustomDate   CustomType = "date"
	CustomSelect CustomType = "select"
)

// Resolver turns names in a query into ids (users by email, sprints and versions by name).
type Resolver interface {
	User(name string) (id string, ok bool)
	CurrentUser() string
	Sprints(name string) []string
	OpenSprints() []string
	Versions(name string) []string
}

// ---- lexer

type tokKind int

const (
	tEOF tokKind = iota
	tWord
	tString
	tOp
	tLParen
	tRParen
	tComma
)

type token struct {
	kind tokKind
	text string
	pos  int
}

func (t token) is(word string) bool { return t.kind == tWord && strings.EqualFold(t.text, word) }

func lex(s string) ([]token, error) {
	var out []token
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c == '(':
			out = append(out, token{tLParen, "(", i})
			i++
		case c == ')':
			out = append(out, token{tRParen, ")", i})
			i++
		case c == ',':
			out = append(out, token{tComma, ",", i})
			i++
		case c == '"' || c == '\'':
			end := strings.IndexByte(s[i+1:], c)
			if end < 0 {
				return nil, fail(i, "unterminated string")
			}
			out = append(out, token{tString, s[i+1 : i+1+end], i})
			i += end + 2
		case strings.ContainsRune("=!~<>", rune(c)):
			op := string(c)
			if i+1 < len(s) && s[i+1] == '=' || c == '!' && i+1 < len(s) && s[i+1] == '~' {
				op = s[i : i+2]
			}
			if op == "!" {
				return nil, fail(i, "unexpected %q", op)
			}
			out = append(out, token{tOp, op, i})
			i += len(op)
		default:
			j := i
			for j < len(s) && !strings.ContainsRune(" \t\n\r(),=!~<>\"'", rune(s[j])) {
				j++
			}
			out = append(out, token{tWord, s[i:j], i})
			i = j
		}
	}
	return append(out, token{tEOF, "", len(s)}), nil
}

// ---- fields

type fieldKind int

const (
	kEnum   fieldKind = iota // =, !=, IN, NOT IN, IS
	kText                    // ~, !~, IS
	kNumber                  // =, !=, <, <=, >, >=, IS
	kDate                    // =, !=, <, <=, >, >=, IS (YYYY-MM-DD)
)

type fieldDef struct {
	name string
	kind fieldKind
}

var fields = map[string]fieldDef{
	"project": {"project", kEnum}, "key": {"key", kEnum}, "issuekey": {"key", kEnum},
	"type": {"type", kEnum}, "issuetype": {"type", kEnum}, "status": {"status", kEnum},
	"statuscategory": {"statuscategory", kEnum}, "priority": {"priority", kEnum},
	"assignee": {"assignee", kEnum}, "reporter": {"reporter", kEnum}, "sprint": {"sprint", kEnum},
	"fixversion": {"fixversion", kEnum}, "resolution": {"resolution", kEnum}, "parent": {"parent", kEnum},
	"summary": {"summary", kText}, "description": {"description", kText}, "text": {"text", kText},
	"startdate": {"startdate", kDate}, "start date": {"startdate", kDate},
	"duedate": {"duedate", kDate}, "due date": {"duedate", kDate}, "due": {"duedate", kDate},
	"story points": {"points", kNumber}, "storypoints": {"points", kNumber}, "points": {"points", kNumber},
}

var sortable = map[string]bool{"key": true, "project": true, "priority": true, "assignee": true, "reporter": true,
	"status": true, "type": true, "summary": true, "points": true}

// ---- AST

type value struct {
	text  string
	fn    string // currentUser, openSprints
	empty bool
	pos   int
}

type node interface {
	match(is *Issue, r Resolver) bool
}

type and struct{ l, r node }
type or struct{ l, r node }
type not struct{ n node }
type clause struct {
	kind   fieldKind
	field  string
	op     string // =, !=, in, not in, ~, !~, <, <=, >, >=, is, is not
	values []value
}

func (n and) match(is *Issue, r Resolver) bool { return n.l.match(is, r) && n.r.match(is, r) }
func (n or) match(is *Issue, r Resolver) bool  { return n.l.match(is, r) || n.r.match(is, r) }
func (n not) match(is *Issue, r Resolver) bool { return !n.n.match(is, r) }

type sortKey struct {
	field string
	desc  bool
}

// Query is a parsed JQL query.
type Query struct {
	where node // nil = everything
	order []sortKey
}

// ---- parser

type parser struct {
	toks   []token
	i      int
	custom map[string]CustomType
}

// field finds a built-in field or one of the custom fields (by name, case-insensitive).
func (p *parser) field(name string) (fieldDef, bool) {
	name = strings.ToLower(name)
	if f, ok := fields[name]; ok {
		return f, true
	}
	t, ok := p.custom[name]
	kind := map[CustomType]fieldKind{CustomText: kText, CustomNumber: kNumber, CustomDate: kDate, CustomSelect: kEnum}[t]
	return fieldDef{name: "cf:" + name, kind: kind}, ok
}

func (p *parser) peek() token { return p.toks[p.i] }
func (p *parser) next() token { t := p.toks[p.i]; p.i++; return t }

// Parse reads a JQL query: clauses joined by AND/OR/NOT with parentheses, then ORDER BY.
func Parse(jql string) (*Query, error) { return ParseWith(jql, nil) }

// ParseWith also knows custom fields (lower-case name → type) as clause fields.
func ParseWith(jql string, custom map[string]CustomType) (*Query, error) {
	if len(jql) > MaxLength {
		return nil, fail(MaxLength, "a query is at most %d characters", MaxLength)
	}
	toks, err := lex(jql)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks, custom: custom}
	q := &Query{}
	if t := p.peek(); t.kind != tEOF && !t.is("order") {
		if q.where, err = p.or(); err != nil {
			return nil, err
		}
	}
	if p.peek().is("order") {
		p.next()
		if t := p.next(); !t.is("by") {
			return nil, fail(t.pos, "expected BY after ORDER")
		}
		for {
			t := p.next()
			f, ok := fields[strings.ToLower(t.text)]
			if (t.kind != tWord && t.kind != tString) || !ok || !sortable[f.name] {
				return nil, fail(t.pos, "cannot order by %q", t.text)
			}
			k := sortKey{field: f.name}
			if p.peek().is("desc") || p.peek().is("asc") {
				k.desc = p.next().is("desc")
			}
			q.order = append(q.order, k)
			if p.peek().kind != tComma {
				break
			}
			p.next()
		}
	}
	if t := p.peek(); t.kind != tEOF {
		return nil, fail(t.pos, "unexpected %q", t.text)
	}
	return q, nil
}

func (p *parser) or() (node, error) {
	l, err := p.and()
	for err == nil && p.peek().is("or") {
		p.next()
		var r node
		if r, err = p.and(); err == nil {
			l = or{l, r}
		}
	}
	return l, err
}

func (p *parser) and() (node, error) {
	l, err := p.not()
	for err == nil && p.peek().is("and") {
		p.next()
		var r node
		if r, err = p.not(); err == nil {
			l = and{l, r}
		}
	}
	return l, err
}

func (p *parser) not() (node, error) {
	if p.peek().is("not") {
		p.next()
		n, err := p.not()
		return not{n}, err
	}
	if p.peek().kind == tLParen {
		p.next()
		n, err := p.or()
		if err != nil {
			return nil, err
		}
		if t := p.next(); t.kind != tRParen {
			return nil, fail(t.pos, "expected )")
		}
		return n, nil
	}
	return p.clause()
}

func (p *parser) clause() (node, error) {
	ft := p.next()
	f, ok := p.field(ft.text)
	if (ft.kind != tWord && ft.kind != tString) || !ok {
		if ft.kind == tEOF {
			return nil, fail(ft.pos, "expected a field")
		}
		return nil, fail(ft.pos, "unknown field %q", ft.text)
	}
	c := clause{field: f.name, kind: f.kind}
	opTok := p.next()
	switch {
	case opTok.kind == tOp:
		c.op = opTok.text
	case opTok.is("in"):
		c.op = "in"
	case opTok.is("not") && p.peek().is("in"):
		p.next()
		c.op = "not in"
	case opTok.is("is"):
		c.op = "is"
		if p.peek().is("not") {
			p.next()
			c.op = "is not"
		}
		v := p.next()
		if !v.is("empty") && !v.is("null") {
			return nil, fail(v.pos, "expected EMPTY")
		}
		return c, nil
	default:
		return nil, fail(opTok.pos, "expected an operator")
	}
	if !allowed(f.kind, c.op) {
		return nil, fail(opTok.pos, "%s does not support %s", ft.text, c.op)
	}
	if c.op == "in" || c.op == "not in" {
		vs, err := p.list(f.name)
		c.values = vs
		return c, err
	}
	v, err := p.value(f.name, f.kind)
	c.values = []value{v}
	return c, err
}

func allowed(k fieldKind, op string) bool {
	switch k {
	case kText:
		return op == "~" || op == "!~"
	case kNumber, kDate:
		return slices.Contains([]string{"=", "!=", "<", "<=", ">", ">="}, op)
	}
	return slices.Contains([]string{"=", "!=", "in", "not in"}, op)
}

func (p *parser) list(field string) ([]value, error) {
	if t := p.peek(); t.kind == tWord && p.toks[p.i+1].kind == tLParen {
		v, err := p.value(field, kEnum) // IN openSprints()
		return []value{v}, err
	}
	if t := p.next(); t.kind != tLParen {
		return nil, fail(t.pos, "expected (")
	}
	var out []value
	for {
		v, err := p.value(field, kEnum)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
		switch t := p.next(); t.kind {
		case tComma:
		case tRParen:
			return out, nil
		default:
			return nil, fail(t.pos, "expected , or )")
		}
	}
}

var functions = map[string]map[string]bool{"currentuser": {"assignee": true, "reporter": true}, "opensprints": {"sprint": true}}

func (p *parser) value(field string, kind fieldKind) (value, error) {
	t := p.next()
	if t.kind != tWord && t.kind != tString {
		return value{}, fail(t.pos, "expected a value")
	}
	v := value{text: t.text, pos: t.pos}
	if t.kind == tWord && p.peek().kind == tLParen {
		p.next()
		if r := p.next(); r.kind != tRParen {
			return value{}, fail(r.pos, "expected )")
		}
		name := strings.ToLower(t.text)
		if !functions[name][field] {
			return value{}, fail(t.pos, "unknown function %s() for %s", t.text, field)
		}
		v.fn = name
		return v, nil
	}
	if t.kind == tWord && (t.is("empty") || t.is("null")) {
		v.empty = true
		return v, nil
	}
	if kind == kNumber {
		if _, err := strconv.ParseFloat(t.text, 64); err != nil {
			return value{}, fail(t.pos, "%q is not a number", t.text)
		}
	}
	if kind == kDate {
		if _, err := time.Parse(time.DateOnly, t.text); err != nil {
			return value{}, fail(t.pos, "%q is not a date (YYYY-MM-DD)", t.text)
		}
	}
	if field == "key" || field == "parent" {
		if i := strings.LastIndexByte(t.text, '-'); i <= 0 || i == len(t.text)-1 {
			return value{}, fail(t.pos, "%q is not an issue key", t.text)
		}
	}
	return v, nil
}

// ---- evaluation

// values returns the issue's values of an enum field (empty when unset).
func values(is *Issue, field string) []string {
	one := func(s string) []string {
		if s == "" {
			return nil
		}
		return []string{s}
	}
	switch field {
	case "project":
		return one(is.Project)
	case "key":
		return one(is.Key)
	case "type":
		return one(is.Type)
	case "status":
		return []string{is.Status, is.StatusName}
	case "statuscategory":
		return one(is.Category)
	case "priority":
		return one(is.Priority)
	case "assignee":
		return one(is.Assignee)
	case "reporter":
		return one(is.Reporter)
	case "sprint":
		return is.Sprints
	case "fixversion":
		return is.FixVersions
	case "resolution":
		return one(is.Resolution)
	case "parent":
		return one(is.Parent)
	case "startdate":
		return one(is.StartDate)
	case "duedate":
		return one(is.DueDate)
	}
	return one(is.Custom[strings.TrimPrefix(field, "cf:")])
}

func norm(s string) string {
	return strings.NewReplacer(" ", "_", "-", "", "'", "").Replace(strings.ToLower(strings.TrimSpace(s)))
}

var categories = map[string]string{"todo": "todo", "to_do": "todo", "new": "todo", "in_progress": "in_progress",
	"inprogress": "in_progress", "indeterminate": "in_progress", "done": "done", "complete": "done"}

// wanted resolves a value to the strings it matches (ids, names) for the field.
func wanted(field string, v value, r Resolver) []string {
	switch {
	case v.fn == "currentuser":
		return []string{r.CurrentUser()}
	case v.fn == "opensprints":
		return r.OpenSprints()
	}
	switch field {
	case "assignee", "reporter":
		if id, ok := r.User(v.text); ok {
			return []string{id}
		}
	case "sprint":
		return append(r.Sprints(v.text), v.text)
	case "fixversion":
		return append(r.Versions(v.text), v.text)
	case "statuscategory":
		return []string{categories[norm(v.text)]}
	case "resolution":
		if norm(v.text) == "unresolved" {
			return []string{""}
		}
	}
	return []string{v.text}
}

func (c clause) match(is *Issue, r Resolver) bool {
	switch c.kind {
	case kText:
		return c.text(is)
	case kNumber:
		return c.number(is)
	case kDate:
		return c.date(is)
	}
	have := values(is, c.field)
	if c.op == "is" || c.op == "is not" {
		return (len(have) == 0) == (c.op == "is")
	}
	hit := false
	for _, v := range c.values {
		if v.empty {
			hit = hit || len(have) == 0
			continue
		}
		for _, w := range wanted(c.field, v, r) {
			if w == "" && len(have) == 0 {
				hit = true
			}
			for _, h := range have {
				if w != "" && norm(h) == norm(w) {
					hit = true
				}
			}
		}
	}
	if c.op == "=" || c.op == "in" {
		return hit
	}
	return !hit && len(have) > 0 // like Jira, != and NOT IN never match empty fields
}

func (c clause) text(is *Issue) bool {
	var hay string
	switch c.field {
	case "summary":
		hay = is.Summary
	case "description":
		hay = is.Description
	case "text":
		hay = is.Summary + "\n" + is.Description
	default:
		hay = is.Custom[strings.TrimPrefix(c.field, "cf:")]
	}
	if c.op == "is" || c.op == "is not" {
		return (hay == "") == (c.op == "is")
	}
	in := strings.Contains(strings.ToLower(hay), strings.ToLower(c.values[0].text))
	return in == (c.op == "~")
}

func (c clause) number(is *Issue) bool {
	have, ok := 0.0, false
	if c.field == "points" {
		if is.Points != nil {
			have, ok = *is.Points, true
		}
	} else if v, err := strconv.ParseFloat(is.Custom[strings.TrimPrefix(c.field, "cf:")], 64); err == nil {
		have, ok = v, true
	}
	if c.op == "is" || c.op == "is not" || c.values[0].empty {
		return !ok == (c.op == "is" || c.op == "=")
	}
	if !ok {
		return false
	}
	want, _ := strconv.ParseFloat(c.values[0].text, 64)
	return compareOp(c.op, cmp.Compare(have, want))
}

// date compares YYYY-MM-DD values (they sort as text).
func (c clause) date(is *Issue) bool {
	have := strings.Join(values(is, c.field), "")
	if c.op == "is" || c.op == "is not" || c.values[0].empty {
		return (have == "") == (c.op == "is" || c.op == "=")
	}
	if have == "" {
		return false
	}
	return compareOp(c.op, cmp.Compare(have, c.values[0].text))
}

// compareOp applies a comparison operator to a three-way comparison result.
func compareOp(op string, c int) bool {
	switch op {
	case "=":
		return c == 0
	case "!=":
		return c != 0
	case "<":
		return c < 0
	case "<=":
		return c <= 0
	case ">":
		return c > 0
	}
	return c >= 0
}

var priorities = map[string]int{"lowest": 1, "low": 2, "medium": 3, "high": 4, "highest": 5}

// compare orders two issues by one field; empty values always come last.
func compare(a, b *Issue, field string) (c int, emptyA, emptyB bool) {
	str := func(x, y string) (int, bool, bool) {
		return cmp.Compare(strings.ToLower(x), strings.ToLower(y)), x == "", y == ""
	}
	switch field {
	case "key":
		return byKey(a, b), false, false
	case "project":
		return str(a.Project, b.Project)
	case "priority":
		return cmp.Compare(priorities[a.Priority], priorities[b.Priority]), a.Priority == "", b.Priority == ""
	case "assignee":
		return str(a.Assignee, b.Assignee)
	case "reporter":
		return str(a.Reporter, b.Reporter)
	case "status":
		return str(a.StatusName, b.StatusName)
	case "type":
		return str(a.Type, b.Type)
	case "summary":
		return str(a.Summary, b.Summary)
	}
	if a.Points == nil || b.Points == nil {
		return 0, a.Points == nil, b.Points == nil
	}
	return cmp.Compare(*a.Points, *b.Points), false, false
}

func byKey(a, b *Issue) int {
	if c := cmp.Compare(a.Project, b.Project); c != 0 {
		return c
	}
	return cmp.Compare(a.Number, b.Number)
}

// Run returns the issues matching the query, in its ORDER BY (then key) order.
func (q *Query) Run(issues []Issue, r Resolver) ([]Issue, error) {
	out := []Issue{}
	for i := range issues {
		if q.where == nil || q.where.match(&issues[i], r) {
			out = append(out, issues[i])
		}
	}
	slices.SortStableFunc(out, func(a, b Issue) int {
		for _, k := range q.order {
			c, ea, eb := compare(&a, &b, k.field)
			switch {
			case ea && eb:
				continue
			case ea:
				return 1
			case eb:
				return -1
			}
			if k.desc {
				c = -c
			}
			if c != 0 {
				return c
			}
		}
		return byKey(&a, &b)
	})
	return out, nil
}

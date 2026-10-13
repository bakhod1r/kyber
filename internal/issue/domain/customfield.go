package domain

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrInvalidCustomField = errors.New("invalid custom field")
	ErrInvalidFieldValue  = errors.New("invalid custom field value")
	ErrFieldNotFound      = errors.New("custom field not found")
	ErrFieldConflict      = errors.New("custom field was changed concurrently")
)

type (
	FieldID   string
	FieldType string
)

const (
	FieldText   FieldType = "text"
	FieldNumber FieldType = "number"
	FieldDate   FieldType = "date"
	FieldSelect FieldType = "select"
)

const (
	maxFieldName   = 60
	maxFieldText   = 1000
	maxFieldOption = 60
	maxOptions     = 100
)

// reserved are the built-in fields of JQL; custom fields may not shadow them.
var reserved = []string{"project", "key", "issuekey", "type", "issuetype", "status", "statuscategory", "priority",
	"assignee", "reporter", "sprint", "fixversion", "resolution", "parent", "summary", "description", "text",
	"story points", "storypoints", "points", "startdate", "start date", "duedate", "due date", "due"}

// CustomField is a project's own issue field (Jira's custom field): text, number, date or a
// choice from options. Issues hold its values; the definition says how values are checked.
type CustomField struct {
	id      FieldID
	project string
	name    string
	typ     FieldType
	options []string
	version int
}

type CustomFieldSnapshot struct {
	ID      FieldID
	Project string
	Name    string
	Type    FieldType
	Options []string
	Version int
}

func NewCustomField(id FieldID, project, name string, typ FieldType, options []string) (*CustomField, error) {
	if !slices.Contains([]FieldType{FieldText, FieldNumber, FieldDate, FieldSelect}, typ) {
		return nil, fmt.Errorf("%w: type %q", ErrInvalidCustomField, typ)
	}
	f := &CustomField{id: id, project: project, typ: typ}
	return f, f.Edit(name, options)
}

func RehydrateCustomField(s CustomFieldSnapshot) *CustomField {
	return &CustomField{id: s.ID, project: s.Project, name: s.Name, typ: s.Type, options: slices.Clone(s.Options), version: s.Version}
}

func (f *CustomField) Snapshot() CustomFieldSnapshot {
	return CustomFieldSnapshot{ID: f.id, Project: f.project, Name: f.name, Type: f.typ, Options: slices.Clone(f.options), Version: f.version}
}

func (f *CustomField) ID() FieldID       { return f.id }
func (f *CustomField) Project() string   { return f.project }
func (f *CustomField) Name() string      { return f.name }
func (f *CustomField) Type() FieldType   { return f.typ }
func (f *CustomField) Options() []string { return slices.Clone(f.options) }
func (f *CustomField) Version() int      { return f.version }
func (f *CustomField) MarkPersisted()    { f.version++ }

// Edit renames the field and replaces a select field's options (other types take none).
func (f *CustomField) Edit(name string, options []string) error {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > maxFieldName || slices.Contains(reserved, strings.ToLower(name)) {
		return fmt.Errorf("%w: name must be 1-%d characters and not a built-in field", ErrInvalidCustomField, maxFieldName)
	}
	var opts []string
	if f.typ == FieldSelect {
		if len(options) == 0 || len(options) > maxOptions {
			return fmt.Errorf("%w: a select field needs 1-%d options", ErrInvalidCustomField, maxOptions)
		}
		seen := map[string]bool{}
		for _, o := range options {
			o = strings.TrimSpace(o)
			if o == "" || utf8.RuneCountInString(o) > maxFieldOption || seen[strings.ToLower(o)] {
				return fmt.Errorf("%w: options must be distinct and 1-%d characters", ErrInvalidCustomField, maxFieldOption)
			}
			seen[strings.ToLower(o)] = true
			opts = append(opts, o)
		}
	} else if len(options) > 0 {
		return fmt.Errorf("%w: only select fields have options", ErrInvalidCustomField)
	}
	f.name, f.options = name, opts
	return nil
}

// Normalize checks a raw value against the field's type and returns its canonical form
// ("" stays "": no value).
func (f *CustomField) Normalize(raw string) (string, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "", nil
	}
	bad := func() (string, error) {
		return "", fmt.Errorf("%w: %q is not a valid %s for %s", ErrInvalidFieldValue, raw, f.typ, f.name)
	}
	switch f.typ {
	case FieldNumber:
		n, err := strconv.ParseFloat(v, 64)
		if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
			return bad()
		}
		return strconv.FormatFloat(n, 'f', -1, 64), nil
	case FieldDate:
		if _, err := time.Parse(time.DateOnly, v); err != nil {
			return bad()
		}
		return v, nil
	case FieldSelect:
		for _, o := range f.options {
			if strings.EqualFold(o, v) {
				return o, nil
			}
		}
		return bad()
	}
	if utf8.RuneCountInString(v) > maxFieldText {
		return bad()
	}
	return v, nil
}

// CustomFieldRepository stores field definitions in the workspace of ctx.
type CustomFieldRepository interface {
	SaveField(ctx context.Context, f *CustomField) error // insert (Version 0) or optimistic update
	FieldByID(ctx context.Context, id FieldID) (*CustomField, error)
	FieldsOf(ctx context.Context, project string) ([]*CustomField, error) // in creation order
	DeleteField(ctx context.Context, id FieldID) error
}

// ---- values on the issue

// Field returns the issue's value of a custom field ("" = none).
func (i *Issue) Field(id FieldID) string { return i.fields[id] }

// Fields returns every custom field value the issue holds.
func (i *Issue) Fields() map[FieldID]string { return maps.Clone(i.fields) }

// SetField sets (or with "" clears) a value of one of the project's custom fields.
func (i *Issue) SetField(f *CustomField, raw string) error {
	if f.project != i.key.Project() {
		return fmt.Errorf("%w: %s is not a field of %s", ErrInvalidFieldValue, f.name, i.key.Project())
	}
	v, err := f.Normalize(raw)
	if err != nil {
		return err
	}
	if i.fieldNames == nil {
		i.fieldNames = map[FieldID]string{}
	}
	i.fieldNames[f.id] = f.name
	fields := maps.Clone(i.fields) // copy on write: snapshots and stored copies never share the map
	if fields == nil {
		fields = map[FieldID]string{}
	}
	if v == "" {
		delete(fields, f.id)
	} else {
		fields[f.id] = v
	}
	i.fields = fields
	return nil
}

// fieldChanges lists custom-field changes between two snapshots for the changelog.
func (i *Issue) fieldChanges(b, a map[FieldID]string) []FieldChange {
	ids := slices.Sorted(maps.Keys(b))
	for id := range a {
		if _, ok := b[id]; !ok {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	var out []FieldChange
	for _, id := range ids {
		if b[id] != a[id] {
			out = append(out, FieldChange{Field: "custom:" + i.fieldNames[id], From: b[id], To: a[id]}) // only SetField changes values
		}
	}
	return out
}

package application

import (
	"context"
	"errors"

	"github.com/bakhod1r/kyber/internal/issue/domain"
)

// FieldSpec is a custom field definition as admins edit it.
type FieldSpec struct {
	Name    string
	Type    string
	Options []string
}

// CustomFields lists the project's custom fields (anyone who can browse the project).
func (s *Service) CustomFields(ctx context.Context, actor, project string) ([]*domain.CustomField, error) {
	if err := s.access.Can(ctx, actor, project, PermBrowse, nil); err != nil {
		return nil, err
	}
	return s.fieldsOf(ctx, project)
}

func (s *Service) fieldsOf(ctx context.Context, project string) ([]*domain.CustomField, error) {
	if s.fields == nil {
		return nil, nil
	}
	return s.fields.FieldsOf(ctx, project)
}

// CreateCustomField adds a field to the project (project admins).
func (s *Service) CreateCustomField(ctx context.Context, actor, project string, spec FieldSpec) (*domain.CustomField, error) {
	if err := s.access.Can(ctx, actor, project, PermAdminister, nil); err != nil {
		return nil, err
	}
	f, err := domain.NewCustomField(domain.FieldID(s.newID()), project, spec.Name, domain.FieldType(spec.Type), spec.Options)
	if err != nil {
		return nil, err
	}
	return f, s.fields.SaveField(ctx, f)
}

// adminField loads a field the actor administers; outsiders get ErrFieldNotFound.
func (s *Service) adminField(ctx context.Context, actor, id string) (*domain.CustomField, error) {
	if s.fields == nil {
		return nil, domain.ErrFieldNotFound
	}
	f, err := s.fields.FieldByID(ctx, domain.FieldID(id))
	if err != nil {
		return nil, err
	}
	err = s.access.Can(ctx, actor, f.Project(), PermAdminister, nil)
	if errors.Is(err, ErrProjectNotFound) {
		return nil, domain.ErrFieldNotFound
	}
	return f, err
}

// EditCustomField renames a field or changes a select field's options (at version).
func (s *Service) EditCustomField(ctx context.Context, actor, id string, version int, spec FieldSpec) (*domain.CustomField, error) {
	f, err := s.adminField(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	if f.Version() != version {
		return nil, domain.ErrFieldConflict
	}
	if err := f.Edit(spec.Name, spec.Options); err != nil {
		return nil, err
	}
	return f, s.fields.SaveField(ctx, f)
}

// DeleteCustomField removes a field; values issues held for it are no longer shown or searched.
func (s *Service) DeleteCustomField(ctx context.Context, actor, id string) error {
	f, err := s.adminField(ctx, actor, id)
	if err != nil {
		return err
	}
	return s.fields.DeleteField(ctx, f.ID())
}

// setFields applies custom field values (field id → raw value, "" clears) to an issue.
func (s *Service) setFields(ctx context.Context, is *domain.Issue, values map[string]string) error {
	if len(values) == 0 {
		return nil
	}
	defs, err := s.fieldsOf(ctx, is.Key().Project())
	if err != nil {
		return err
	}
	byID := map[string]*domain.CustomField{}
	for _, f := range defs {
		byID[string(f.ID())] = f
	}
	for id, raw := range values {
		f, ok := byID[id]
		if !ok {
			return errors.Join(domain.ErrInvalidFieldValue, errors.New("unknown field "+id))
		}
		if err := is.SetField(f, raw); err != nil {
			return err
		}
	}
	return nil
}

// FieldValues returns the issue's values of the project's current fields (deleted fields drop out).
func (s *Service) FieldValues(ctx context.Context, is *domain.Issue) (map[string]string, error) {
	defs, err := s.fieldsOf(ctx, is.Key().Project())
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, f := range defs {
		if v := is.Field(f.ID()); v != "" {
			out[string(f.ID())] = v
		}
	}
	return out, nil
}

// ProjectFields lists the project's custom fields without authorization, for read models in other
// contexts (search) that authorize the caller themselves.
func (s *Service) ProjectFields(ctx context.Context, project string) ([]*domain.CustomField, error) {
	return s.fieldsOf(ctx, project)
}

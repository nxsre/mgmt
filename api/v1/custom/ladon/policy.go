package ladon

import (
	"encoding/json"

	"github.com/ory/ladon"
	"github.com/pkg/errors"
)

type Policy struct {
	ID          string           `json:"id" gorethink:"id"`
	Description string           `json:"description" gorethink:"description"`
	Subjects    []string         `json:"subjects" gorethink:"subjects"`
	Effect      string           `json:"effect" gorethink:"effect"`
	Resources   []string         `json:"resources" gorethink:"resources"`
	Actions     []string         `json:"actions" gorethink:"actions"`
	Conditions  ladon.Conditions `json:"conditions" gorethink:"conditions"`
}

func (p *Policy) GetID() string {
	return p.ID
}

func (p *Policy) GetDescription() string {
	return p.Description
}
func (p *Policy) GetSubjects() []string {
	return p.Subjects
}
func (p *Policy) AllowAccess() bool {
	return p.Effect == ladon.AllowAccess
}
func (p *Policy) GetEffect() string {
	return p.Effect
}
func (p *Policy) GetResources() []string {
	return p.Resources
}
func (p *Policy) GetActions() []string {
	return p.Actions
}
func (p *Policy) GetConditions() ladon.Conditions {
	return p.Conditions
}
func (p *Policy) GetStartDelimiter() byte {
	return '<'
}
func (p *Policy) GetEndDelimiter() byte {
	return '>'
}

// UnmarshalJSON overwrite own policy with values of the given in policy in JSON format
func (p *Policy) UnmarshalJSON(data []byte) error {
	var pol = struct {
		ID          string           `json:"id" gorethink:"id"`
		Description string           `json:"description" gorethink:"description"`
		Subjects    []string         `json:"subjects" gorethink:"subjects"`
		Effect      string           `json:"effect" gorethink:"effect"`
		Resources   []string         `json:"resources" gorethink:"resources"`
		Actions     []string         `json:"actions" gorethink:"actions"`
		Conditions  ladon.Conditions `json:"conditions" gorethink:"conditions"`
	}{
		Conditions: ladon.Conditions{},
	}

	if err := json.Unmarshal(data, &pol); err != nil {
		return errors.WithStack(err)
	}

	*p = *&Policy{
		ID:          pol.ID,
		Description: pol.Description,
		Subjects:    pol.Subjects,
		Effect:      pol.Effect,
		Resources:   pol.Resources,
		Actions:     pol.Actions,
		Conditions:  pol.Conditions,
	}
	return nil
}

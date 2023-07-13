package resourcemodifiers

import (
	"strings"

	"fmt"
)

func (r *ResourceModifierRule) Validate() error {
	if err := r.Conditions.Validate(); err != nil {
		return err
	}
	for _, patch := range r.Patches {
		if err := patch.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (p *ResourceModifiers) Validate() error {
	if !strings.EqualFold(p.Version, ResourceModifierSupportedVersionV1) {
		return fmt.Errorf("unsupported resource modifier version %s", p.Version)
	}
	if len(p.ResourceModifierRules) == 0 {
		return fmt.Errorf("resource modifier rules cannot be empty")
	}
	for _, rule := range p.ResourceModifierRules {
		if err := rule.Validate(); err != nil {
			return err
		}
	}

	return nil
}

func (p *JSONPatch) Validate() error {
	if p.Operation == "" {
		return fmt.Errorf("operation cannot be empty")
	}
	if operation := strings.ToLower(p.Operation); operation != "add" && operation != "remove" && operation != "replace" && operation != "test" && operation != "move" && operation != "copy" {
		return fmt.Errorf("unsupported operation %s", p.Operation)
	}
	if p.Path == "" {
		return fmt.Errorf("path cannot be empty")
	}
	return nil
}

func (c *Conditions) Validate() error {
	if c.GroupKind == "" {
		return fmt.Errorf("groupkind cannot be empty")
	}
	return nil
}

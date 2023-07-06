package resourcemodifiers

import (
	"testing"
)

func TestResourceModifiers_Validate(t *testing.T) {
	type fields struct {
		Version               string
		ResourceModifierRules []ResourceModifierRule
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "correct version, non 0 length ResourceModifierRules",
			fields: fields{
				Version: "v1",
				ResourceModifierRules: []ResourceModifierRule{
					{
						Conditions: Conditions{
							GroupKind:         "persistentvolumeclaims.storage.k8s.io",
							ResourceNameRegex: ".*",
							Namespaces:        []string{"bar", "foo"},
						},
						Patches: []JSONPatch{
							{
								Operation: "replace",
								Path:      "/spec/storageClassName",
								Value:     "premium",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "incorrect version, non 0 length ResourceModifierRules",
			fields: fields{
				Version: "v2",
				ResourceModifierRules: []ResourceModifierRule{
					{
						Conditions: Conditions{
							GroupKind:         "persistentvolumeclaims.storage.k8s.io",
							ResourceNameRegex: ".*",
							Namespaces:        []string{"bar", "foo"},
						},
						Patches: []JSONPatch{
							{
								Operation: "replace",
								Path:      "/spec/storageClassName",
								Value:     "premium",
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "correct version, 0 length ResourceModifierRules",
			fields: fields{
				Version:               "v1",
				ResourceModifierRules: []ResourceModifierRule{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &ResourceModifiers{
				Version:               tt.fields.Version,
				ResourceModifierRules: tt.fields.ResourceModifierRules,
			}
			if err := p.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("ResourceModifiers.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestJsonPatch_Validate(t *testing.T) {
	type fields struct {
		Operation string
		Path      string
		Value     string
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "not empty operation, path, and new value, valid scenario",
			fields: fields{
				Operation: "replace",
				Path:      "/spec/storageClassName",
				Value:     "premium",
			},
			wantErr: false,
		},
		{
			name: "empty operation throws error",
			fields: fields{
				Operation: "",
				Path:      "/spec/storageClassName",
				Value:     "premium",
			},
			wantErr: true,
		},
		{
			name: "empty path throws error",
			fields: fields{
				Operation: "replace",
				Path:      "",
				Value:     "premium",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &JSONPatch{
				Operation: tt.fields.Operation,
				Path:      tt.fields.Path,
				Value:     tt.fields.Value,
			}
			if err := p.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("JsonPatch.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConditions_Validate(t *testing.T) {
	type fields struct {
		Namespaces        []string
		GroupKind         string
		ResourceNameRegex string
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "non 0 length namespaces, non empty group kind, non empty resource name regex",
			fields: fields{
				Namespaces:        []string{"bar", "foo"},
				GroupKind:         "persistentvolumeclaims.storage.k8s.io",
				ResourceNameRegex: ".*",
			},
			wantErr: false,
		},
		{
			name: "empty group kind throws error",
			fields: fields{
				Namespaces:        []string{"bar", "foo"},
				GroupKind:         "",
				ResourceNameRegex: ".*",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Conditions{
				Namespaces:        tt.fields.Namespaces,
				GroupKind:         tt.fields.GroupKind,
				ResourceNameRegex: tt.fields.ResourceNameRegex,
			}
			if err := c.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Conditions.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

package logging

import (
	"reflect"
	"sync"
	"testing"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/vmware-tanzu/velero/pkg/util/results"
)

type fields struct {
	mu      sync.RWMutex
	counts  map[logrus.Level]int
	entries map[logrus.Level]*results.Result
}

func TestLogHook_Fire(t *testing.T) {
	type args struct {
		entry *logrus.Entry
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "test",
			fields: fields{
				mu: sync.RWMutex{},
				counts: map[logrus.Level]int{
					logrus.ErrorLevel: 1,
				},
				entries: map[logrus.Level]*results.Result{
					logrus.ErrorLevel: {
						Velero: []string{"test error"},
					},
				},
			},
			args: args{
				entry: &logrus.Entry{
					Level: logrus.ErrorLevel,
					Data: map[string]interface{}{
						"namespace": "test",
						"error":     errors.New("test error"),
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &LogHook{
				mu:      tt.fields.mu,
				counts:  tt.fields.counts,
				entries: tt.fields.entries,
			}
			if err := h.Fire(tt.args.entry); (err != nil) != tt.wantErr {
				t.Errorf("LogHook.Fire() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLogHook_GetCount(t *testing.T) {
	type args struct {
		level logrus.Level
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   int
	}{
		{
			name: "test",
			fields: fields{
				mu: sync.RWMutex{},
				counts: map[logrus.Level]int{
					logrus.ErrorLevel: 1,
				},
				entries: map[logrus.Level]*results.Result{
					logrus.ErrorLevel: {
						Velero: []string{"test error"},
					},
				},
			},
			args: args{
				level: logrus.ErrorLevel,
			},
			want: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &LogHook{
				mu:      tt.fields.mu,
				counts:  tt.fields.counts,
				entries: tt.fields.entries,
			}
			if got := h.GetCount(tt.args.level); got != tt.want {
				t.Errorf("LogHook.GetCount() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLogHook_GetEntries(t *testing.T) {
	type args struct {
		level logrus.Level
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   results.Result
	}{
		{
			name: "test",
			fields: fields{
				mu: sync.RWMutex{},
				counts: map[logrus.Level]int{
					logrus.ErrorLevel: 1,
				},
				entries: map[logrus.Level]*results.Result{
					logrus.ErrorLevel: {
						Velero: []string{"test error"},
					},
				},
			},
			args: args{
				level: logrus.ErrorLevel,
			},
			want: results.Result{
				Velero: []string{"test error"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &LogHook{
				mu:      tt.fields.mu,
				counts:  tt.fields.counts,
				entries: tt.fields.entries,
			}
			if got := h.GetEntries(tt.args.level); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LogHook.GetEntries() = %v, want %v", got, tt.want)
			}
		})
	}
}

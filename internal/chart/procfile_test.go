package chart

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
)

func TestParseProcfile(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    *Procfile
		wantErr bool
	}{
		{
			name:    "single web command",
			content: "{\"processes\":[{\"type\":\"web\"}]}",
			want: &Procfile{
				Processes: map[string][]string{
					"web": {"web"},
				},
				RoutableProcessName: "web",
			},
		},
		{
			name:    "single command",
			content: "{\"processes\":[{\"type\":\"long-command-name\"}]}",
			want: &Procfile{
				Processes: map[string][]string{
					"long-command-name": {"long-command-name"},
				},
				RoutableProcessName: "long-command-name",
			},
		},
		{
			name:    "two commands",
			content: "{\"processes\":[{\"type\":\"web\"},{\"type\":\"worker\"}]}",
			want: &Procfile{
				Processes: map[string][]string{
					"web":    {"web"},
					"worker": {"worker"},
				},
				RoutableProcessName: "web",
			},
		},
		{
			name:    "two commands without web",
			content: "{\"processes\":[{\"type\":\"worker\"},{\"type\":\"abc\"}]}",
			want: &Procfile{
				Processes: map[string][]string{
					"worker": {"worker"},
					"abc":    {"abc"},
				},
				RoutableProcessName: "abc",
			},
		},
		{
			name:    "three commands without web",
			content: "{\"processes\":[{\"type\":\"aaa\"},{\"type\":\"zzz\"},{\"type\":\"bbb\"}]}",
			want: &Procfile{
				Processes: map[string][]string{
					"aaa": {"aaa"},
					"zzz": {"zzz"},
					"bbb": {"bbb"},
				},
				RoutableProcessName: "aaa",
			},
		},
		{
			name:    "ignore illicit name",
			content: "{\"processes\":[{\"type\":\"web\"},{\"type\":\"bad.name\"}]}",
			want: &Procfile{
				Processes: map[string][]string{
					"web": {"web"},
				},
				RoutableProcessName: "web",
			},
		},
		{
			name:    "broken json",
			content: "",
			wantErr: true,
		},
		{
			name:    "no processes",
			content: "{\"processes\":[]}",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CreateProcfile(tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseProcfile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseProcfile() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProcfileFromProcesses(t *testing.T) {
	tests := []struct {
		name      string
		processes []ketchv1.ProcessSpec

		want    *Procfile
		wantErr bool
	}{
		{
			name: "single process",
			processes: []ketchv1.ProcessSpec{
				{
					Name: "worker",
					Cmd:  []string{"python web.py"},
				},
			},
			want: &Procfile{
				Processes: map[string][]string{
					"worker": {"python web.py"},
				},
				RoutableProcessName: "worker",
			},
		},
		{
			name: "two processes",
			processes: []ketchv1.ProcessSpec{
				{Name: "worker", Cmd: []string{"entrypoint.sh", "npm", "worker"}},
				{Name: "abc", Cmd: []string{"entrypoint.sh", "npm", "abc"}},
			},
			want: &Procfile{
				Processes: map[string][]string{
					"worker": {"entrypoint.sh", "npm", "worker"},
					"abc":    {"entrypoint.sh", "npm", "abc"},
				},
				RoutableProcessName: "abc",
			},
		},
		{
			name: "two process with web",
			processes: []ketchv1.ProcessSpec{
				{Name: "web", Cmd: []string{"entrypoint.sh", "npm", "start"}},
				{Name: "abc", Cmd: []string{"entrypoint.sh", "npm", "abc"}},
			},
			want: &Procfile{
				Processes: map[string][]string{
					"web": {"entrypoint.sh", "npm", "start"},
					"abc": {"entrypoint.sh", "npm", "abc"},
				},
				RoutableProcessName: "web",
			},
		},
		{
			name:      "no processes",
			processes: []ketchv1.ProcessSpec{},
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ProcfileFromProcesses(tt.processes)
			if (err != nil) != tt.wantErr {
				t.Errorf("ProcfileFromProcesses() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("ProcfileFromProcesses mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

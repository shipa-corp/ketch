package deploy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func intRef(i int) *int {
	return &i
}

func TestChangeSet_getStepWeight(t *testing.T) {

	tests := []struct {
		name    string
		set     ChangeSet
		action  func(set *ChangeSet)
		want    uint8
		wantErr string
	}{
		{
			name: "happy path",
			set:  ChangeSet{steps: intRef(4)},
			want: 25,
		},
		{
			name: "happy path",
			set:  ChangeSet{steps: intRef(5)},
			want: 20,
		},
		{
			name:    "error - no steps",
			set:     ChangeSet{},
			wantErr: `"steps" missing`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			weight, err := tt.set.getStepWeight()
			if len(tt.wantErr) > 0 {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErr, err.Error())
				return
			}
			require.Nil(t, err)
			require.Equal(t, tt.want, weight)
		})
	}
}

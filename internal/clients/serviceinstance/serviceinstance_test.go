package serviceinstance

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestDiffSharedSpaces(t *testing.T) {
	type args struct {
		current []string
		desired []string
	}

	type want struct {
		toAdd    []string
		toRemove []string
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"NoChanges": {
			args: args{
				current: []string{"space-1", "space-2", "space-3"},
				desired: []string{"space-1", "space-2", "space-3"},
			},
			want: want{
				toAdd:    nil,
				toRemove: nil,
			},
		},
		"AddOnly": {
			args: args{
				current: []string{"space-1", "space-2"},
				desired: []string{"space-1", "space-2", "space-3", "space-4"},
			},
			want: want{
				toAdd:    []string{"space-3", "space-4"},
				toRemove: nil,
			},
		},
		"RemoveOnly": {
			args: args{
				current: []string{"space-1", "space-2", "space-3", "space-4"},
				desired: []string{"space-1", "space-2"},
			},
			want: want{
				toAdd:    nil,
				toRemove: []string{"space-3", "space-4"},
			},
		},
		"AddAndRemove": {
			args: args{
				current: []string{"space-1", "space-2", "space-3"},
				desired: []string{"space-2", "space-4", "space-5"},
			},
			want: want{
				toAdd:    []string{"space-4", "space-5"},
				toRemove: []string{"space-1", "space-3"},
			},
		},
		"AddAll": {
			args: args{
				current: []string{},
				desired: []string{"space-1", "space-2", "space-3"},
			},
			want: want{
				toAdd:    []string{"space-1", "space-2", "space-3"},
				toRemove: nil,
			},
		},
		"RemoveAll": {
			args: args{
				current: []string{"space-1", "space-2", "space-3"},
				desired: []string{},
			},
			want: want{
				toAdd:    nil,
				toRemove: []string{"space-1", "space-2", "space-3"},
			},
		},
		"BothEmpty": {
			args: args{
				current: []string{},
				desired: []string{},
			},
			want: want{
				toAdd:    nil,
				toRemove: nil,
			},
		},
		"CompleteReplacement": {
			args: args{
				current: []string{"space-1", "space-2"},
				desired: []string{"space-3", "space-4"},
			},
			want: want{
				toAdd:    []string{"space-3", "space-4"},
				toRemove: []string{"space-1", "space-2"},
			},
		},
		"DuplicatesInCurrent": {
			args: args{
				current: []string{"space-1", "space-1", "space-2"},
				desired: []string{"space-2", "space-3"},
			},
			want: want{
				toAdd:    []string{"space-3"},
				toRemove: []string{"space-1"},
			},
		},
		"DuplicatesInDesired": {
			args: args{
				current: []string{"space-1", "space-2"},
				desired: []string{"space-2", "space-2", "space-3"},
			},
			want: want{
				// The second "space-2" will be added because we delete from the set on first encounter
				toAdd:    []string{"space-2", "space-3"},
				toRemove: []string{"space-1"},
			},
		},
		"SingleItemNoChange": {
			args: args{
				current: []string{"space-1"},
				desired: []string{"space-1"},
			},
			want: want{
				toAdd:    nil,
				toRemove: nil,
			},
		},
		"SingleItemAdd": {
			args: args{
				current: []string{},
				desired: []string{"space-1"},
			},
			want: want{
				toAdd:    []string{"space-1"},
				toRemove: nil,
			},
		},
		"SingleItemRemove": {
			args: args{
				current: []string{"space-1"},
				desired: []string{},
			},
			want: want{
				toAdd:    nil,
				toRemove: []string{"space-1"},
			},
		},
		"LargeSet": {
			args: args{
				current: []string{"space-1", "space-2", "space-3", "space-4", "space-5", "space-6", "space-7", "space-8", "space-9", "space-10"},
				desired: []string{"space-3", "space-5", "space-7", "space-11", "space-12", "space-13"},
			},
			want: want{
				toAdd:    []string{"space-11", "space-12", "space-13"},
				toRemove: []string{"space-1", "space-2", "space-4", "space-6", "space-8", "space-9", "space-10"},
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			toAdd, toRemove := diffSharedSpaces(tc.args.current, tc.args.desired)

			// Use cmpopts.SortSlices to handle order independence in slice comparison
			sortStrings := cmpopts.SortSlices(func(a, b string) bool { return a < b })

			if diff := cmp.Diff(tc.want.toAdd, toAdd, sortStrings); diff != "" {
				t.Errorf("diffSharedSpaces(...) toAdd mismatch (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.want.toRemove, toRemove, sortStrings); diff != "" {
				t.Errorf("diffSharedSpaces(...) toRemove mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

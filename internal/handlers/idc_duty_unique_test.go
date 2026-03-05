package handlers

import (
	"errors"
	"testing"
)

func TestIsIDCDutyDuplicateDateError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "new composite index",
			err:  errors.New(`ERROR: duplicate key value violates unique constraint "idx_idc_duty_user_date"`),
			want: true,
		},
		{
			name: "legacy unique index",
			err:  errors.New(`ERROR: duplicate key value violates unique constraint "idx_idc_duty_records_date"`),
			want: true,
		},
		{
			name: "legacy unique constraint",
			err:  errors.New(`ERROR: duplicate key value violates unique constraint "idc_duty_records_date_key"`),
			want: true,
		},
		{
			name: "non duplicate error",
			err:  errors.New("timeout"),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tc := range cases {
		if got := isIDCDutyDuplicateDateError(tc.err); got != tc.want {
			t.Fatalf("%s: got=%v want=%v", tc.name, got, tc.want)
		}
	}
}

func TestIDCDutyWriteErrorMessage(t *testing.T) {
	dupErr := errors.New(`ERROR: duplicate key value violates unique constraint "idx_idc_duty_user_date"`)
	if got := idcDutyWriteErrorMessage("保存失败：", dupErr); got != "同一用户在同一天只能创建一条 IDC 值班记录" {
		t.Fatalf("unexpected duplicate message: %q", got)
	}

	genericErr := errors.New("db unavailable")
	if got := idcDutyWriteErrorMessage("保存失败：", genericErr); got != "保存失败：db unavailable" {
		t.Fatalf("unexpected generic message: %q", got)
	}
}

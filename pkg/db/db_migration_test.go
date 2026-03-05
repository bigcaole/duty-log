package db

import "testing"

func TestIDCDutyUserDateUniqueIndexName(t *testing.T) {
	if got := idcDutyUserDateUniqueIndexName(); got != "idx_idc_duty_user_date" {
		t.Fatalf("unexpected index name: %q", got)
	}
}

func TestIDCDutyLegacyUniqueDateConstraintSQL(t *testing.T) {
	sqlList := idcDutyLegacyUniqueDateConstraintSQL()
	if len(sqlList) == 0 {
		t.Fatalf("legacy SQL list should not be empty")
	}
	if sqlList[0] != `ALTER TABLE idc_duty_records DROP CONSTRAINT IF EXISTS idc_duty_records_date_key` {
		t.Fatalf("unexpected first SQL: %q", sqlList[0])
	}
}

package repository

import (
	"encoding/json"
	"testing"

	"gorm.io/datatypes"
)

func TestRemapJSONIDs(t *testing.T) {
	got := remapJSONIDs(datatypes.JSON([]byte(`[1,2,9]`)), map[int64]int64{1: 11, 2: 22})
	var ids []int64
	if err := json.Unmarshal(got, &ids); err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != 11 || ids[1] != 22 {
		t.Fatalf("ids = %v", ids)
	}
}

func TestMappedPtrDropsMissingReference(t *testing.T) {
	id := int64(7)
	if got := mappedPtr(&id, map[int64]int64{}); got != nil {
		t.Fatalf("got = %v", *got)
	}
	if got := mappedPtr(&id, map[int64]int64{7: 70}); got == nil || *got != 70 {
		t.Fatalf("got = %v", got)
	}
}

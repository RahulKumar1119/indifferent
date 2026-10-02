package storage

import "testing"

func TestObjectStatus_StandardDownloads(t *testing.T) {
	for _, st := range []ObjectStatus{
		{},
		{StorageClass: "STANDARD"},
		{StorageClass: "GLACIER_IR"},
		{StorageClass: "INTELLIGENT_TIERING"},
	} {
		if st.Archived() {
			t.Errorf("%+v: expected not archived", st)
		}
		if !st.Downloadable() {
			t.Errorf("%+v: expected downloadable", st)
		}
	}
}

func TestObjectStatus_GlacierNeedsRestore(t *testing.T) {
	for _, st := range []ObjectStatus{
		{StorageClass: "GLACIER"},
		{StorageClass: "DEEP_ARCHIVE"},
	} {
		if !st.Archived() {
			t.Errorf("%+v: expected archived", st)
		}
		if st.Downloadable() {
			t.Errorf("%+v: expected not downloadable without restore", st)
		}
		if st.RestoreInProgress() {
			t.Errorf("%+v: expected no restore in progress", st)
		}
	}
}

func TestObjectStatus_RestoreLifecycle(t *testing.T) {
	inProgress := ObjectStatus{StorageClass: "GLACIER", Restore: `ongoing-request="true"`}
	if !inProgress.RestoreInProgress() || inProgress.Downloadable() {
		t.Errorf("restoring object must not be downloadable: %+v", inProgress)
	}

	restored := ObjectStatus{
		StorageClass: "GLACIER",
		Restore:      `ongoing-request="false", expiry-date="Fri, 09 Oct 2026 00:00:00 GMT"`,
	}
	if restored.RestoreInProgress() {
		t.Errorf("completed restore misread as in-progress: %+v", restored)
	}
	if !restored.Downloadable() {
		t.Errorf("restored copy must be downloadable: %+v", restored)
	}
}

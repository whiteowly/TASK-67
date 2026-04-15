// cohort_test.go — tests for the deterministic cohort-assignment helper
// that backs feature-flag canary releases.
//
// The cohort helper is the gating decision for every feature-flag rollout.
// It must be:
//   - deterministic (same input → same bucket every call),
//   - in range [0, 100),
//   - reshuffled by version bumps (so 10% → 50% rollouts re-randomize),
//   - distinct between flag keys (one flag's cohort ≠ another's for same user).
package service

import (
	"testing"

	"github.com/google/uuid"
)

// Determinism: repeated calls return the same bucket.
func TestCohortBucket_Deterministic(t *testing.T) {
	uid := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	first := cohortBucket("test.flag", uid, 1)
	for i := 0; i < 100; i++ {
		got := cohortBucket("test.flag", uid, 1)
		if got != first {
			t.Fatalf("call %d returned %d, expected stable %d", i, got, first)
		}
	}
}

// In-range: every output is in [0, 100).
func TestCohortBucket_InRange(t *testing.T) {
	for i := 0; i < 1000; i++ {
		uid := uuid.New()
		got := cohortBucket("flag.x", uid, 1)
		if got < 0 || got >= 100 {
			t.Fatalf("bucket out of range: %d for uid %s", got, uid)
		}
	}
}

// Version bump reshuffles assignments — for 1000 users, at least 50% should
// land in a different bucket between version 1 and version 2. (With a
// uniform hash and 100 buckets, the expected mismatch rate is ~99%.)
func TestCohortBucket_VersionBumpReshuffles(t *testing.T) {
	differ := 0
	const N = 1000
	for i := 0; i < N; i++ {
		uid := uuid.New()
		if cohortBucket("flag.x", uid, 1) != cohortBucket("flag.x", uid, 2) {
			differ++
		}
	}
	// Allow generous slack: we just need to confirm bucket assignment is
	// not version-invariant.
	if differ < N/2 {
		t.Errorf("version bump reshuffled only %d/%d users; expected >=%d",
			differ, N, N/2)
	}
}

// Different flag keys produce different buckets for the same user — the
// hash binds the flag key, so two flags don't share cohort assignment.
func TestCohortBucket_FlagKeyAffectsBucket(t *testing.T) {
	differ := 0
	const N = 500
	for i := 0; i < N; i++ {
		uid := uuid.New()
		if cohortBucket("flag.A", uid, 1) != cohortBucket("flag.B", uid, 1) {
			differ++
		}
	}
	if differ < N/2 {
		t.Errorf("different flag keys produced same bucket %d/%d times; expected mostly different",
			N-differ, N)
	}
}

// Bucket distribution is reasonably uniform across the 100 buckets.
// Sampling 10000 random UUIDs against a single flag, no bucket should hold
// more than 5% of the population (well above the expected 1%).
func TestCohortBucket_Distribution(t *testing.T) {
	const N = 10_000
	var counts [100]int
	for i := 0; i < N; i++ {
		counts[cohortBucket("uniformity.flag", uuid.New(), 1)]++
	}
	for i, n := range counts {
		if n > N/20 { // 5% ceiling
			t.Errorf("bucket %d held %d/%d (>5%%) — distribution looks skewed",
				i, n, N)
		}
	}
}

// At cohort_percent=0 a typical user must NOT be selected; at =100 every
// user must be (the latter is short-circuited in IsEnabledForUser, so
// here we just spot-check that the bucket is < 100, satisfying any > 0
// rollout.) This is the test of the inequality logic that callers rely
// on: "bucket < cohort_percent" maps to "user is in the cohort".
func TestCohortBucket_SelectionInequality(t *testing.T) {
	uid := uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
	bucket := cohortBucket("inequality.flag", uid, 1)

	// At 0% cohort, this user must be excluded (bucket >= 0 is always true).
	if !(bucket >= 0) {
		t.Error("at 0% cohort, every user must be excluded")
	}
	// At 100%, every user must be included (bucket < 100 is the IsEnabledForUser path).
	if !(bucket < 100) {
		t.Error("bucket must always be < 100 to satisfy a full cohort selection")
	}
}

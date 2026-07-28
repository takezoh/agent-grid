package sandbox

import "testing"

func TestRepositoryEnvironmentKeyAndIsolation(t *testing.T) {
	k := RepositoryEnvironmentKey{ServerID: "s1", RepositoryID: "42", ConfigGen: "g1"}
	if err := k.Validate(); err != nil {
		t.Fatal(err)
	}
	if k.String() != "s1|42|g1" {
		t.Fatalf("key=%s", k.String())
	}
	if err := (RepositoryEnvironmentKey{}).Validate(); err == nil {
		t.Fatal("empty key")
	}
	p := RepositoryIsolationPlan()
	if p.IsShared() {
		t.Fatal("must be project isolation")
	}
	if err := RejectSharedRepositoryIsolation(IsolationPlan{Kind: IsolationShared}); err == nil {
		t.Fatal("shared must be rejected")
	}
	if err := RejectSharedRepositoryIsolation(p); err != nil {
		t.Fatal(err)
	}
}

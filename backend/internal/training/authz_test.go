package training

import "testing"

func TestPrincipalCanAccessEmployee(t *testing.T) {
	if !principalCanAccessEmployee(Principal{Email: "USER@EXAMPLE.COM"}, "user@example.com") {
		t.Fatal("employee should access own rows case-insensitively")
	}
	if principalCanAccessEmployee(Principal{Email: "user@example.com"}, "other@example.com") {
		t.Fatal("employee should not access other employee rows")
	}
	if !principalCanAccessEmployee(Principal{Email: "people@example.com", IsPeopleAdmin: true}, "other@example.com") {
		t.Fatal("People admin should access employee rows")
	}
	if principalCanAccessEmployee(Principal{}, "other@example.com") {
		t.Fatal("missing principal email should not access employee rows")
	}
}

func TestActorForPrincipal(t *testing.T) {
	if got := actorForPrincipal(Principal{}); got != ActorEmployee {
		t.Fatalf("actorForPrincipal employee = %q", got)
	}
	if got := actorForPrincipal(Principal{IsPeopleAdmin: true}); got != ActorPeopleAdmin {
		t.Fatalf("actorForPrincipal people = %q", got)
	}
}
